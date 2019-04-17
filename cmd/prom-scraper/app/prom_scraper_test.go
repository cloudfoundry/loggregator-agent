package app_test

import (
	"code.cloudfoundry.org/loggregator-agent/cmd/prom-scraper/app"
	"fmt"
	"github.com/onsi/gomega/gexec"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

var _ = Describe("PromScraper", func() {
	var (
		spyAgent   *spyAgent
		cfg        app.Config
		testLogger = log.New(GinkgoWriter, "", log.LstdFlags)
	)

	Describe("when there is a debug_port config file", func() {
		var debugConfigDir = debugPortConfigDir()

		BeforeEach(func() {
			spyAgent = newSpyAgent()
			startPromServer(debugConfigDir, promOutput)

			cfg = app.Config{
				ClientKeyPath:          testhelper.Cert("metron.key"),
				ClientCertPath:         testhelper.Cert("metron.crt"),
				CACertPath:             testhelper.Cert("loggregator-ca.crt"),
				LoggregatorIngressAddr: spyAgent.addr,
				DefaultSourceID:        "some-id",
				DebugPortCfg:           fmt.Sprintf("%s/*/debug_port.yml", debugConfigDir),
				ScrapeInterval:         100 * time.Millisecond,
			}
		})

		AfterEach(func() {
			os.RemoveAll(debugConfigDir)
			gexec.CleanupBuildArtifacts()
		})

		It("scrapes a prometheus endpoint and sends those metrics to a loggregator agent", func() {
			ps := app.NewPromScraper(cfg, testLogger)
			go ps.Run()

			Eventually(spyAgent.Envelopes).Should(And(
				ContainElement(buildEnvelope("node_timex_pps_calibration_total", 1)),
				ContainElement(buildEnvelope("node_timex_pps_error_total", 2)),
				ContainElement(buildEnvelope("node_timex_pps_frequency_hertz", 3)),
				ContainElement(buildEnvelope("node_timex_pps_jitter_seconds", 4)),
				ContainElement(buildEnvelope("node_timex_pps_jitter_total", 5)),
			))
		})
	})

	Describe("when there are multiple debug_port config files", func() {
		var debugConfigDir = debugPortConfigDir()

		BeforeEach(func() {
			spyAgent = newSpyAgent()

			startPromServer(debugConfigDir, promOutput)
			startPromServer(debugConfigDir, promOutput2)

			cfg = app.Config{
				ClientKeyPath:          testhelper.Cert("prom-scraper.key"),
				ClientCertPath:         testhelper.Cert("prom-scraper.crt"),
				CACertPath:             testhelper.Cert("loggregator-ca.crt"),
				LoggregatorIngressAddr: spyAgent.addr,
				DefaultSourceID:        "some-id",
				DebugPortCfg:           fmt.Sprintf("%s/*/debug_port.yml", debugConfigDir),
				ScrapeInterval:         100 * time.Millisecond,
			}
		})

		AfterEach(func() {
			os.RemoveAll(debugConfigDir)
			gexec.CleanupBuildArtifacts()
		})

		It("scrapes multiple prometheus endpoints and sends those metrics to a loggregator agent", func() {
			ps := app.NewPromScraper(cfg, testLogger)
			go ps.Run()

			Eventually(spyAgent.Envelopes).Should(And(
				ContainElement(buildEnvelope("node_timex_pps_calibration_total", 1)),
				ContainElement(buildEnvelope("node_timex_pps_error_total", 2)),
				ContainElement(buildEnvelope("node_timex_pps_frequency_hertz", 3)),
				ContainElement(buildEnvelope("node_timex_pps_jitter_seconds", 4)),
				ContainElement(buildEnvelope("node_timex_pps_jitter_total", 5)),
				ContainElement(buildEnvelope("node2_counter", 6)),
			))
		})
	})
})

func startPromServer(debugConfigDir string, promOutput string) {
	promServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(promOutput))
	}))

	addr := promServer.Listener.Addr().String()
	tokens := strings.Split(addr, ":")
	port := tokens[len(tokens)-1]

	createDebugPortConfigFile(debugConfigDir, port)
}

func buildEnvelope(name string, value float64) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		SourceId: "some-id",
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					name: {Value: value},
				},
			},
		},
	}
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

const (
	promOutput2 = `
# HELP node2_counter A second counter from another metrics url
# TYPE node2_counter counter
node2_counter 6
`
)
const debugConfigTemplate = `---
debug: %s
`

func debugPortConfigDir() string {
	dir, err := ioutil.TempDir(".", "")
	if err != nil {
		log.Fatal(err)
	}

	return dir
}

func createDebugPortConfigFile(configDir, port string) {
	fDir, err := ioutil.TempDir(configDir, "")
	if err != nil {
		log.Fatal(err)
	}

	tmpfn := filepath.Join(fDir, "debug_port.yml")
	tmpfn, err = filepath.Abs(tmpfn)
	Expect(err).ToNot(HaveOccurred())

	contents := fmt.Sprintf(debugConfigTemplate, port)
	if err := ioutil.WriteFile(tmpfn, []byte(contents), 0666); err != nil {
		log.Fatal(err)
	}
}

type spyAgent struct {
	loggregator_v2.IngressServer

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
