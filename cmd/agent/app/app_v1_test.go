package app_test

import (
	"code.cloudfoundry.org/loggregator-agent/cmd/agent/app"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"net"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("v1 App", func() {
	It("uses DopplerAddrWithAZ for AZ affinity", func() {
		spyLookup := newSpyLookup()

		clientCreds, err := plumbing.NewClientCredentials(
			testhelper.Cert("metron.crt"),
			testhelper.Cert("metron.key"),
			testhelper.Cert("loggregator-ca.crt"),
			"doppler",
		)
		Expect(err).ToNot(HaveOccurred())

		config := buildAgentConfig("127.0.0.1", 1234)
		config.Zone = "something-bad"
		expectedHost, _, err := net.SplitHostPort(config.RouterAddrWithAZ)
		Expect(err).ToNot(HaveOccurred())

		app := app.NewV1App(
			&config,
			clientCreds,
			testhelper.NewMetricClient(),
			app.WithV1Lookup(spyLookup.lookup),
		)
		go app.Start()

		Eventually(spyLookup.calledWith(expectedHost)).Should(BeTrue())
	})

	DescribeTable("has metric and expected tags", func(name string, tags map[string]string) {
		spyLookup := newSpyLookup()

		clientCreds, err := plumbing.NewClientCredentials(
			testhelper.Cert("metron.crt"),
			testhelper.Cert("metron.key"),
			testhelper.Cert("loggregator-ca.crt"),
			"doppler",
		)
		Expect(err).ToNot(HaveOccurred())

		config := buildAgentConfig("127.0.0.1", 1234)
		config.Zone = "something-bad"
		Expect(err).ToNot(HaveOccurred())

		mc := testhelper.NewMetricClient()
		app := app.NewV1App(
			&config,
			clientCreds,
			mc,
			app.WithV1Lookup(spyLookup.lookup),
		)
		go app.Start()

		Eventually(func() bool {
			return mc.HasMetric(name, tags)
		}).Should(BeTrue())

		m := mc.GetMetric(name, tags)
		for k, v := range tags {
			Expect(m.Opts.ConstLabels).To(HaveKeyWithValue(k, v))
		}
	},
		Entry("Dropped", "dropped", map[string]string{}),
		Entry(
			"AverageEnvelope",
			"average_envelope",
			map[string]string{"unit": "bytes/minute", "loggregator": "v1"},
		),
	)
})

type spyLookup struct {
	mu          sync.Mutex
	_calledWith map[string]struct{}
}

func newSpyLookup() *spyLookup {
	return &spyLookup{
		_calledWith: make(map[string]struct{}),
	}
}

func (s *spyLookup) calledWith(host string) func() bool {
	return func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		_, ok := s._calledWith[host]
		return ok
	}
}

func (s *spyLookup) lookup(host string) ([]net.IP, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s._calledWith[host] = struct{}{}
	return []net.IP{
		net.IPv4(byte(127), byte(0), byte(0), byte(1)),
	}, nil
}

func stubGaugeMap() map[string]prometheus.Gauge {
	return map[string]prometheus.Gauge{
		// metric-documentation-health: (dopplerConnections)
		// Number of connections open to dopplers.
		"dopplerConnections": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "agent",
				Name:      "dopplerConnections",
				Help:      "Number of connections open to dopplers",
			},
		),
		// metric-documentation-health: (dopplerV1Streams)
		// Number of V1 gRPC streams to dopplers.
		"dopplerV1Streams": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "agent",
				Name:      "dopplerV1Streams",
				Help:      "Number of V1 gRPC streams to dopplers",
			},
		),
		// metric-documentation-health: (dopplerV2Streams)
		// Number of V2 gRPC streams to dopplers.
		"dopplerV2Streams": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "agent",
				Name:      "dopplerV2Streams",
				Help:      "Number of V2 gRPC streams to dopplers",
			},
		),
		"originMappings": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "agent",
				Name:      "originMappings",
				Help:      "Number of origin -> source id conversions",
			},
		),
	}
}

func buildAgentConfig(dopplerURI string, dopplerGRPCPort int) app.Config {
	availabilityZone := "test-availability-zone"
	jobName := "test-job-name"
	jobIndex := "42"

	return app.Config{
		Index: jobIndex,
		Job:   jobName,
		Zone:  availabilityZone,

		Tags: map[string]string{
			"auto-tag-1": "auto-tag-value-1",
			"auto-tag-2": "auto-tag-value-2",
		},

		Deployment: "deployment",

		RouterAddr:       fmt.Sprintf("%s:%d", dopplerURI, dopplerGRPCPort),
		RouterAddrWithAZ: fmt.Sprintf("%s.%s:%d", availabilityZone, dopplerURI, dopplerGRPCPort),

		GRPC: app.GRPC{
			CertFile: testhelper.Cert("metron.crt"),
			KeyFile:  testhelper.Cert("metron.key"),
			CAFile:   testhelper.Cert("loggregator-ca.crt"),
		},

		MetricBatchIntervalMilliseconds: 5000,
	}
}
