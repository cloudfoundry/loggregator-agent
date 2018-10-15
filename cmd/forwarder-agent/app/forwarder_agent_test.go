package app_test

import (
	"context"
	"net"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/cmd/forwarder-agent/app"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

var _ = Describe("ForwarderAgent", func() {
	var (
		bf       *stubBindingFetcher
		sm       *spyMetrics
		spyAgent *spyLoggregatorAgent
	)

	BeforeEach(func() {
		bf = newStubBindingFetcher()
		sm = newSpyMetrics()
		spyAgent = newSpyLoggregatorAgent()
	})

	It("reports the number of binding that come from the Getter", func() {
		bf.bindings <- []cups.Binding{
			{"app-1", "host-1", "v3-syslog://drain.url.com"},
			{"app-2", "host-2", "v3-syslog://drain.url.com"},
			{"app-3", "host-3", "v3-syslog://drain.url.com"},
		}

		fa := app.NewForwarderAgent(0, spyAgent.Addr(), sm, bf, 100*time.Millisecond)
		fa.Run(false)

		var mv float64
		Eventually(sm.metricValues).Should(Receive(&mv))
		Expect(mv).To(BeNumerically("==", 3))
	})

	It("polls for updates from the binding fetcher and updates the metric accordingly", func() {
		bf.bindings <- []cups.Binding{
			{"app-1", "host-1", "v3-syslog://drain.url.com"},
			{"app-3", "host-3", "v3-syslog://drain.url.com"},
		}
		bf.bindings <- []cups.Binding{
			{"app-1", "host-1", "v3-syslog://drain.url.com"},
			{"app-3", "host-3", "v3-syslog://drain.url.com"},
			{"app-3", "host-3", "v3-syslog://drain.url.com"},
		}

		fa := app.NewForwarderAgent(0, spyAgent.Addr(), sm, bf, 100*time.Millisecond)
		fa.Run(false)

		Eventually(sm.metricValues).Should(HaveLen(2))
		Expect(<-sm.metricValues).To(BeNumerically("==", 2))
		Expect(<-sm.metricValues).To(BeNumerically("==", 3))
	})

	Describe("envelope forwarding", func() {
		It("recieves V2 envelopes and forwards them to loggregator agent", func() {
			fa := app.NewForwarderAgent(0, spyAgent.Addr(), sm, bf, 100*time.Millisecond)
			fa.Run(false)

			Eventually(fa.IngressAddr, 2).ShouldNot(BeEmpty())
			conn, err := grpc.Dial(fa.IngressAddr(), grpc.WithBlock(), grpc.WithInsecure())
			Expect(err).ToNot(HaveOccurred())

			ingressClient := loggregator_v2.NewIngressClient(conn)

			// Real Ingress Client -> Real Forwarder Agent -> Fake Loggregator Agent
			//                   Sender                 BatchSender
			//                   BatchSender
			//                   Send
			ingressClient.Send(context.Background(), &loggregator_v2.EnvelopeBatch{
				Batch: []*loggregator_v2.Envelope{
					{
						Message: &loggregator_v2.Envelope_Log{
							Log: &loggregator_v2.Log{
								Payload: []byte("test message"),
							},
						},
					},
				},
			})

			Eventually(spyAgent.envs).Should(HaveLen(1))
		})
	})
})

type spyLoggregatorAgent struct {
	listener net.Listener
	_envs    []*loggregator_v2.Envelope
}

func newSpyLoggregatorAgent() *spyLoggregatorAgent {
	spyAgent := &spyLoggregatorAgent{}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	spyAgent.listener = lis

	s := grpc.NewServer(
		// grpc.Creds(transportCreds),
		grpc.KeepaliveEnforcementPolicy(
			keepalive.EnforcementPolicy{
				MinTime:             10 * time.Second,
				PermitWithoutStream: true,
			},
		),
	)
	loggregator_v2.RegisterIngressServer(s, spyAgent)

	go s.Serve(lis)

	return spyAgent
}

func (a *spyLoggregatorAgent) Addr() string {
	return a.listener.Addr().String()
}

func (a *spyLoggregatorAgent) Sender(loggregator_v2.Ingress_SenderServer) error {
	panic("Not implemented")
}

func (a *spyLoggregatorAgent) BatchSender(senderServer loggregator_v2.Ingress_BatchSenderServer) error {
	for {
		batch, err := senderServer.Recv()
		if err != nil {
			return err
		}

		for _, env := range batch.GetBatch() {
			a._envs = append(a._envs, env)
		}
	}
}

func (a *spyLoggregatorAgent) Send(context.Context, *loggregator_v2.EnvelopeBatch) (*loggregator_v2.SendResponse, error) {
	panic("Not implemented")
}

func (a spyLoggregatorAgent) envs() []*loggregator_v2.Envelope {
	return a._envs
}

type stubBindingFetcher struct {
	bindings chan []cups.Binding
}

func newStubBindingFetcher() *stubBindingFetcher {
	return &stubBindingFetcher{
		bindings: make(chan []cups.Binding, 100),
	}
}

func (s *stubBindingFetcher) FetchBindings() ([]cups.Binding, error) {
	select {
	case b := <-s.bindings:
		return b, nil
	default:
		return nil, nil
	}
}

type spyMetrics struct {
	name         string
	metricValues chan float64
}

func newSpyMetrics() *spyMetrics {
	return &spyMetrics{
		metricValues: make(chan float64, 100),
	}
}

func (sm *spyMetrics) NewGauge(name string) func(float64) {
	sm.name = name
	return func(val float64) {
		sm.metricValues <- val
	}
}
