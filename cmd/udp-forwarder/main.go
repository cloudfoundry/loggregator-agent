package main

import (
	"expvar"
	"fmt"
	"log"
	"os"

	"net/http"
	_ "net/http/pprof"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/loggregator-agent/cmd/udp-forwarder/app"
	ingress "code.cloudfoundry.org/loggregator-agent/pkg/ingress/v1"
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	"github.com/cloudfoundry/sonde-go/events"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Println("starting UDP Forwarder...")
	defer log.Println("closing UDP Forwarder...")

	cfg := app.LoadConfig(log)

	tlsConfig, err := loggregator.NewIngressTLSConfig(
		cfg.LoggregatorAgentGRPC.CAFile,
		cfg.LoggregatorAgentGRPC.CertFile,
		cfg.LoggregatorAgentGRPC.KeyFile,
	)
	if err != nil {
		log.Fatalf("Failed to create loggregator agent credentials: %s", err)
	}

	v2Ingress, err := loggregator.NewIngressClient(
		tlsConfig,
		loggregator.WithLogger(log),
		loggregator.WithAddr(cfg.LoggregatorAgentGRPC.Addr),
	)
	if err != nil {
		log.Fatalf("Failed to create loggregator agent client: %s", err)
	}

	metrics := metrics.New(expvar.NewMap("UDPForwarder"))

	dropsondeUnmarshaller := ingress.NewUnMarshaller(v2Writer{v2Ingress})
	networkReader, err := ingress.NewNetworkReader(
		fmt.Sprintf("127.0.0.1:%d", cfg.UDPPort),
		dropsondeUnmarshaller,
		metrics,
	)
	if err != nil {
		log.Fatalf("Failed to listen on 127.0.0.1:%d: %s", cfg.UDPPort, err)
	}

	go func() {
		http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", cfg.DebugPort), nil)
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
