package main_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"sync"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"google.golang.org/grpc"
)

var _ = Describe("Main", func() {
	var (
		promServer *httptest.Server
		spyAgent   *spyAgent

		session *gexec.Session
	)

	BeforeEach(func() {
		spyAgent = newSpyAgent()
		promServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Write([]byte(promOutput))
		}))

		session = startScraper(
			"CLIENT_KEY_PATH="+testhelper.Cert("prom-scraper.key"),
			"CLIENT_CERT_PATH="+testhelper.Cert("prom-scraper.crt"),
			"CA_CERT_PATH="+testhelper.Cert("loggregator-ca.crt"),
			"LOGGREGATOR_AGENT_ADDR="+spyAgent.addr,
			"METRICS_URL="+promServer.URL,
			"SOURCE_ID=some-id",
			"SCRAPE_INTERVAL=100ms",
		)
	})

	AfterEach(func() {
		session.Kill()
		gexec.CleanupBuildArtifacts()
	})

	It("scrapes a prometheus endpoint and send those metrics to loggregator agent", func() {
		Eventually(spyAgent.Envelopes).Should(And(
			ContainElement(buildEnvelope("node_timex_pps_calibration_total", 1)),
			ContainElement(buildEnvelope("node_timex_pps_error_total", 2)),
			ContainElement(buildEnvelope("node_timex_pps_frequency_hertz", 3)),
			ContainElement(buildEnvelope("node_timex_pps_jitter_seconds", 4)),
			ContainElement(buildEnvelope("node_timex_pps_jitter_total", 5)),
		))
	})
})

func buildEnvelope(name string, value float64) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		SourceId: "some-id",
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					name: &loggregator_v2.GaugeValue{Value: value},
				},
			},
		},
	}
}

func startScraper(envs ...string) *gexec.Session {
	path, err := gexec.Build("code.cloudfoundry.org/loggregator-agent/cmd/prom-scraper")
	if err != nil {
		panic(err)
	}

	cmd := exec.Command(path)
	cmd.Env = envs
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		panic(err)
	}

	return session
}

const (
	promOutput = `
# HELP node_timex_pps_calibration_total Pulse per second count of calibration intervals.
# TYPE node_timex_pps_calibration_total counter
node_timex_pps_calibration_total 1
# HELP node_timex_pps_error_total Pulse per second count of calibration errors.
# TYPE node_timex_pps_error_total counter
node_timex_pps_error_total 2
# HELP node_timex_pps_frequency_hertz Pulse per second frequency.
# TYPE node_timex_pps_frequency_hertz gauge
node_timex_pps_frequency_hertz 3
# HELP node_timex_pps_jitter_seconds Pulse per second jitter.
# TYPE node_timex_pps_jitter_seconds gauge
node_timex_pps_jitter_seconds 4
# HELP node_timex_pps_jitter_total Pulse per second count of jitter limit exceeded events.
# TYPE node_timex_pps_jitter_total counter
node_timex_pps_jitter_total 5
`
)

type spyAgent struct {
	mu        sync.Mutex
	envelopes []*loggregator_v2.Envelope
	addr      string
}

func newSpyAgent() *spyAgent {
	agent := &spyAgent{}

	serverCreds, err := plumbing.NewServerCredentials(
		testhelper.Cert("metron.crt"),
		testhelper.Cert("metron.key"),
		testhelper.Cert("loggregator-ca.crt"),
	)
	if err != nil {
		panic(err)
	}

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	agent.addr = lis.Addr().String()

	grpcServer := grpc.NewServer(grpc.Creds(serverCreds))
	loggregator_v2.RegisterIngressServer(grpcServer, agent)

	go grpcServer.Serve(lis)

	return agent
}

func (s *spyAgent) Sender(loggregator_v2.Ingress_SenderServer) error {
	panic("not implemented")
}

func (s *spyAgent) BatchSender(srv loggregator_v2.Ingress_BatchSenderServer) error {
	for {
		batch, err := srv.Recv()
		if err != nil {
			return err
		}

		for _, e := range batch.GetBatch() {
			if e.GetTimestamp() == 0 {
				panic("0 timestamp!?")
			}

			// We want to make our lives easier for matching against envelopes
			e.Timestamp = 0
		}

		s.mu.Lock()
		s.envelopes = append(s.envelopes, batch.GetBatch()...)
		s.mu.Unlock()
	}
}

func (s *spyAgent) Envelopes() []*loggregator_v2.Envelope {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make([]*loggregator_v2.Envelope, len(s.envelopes))
	copy(results, s.envelopes)
	return results
}

func (s *spyAgent) Send(context.Context, *loggregator_v2.EnvelopeBatch) (*loggregator_v2.SendResponse, error) {
	panic("not implemented")
}
