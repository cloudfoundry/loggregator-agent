package app

import (
	"fmt"
	"log"
	"net"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/pulseemitter"
	"code.cloudfoundry.org/loggregator-agent/pkg/healthendpoint"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"github.com/prometheus/client_golang/prometheus"
)

type Agent struct {
	config *Config
	lookup func(string) ([]net.IP, error)
}

// AgentOption configures agent options.
type AgentOption func(*Agent)

// WithLookup allows the default DNS resolver to be changed.
func WithLookup(l func(string) ([]net.IP, error)) func(*Agent) {
	return func(a *Agent) {
		a.lookup = l
	}
}

func NewAgent(
	c *Config,
	opts ...AgentOption,
) *Agent {
	a := &Agent{
		config: c,
		lookup: net.LookupIP,
	}

	for _, o := range opts {
		o(a)
	}

	return a
}

func (a *Agent) Start() {
	clientCreds, err := plumbing.NewClientCredentials(
		a.config.GRPC.CertFile,
		a.config.GRPC.KeyFile,
		a.config.GRPC.CAFile,
		"doppler",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for client: %s", err)
	}

	var opts []plumbing.ConfigOption
	if len(a.config.GRPC.CipherSuites) > 0 {
		opts = append(opts, plumbing.WithCipherSuites(a.config.GRPC.CipherSuites))
	}

	serverCreds, err := plumbing.NewServerCredentials(
		a.config.GRPC.CertFile,
		a.config.GRPC.KeyFile,
		a.config.GRPC.CAFile,
		opts...,
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for server: %s", err)
	}

	batchInterval := time.Duration(a.config.MetricBatchIntervalMilliseconds) * time.Millisecond
	ingressTLS, err := loggregator.NewIngressTLSConfig(
		a.config.GRPC.CAFile,
		a.config.GRPC.CertFile,
		a.config.GRPC.KeyFile,
	)
	if err != nil {
		log.Fatalf("failed to load ingress TLS config: %s", err)
	}

	ingressClient, err := loggregator.NewIngressClient(ingressTLS,
		loggregator.WithTag("origin", "loggregator.metron"),
		loggregator.WithAddr(fmt.Sprintf("127.0.0.1:%d", a.config.GRPC.Port)),
	)
	if err != nil {
		log.Fatalf("failed to initialize ingress client: %s", err)
	}

	metricClient := pulseemitter.New(
		ingressClient,
		pulseemitter.WithPulseInterval(batchInterval),
		pulseemitter.WithSourceID(a.config.MetricSourceID),
	)

	healthRegistrar := startHealthEndpoint(fmt.Sprintf("127.0.0.1:%d", a.config.HealthEndpointPort))

	appV1 := NewV1App(a.config, healthRegistrar, clientCreds, metricClient)
	go appV1.Start()

	appV2 := NewV2App(a.config, healthRegistrar, clientCreds, serverCreds, metricClient)
	go appV2.Start()
}

func startHealthEndpoint(addr string) *healthendpoint.Registrar {
	promRegistry := prometheus.NewRegistry()
	healthendpoint.StartServer(addr, promRegistry)
	healthRegistrar := healthendpoint.New(promRegistry, map[string]prometheus.Gauge{
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
		// metric-documentation-health: (originMappings)
		// Number of origin -> sourceId mappings
		"originMappings": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "agent",
				Name:      "originMappings",
				Help:      "Number of origin -> source id conversions",
			},
		),
	})

	return healthRegistrar
}
