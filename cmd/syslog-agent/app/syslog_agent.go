package app

import (
	"fmt"
	"log"

	"net/http"
	_ "net/http/pprof"

	gendiodes "code.cloudfoundry.org/go-diodes"
	"code.cloudfoundry.org/loggregator-agent/pkg/diodes"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"google.golang.org/grpc"
)

// SyslogAgent manages starting the syslog agent service.
type SyslogAgent struct {
	debugPort      uint16
	m              Metrics
	bindingManager BindingManager
	grpc           GRPC
	log            *log.Logger
}

type Metrics interface {
	NewGauge(string) func(float64)
	NewCounter(name string) func(uint64)
}

type BindingManager interface {
	Run()
	GetDrains(string) []egress.Writer
}

// NewSyslogAgent intializes and returns a new syslog agent.
func NewSyslogAgent(
	debugPort uint16,
	m Metrics,
	bm BindingManager,
	grpc GRPC,
	log *log.Logger,
) *SyslogAgent {
	return &SyslogAgent{
		debugPort:      debugPort,
		bindingManager: bm,
		grpc:           grpc,
		m:              m,
		log:            log,
	}
}

func (s SyslogAgent) Run() {
	go http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", s.debugPort), nil)

	ingressDropped := s.m.NewCounter("IngressDropped")
	diode := diodes.NewManyToOneEnvelopeV2(10000, gendiodes.AlertFunc(func(missed int) {
		ingressDropped(uint64(missed))
	}))

	drainIngress := s.m.NewCounter("DrainIngress")
	go s.bindingManager.Run()
	go func() {
		for {
			e := diode.Next()

			drainWriters := s.bindingManager.GetDrains(e.SourceId)
			for _, w := range drainWriters {
				drainIngress(1)

				// Ignore this because we typically wrap everything in a diode
				// writer which doesn't return an error
				_ = w.Write(e)
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
