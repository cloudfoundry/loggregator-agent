package app

import (
	"fmt"
	"log"
	"time"

	"net/http"
	_ "net/http/pprof"

	gendiodes "code.cloudfoundry.org/go-diodes"
	"code.cloudfoundry.org/loggregator-agent/pkg/binding"
	"code.cloudfoundry.org/loggregator-agent/pkg/diodes"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"code.cloudfoundry.org/loggregator-agent/pkg/timeoutwaitgroup"
	"google.golang.org/grpc"
)

// SyslogAgent manages starting the syslog agent service.
type SyslogAgent struct {
	debugPort       uint16
	pollingInterval time.Duration
	m               Metrics
	bf              BindingFetcher
	grpc            GRPC
	skipCertVerify  bool
	log             *log.Logger
}

type Metrics interface {
	NewGauge(string) func(float64)
	NewCounter(name string) func(uint64)
}

type BindingFetcher interface {
	FetchBindings() ([]syslog.Binding, error)
}

// NewSyslogAgent intializes and returns a new syslog agent.
func NewSyslogAgent(
	debugPort uint16,
	m Metrics,
	bf BindingFetcher,
	pi time.Duration,
	grpc GRPC,
	skipCertVerify bool,
	log *log.Logger,
) *SyslogAgent {
	return &SyslogAgent{
		debugPort:       debugPort,
		pollingInterval: pi,
		bf:              bf,
		grpc:            grpc,
		m:               m,
		skipCertVerify:  skipCertVerify,
		log:             log,
	}
}

// Run starts all the sub-processes of the syslog agent. If blocking is
// true this method will block otherwise it will return immediately and run
// the syslog agent a goroutine.
func (s SyslogAgent) Run(blocking bool) {
	if blocking {
		s.run()
		return
	}

	go s.run()
}

func (s SyslogAgent) run() {
	go http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", s.debugPort), nil)

	ingressDropped := s.m.NewCounter("IngressDropped")
	diode := diodes.NewManyToOneEnvelopeV2(10000, gendiodes.AlertFunc(func(missed int) {
		ingressDropped(uint64(missed))
	}))

	constructors := map[string]syslog.WriterConstructor{
		"https":      syslog.NewHTTPSWriter,
		"syslog-tls": syslog.NewTLSWriter,
		"syslog":     syslog.NewTCPWriter,
	}

	connector := syslog.NewSyslogConnector(
		syslog.NetworkTimeoutConfig{
			Keepalive:    10 * time.Second,
			DialTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		s.skipCertVerify,
		timeoutwaitgroup.New(time.Minute),
		syslog.WithConstructors(constructors),
	)

	bindingManager := binding.NewManager(
		s.bf,
		connector,
		s.m,
		s.pollingInterval,
		s.log,
	)
	go bindingManager.Run()

	go func() {
		for {
			e := diode.Next()

			drainWriters := bindingManager.GetDrains(e.SourceId)
			for _, w := range drainWriters {
				w.Write(e)
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
