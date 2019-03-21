package app

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"fmt"
	"log"
	"net/http"

	loggregator "code.cloudfoundry.org/go-loggregator"
	ingress "code.cloudfoundry.org/loggregator-agent/pkg/ingress/v1"
	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	"github.com/cloudfoundry/sonde-go/events"
)

type Metrics interface {
	NewGauge(name string, opts ...metrics.MetricOption) (metrics.Gauge, error)
	NewCounter(name string, opts ...metrics.MetricOption) (metrics.Counter, error)
}

type UDPForwarder struct {
	grpc      GRPC
	udpPort   int
	debugPort int
	log       *log.Logger
	metrics   Metrics
}

func NewUDPForwarder(cfg Config, l *log.Logger, m Metrics) *UDPForwarder {
	return &UDPForwarder{
		grpc:      cfg.LoggregatorAgentGRPC,
		udpPort:   cfg.UDPPort,
		debugPort: cfg.DebugPort,
		log:       l,
		metrics:   m,
	}
}

func (u *UDPForwarder) Run() {
	tlsConfig, err := loggregator.NewIngressTLSConfig(
		u.grpc.CAFile,
		u.grpc.CertFile,
		u.grpc.KeyFile,
	)
	if err != nil {
		u.log.Fatalf("Failed to create loggregator agent credentials: %s", err)
	}

	v2Ingress, err := loggregator.NewIngressClient(
		tlsConfig,
		loggregator.WithLogger(u.log),
		loggregator.WithAddr(u.grpc.Addr),
	)
	if err != nil {
		u.log.Fatalf("Failed to create loggregator agent client: %s", err)
	}

	dropsondeUnmarshaller := ingress.NewUnMarshaller(v2Writer{v2Ingress})
	networkReader, err := ingress.NewNetworkReader(
		fmt.Sprintf("127.0.0.1:%d", u.udpPort),
		dropsondeUnmarshaller,
		u.metrics,
	)
	if err != nil {
		u.log.Fatalf("Failed to listen on 127.0.0.1:%d: %s", u.udpPort, err)
	}

	go func() {
		http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", u.debugPort), nil)
	}()

	go networkReader.StartReading()
	networkReader.StartWriting()
}

type v2Writer struct {
	ingressClient *loggregator.IngressClient
}

func (w v2Writer) Write(e *events.Envelope) {
	v2e := conversion.ToV2(e, true)
	w.ingressClient.Emit(v2e)
}
