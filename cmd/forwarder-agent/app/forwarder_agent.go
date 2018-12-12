package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"net/http"
	_ "net/http/pprof"

	gendiodes "code.cloudfoundry.org/go-diodes"
	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/diodes"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"code.cloudfoundry.org/loggregator-agent/pkg/timeoutwaitgroup"
	"google.golang.org/grpc"
)

// ForwarderAgent manages starting the forwarder agent service.
type ForwarderAgent struct {
	debugPort       uint16
	m               Metrics
	grpc            GRPC
	downstreamAddrs []string
	log             *log.Logger
}

type Metrics interface {
	NewGauge(string) func(float64)
	NewCounter(name string) func(uint64)
}

type BindingFetcher interface {
	FetchBindings() ([]syslog.Binding, error)
}

type Writer interface {
	Write(*loggregator_v2.Envelope) error
}

// NewForwarderAgent intializes and returns a new forwarder agent.
func NewForwarderAgent(
	debugPort uint16,
	m Metrics,
	grpc GRPC,
	downstreamAddrs []string,
	log *log.Logger,
) *ForwarderAgent {
	return &ForwarderAgent{
		debugPort:       debugPort,
		grpc:            grpc,
		m:               m,
		downstreamAddrs: downstreamAddrs,
		log:             log,
	}
}

// Run starts all the sub-processes of the forwarder agent. If blocking is
// true this method will block otherwise it will return immediately and run
// the forwarder agent a goroutine.
func (s ForwarderAgent) Run(blocking bool) {
	if blocking {
		s.run()
		return
	}

	go s.run()
}

func (s ForwarderAgent) run() {
	go http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", s.debugPort), nil)

	ingressDropped := s.m.NewCounter("IngressDropped")
	diode := diodes.NewManyToOneEnvelopeV2(10000, gendiodes.AlertFunc(func(missed int) {
		ingressDropped(uint64(missed))
	}))

	clients := ingressClients(s.downstreamAddrs, s.grpc, s.log)
	go func() {
		for {
			e := diode.Next()
			for _, c := range clients {
				c.Write(e)
			}
		}
	}()

	var opts []plumbing.ConfigOption
	if len(s.grpc.CipherSuites) > 0 {
		opts = append(opts, plumbing.WithCipherSuites(s.grpc.CipherSuites))
	}

	serverCreds, err := plumbing.NewServerCredentials(
		s.grpc.CertFile,
		s.grpc.KeyFile,
		s.grpc.CAFile,
		opts...,
	)
	if err != nil {
		s.log.Fatalf("failed to configure server TLS: %s", err)
	}

	rx := v2.NewReceiver(diode, s.m)
	srv := v2.NewServer(
		fmt.Sprintf("127.0.0.1:%d", s.grpc.Port),
		rx,
		grpc.Creds(serverCreds),
	)
	srv.Start()
}

type clientWriter struct {
	c *loggregator.IngressClient
}

func (c clientWriter) Write(e *loggregator_v2.Envelope) error {
	c.c.Emit(e)
	return nil
}

func (c clientWriter) Close() error {
	return c.c.CloseSend()
}

func ingressClients(downstreamAddrs []string, grpc GRPC, l *log.Logger) []Writer {
	var ingressClients []Writer
	for _, addr := range downstreamAddrs {
		clientCreds, err := loggregator.NewIngressTLSConfig(
			grpc.CAFile,
			grpc.CertFile,
			grpc.KeyFile,
		)
		if err != nil {
			l.Fatalf("failed to configure client TLS: %s", err)
		}

		il := log.New(os.Stderr, fmt.Sprintf("[INGRESS CLIENT] -> %s: ", addr), log.LstdFlags)
		ingressClient, err := loggregator.NewIngressClient(
			clientCreds,
			loggregator.WithLogger(il),
			loggregator.WithAddr(addr),
		)
		if err != nil {
			l.Fatalf("failed to create ingress client for %s: %s", addr, err)
		}

		ctx := context.Background()
		wc := clientWriter{ingressClient}
		dw := egress.NewDiodeWriter(ctx, wc, gendiodes.AlertFunc(func(missed int) {
			il.Printf("Dropped %d logs for url %s", missed, addr)
		}), timeoutwaitgroup.New(time.Minute))

		ingressClients = append(ingressClients, dw)
	}
	return ingressClients
}
