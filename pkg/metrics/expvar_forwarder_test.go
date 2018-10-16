package metrics_test

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExpvarForwarder", func() {
	var (
		r *metrics.ExpvarForwarder

		addr        string
		server1     *httptest.Server
		server2     *httptest.Server
		spyAgent    *spyLoggregatorAgent
		clientCreds credentials.TransportCredentials
	)

	BeforeEach(func() {
		var err error

		clientCreds, err = plumbing.NewClientCredentials(
			testhelper.Cert("expvar-forwarder.crt"),
			testhelper.Cert("expvar-forwarder.key"),
			testhelper.Cert("loggregator-ca.crt"),
			"metron",
		)
		Expect(err).ToNot(HaveOccurred())

		spyAgent = newSpyLogCache()
		addr = spyAgent.start()
	})

	Context("Normal gauges and counters", func() {
		BeforeEach(func() {
			server1 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`
					{
						"ForwarderTest": {
							"CachePeriod": 68644,
							"Egress": 999,
							"Expired": 0,
							"Ingress": 633
						}
					}`,
				))
			}))

			server2 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`
					{
						"ForwarderTest": {
							"Egress": 999,
							"Ingress": 633
						}
					}`,
				))
			}))

			r = metrics.NewExpvarForwarder(addr,
				metrics.WithExpvarInterval(time.Millisecond),
				metrics.WithExpvarDefaultSourceId("default-source"),
				metrics.AddExpvarGaugeTemplate(
					server1.URL,
					"CachePeriod",
					"mS",
					"",
					"{{.ForwarderTest.CachePeriod}}",
					map[string]string{"a": "some-value"},
				),
				metrics.AddExpvarCounterTemplate(
					server2.URL,
					"Egress",
					"test-source",
					"{{.ForwarderTest.Egress}}",
					map[string]string{"a": "some-value"},
				),
				metrics.WithExpvarDialOpts(grpc.WithTransportCredentials(clientCreds)),
			)

			go r.Start()
		})

		It("writes the expvar metrics to LoggregatorAgent", func() {
			Eventually(func() int {
				return len(spyAgent.getEnvelopes())
			}).Should(BeNumerically(">=", 2))

			var e *loggregator_v2.Envelope
			for _, ee := range spyAgent.getEnvelopes() {
				if ee.GetCounter() == nil {
					continue
				}

				e = ee
			}

			Expect(e).ToNot(BeNil())
			Expect(e.SourceId).To(Equal("test-source"))
			Expect(e.Timestamp).ToNot(BeZero())
			Expect(e.GetCounter().Name).To(Equal("Egress"))
			Expect(e.GetCounter().Total).To(Equal(uint64(999)))
			Expect(e.Tags).To(Equal(map[string]string{"a": "some-value"}))

			e = nil
			for _, ee := range spyAgent.getEnvelopes() {
				if ee.GetGauge() == nil {
					continue
				}

				e = ee
			}

			Expect(e).ToNot(BeNil())
			Expect(e.SourceId).To(Equal("default-source"))
			Expect(e.Timestamp).ToNot(BeZero())
			Expect(e.GetGauge().Metrics).To(HaveLen(1))
			Expect(e.GetGauge().Metrics["CachePeriod"].Value).To(Equal(68644.0))
			Expect(e.GetGauge().Metrics["CachePeriod"].Unit).To(Equal("mS"))
			Expect(e.Tags).To(Equal(map[string]string{"a": "some-value"}))
		})

		It("writes correct timestamps to LoggregatorAgent", func() {
			Eventually(func() int {
				return len(spyAgent.getEnvelopes())
			}).Should(BeNumerically(">=", 4))

			var counterEnvelopes []*loggregator_v2.Envelope
			for _, ee := range spyAgent.getEnvelopes() {
				if ee.GetCounter() == nil {
					continue
				}

				counterEnvelopes = append(counterEnvelopes, ee)
			}

			Expect(counterEnvelopes[0].Timestamp).ToNot(Equal(counterEnvelopes[1].Timestamp))
		})

		It("panics if a counter or gauge template is invalid", func() {
			Expect(func() {
				metrics.NewExpvarForwarder(addr,
					metrics.AddExpvarCounterTemplate(
						server1.URL, "some-name", "a", "{{invalid", nil,
					),
				)
			}).To(Panic())

			Expect(func() {
				metrics.NewExpvarForwarder(addr,
					metrics.AddExpvarGaugeTemplate(
						server1.URL, "some-name", "", "a", "{{invalid", nil,
					),
				)
			}).To(Panic())
		})
	})
})

type spyLoggregatorAgent struct {
	mu        sync.Mutex
	envelopes []*loggregator_v2.Envelope
}

func newSpyLogCache() *spyLoggregatorAgent {
	return &spyLoggregatorAgent{}
}

func (s *spyLoggregatorAgent) start() string {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	creds, err := plumbing.NewServerCredentials(
		testhelper.Cert("metron.crt"),
		testhelper.Cert("metron.key"),
		testhelper.Cert("loggregator-ca.crt"),
	)
	Expect(err).ToNot(HaveOccurred())

	srv := grpc.NewServer(grpc.Creds(creds))

	loggregator_v2.RegisterIngressServer(srv, s)
	go srv.Serve(lis)

	return lis.Addr().String()
}

func (s *spyLoggregatorAgent) Send(
	ctx context.Context,
	r *loggregator_v2.EnvelopeBatch,
) (*loggregator_v2.SendResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, e := range r.GetBatch() {
		s.envelopes = append(s.envelopes, e)
	}

	return &loggregator_v2.SendResponse{}, nil
}

func (s *spyLoggregatorAgent) Sender(r loggregator_v2.Ingress_SenderServer) error {
	panic("not implemented")
}

func (s *spyLoggregatorAgent) BatchSender(r loggregator_v2.Ingress_BatchSenderServer) error {
	panic("not implemented")
}

func (s *spyLoggregatorAgent) getEnvelopes() []*loggregator_v2.Envelope {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([]*loggregator_v2.Envelope, len(s.envelopes))
	copy(r, s.envelopes)
	return r
}
