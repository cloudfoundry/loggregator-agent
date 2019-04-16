package app

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"fmt"
	"log"
	"time"

	"net/http"
	_ "net/http/pprof"

	gendiodes "code.cloudfoundry.org/go-diodes"
	"code.cloudfoundry.org/loggregator-agent/pkg/binding"
	"code.cloudfoundry.org/loggregator-agent/pkg/cache"
	"code.cloudfoundry.org/loggregator-agent/pkg/diodes"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"code.cloudfoundry.org/loggregator-agent/pkg/timeoutwaitgroup"
	"google.golang.org/grpc"
)

// SyslogAgent manages starting the syslog agent service.
type SyslogAgent struct {
	pprofPort           uint16
	metrics             Metrics
	bindingManager      BindingManager
	grpc                GRPC
	log                 *log.Logger
	cache               Cache
	bindingsPerAppLimit int
	drainSkipCertVerify bool
}

type Metrics interface {
	NewGauge(name string, options ...metrics.MetricOption) metrics.Gauge
	NewCounter(name string, options ...metrics.MetricOption) metrics.Counter
}

type BindingManager interface {
	Run()
	GetDrains(string) []egress.Writer
}

// NewSyslogAgent intializes and returns a new syslog agent.
func NewSyslogAgent(
	cfg Config,
	m Metrics,
	l *log.Logger,
) *SyslogAgent {
	connector := syslog.NewSyslogConnector(
		syslog.NetworkTimeoutConfig{
			Keepalive:    10 * time.Second,
			DialTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		cfg.DrainSkipCertVerify,
		timeoutwaitgroup.New(time.Minute),
		syslog.NewWriterFactory(m),
		m,
	)

	tlsClient := plumbing.NewTLSHTTPClient(
		cfg.Cache.CertFile,
		cfg.Cache.KeyFile,
		cfg.Cache.CAFile,
		cfg.Cache.CommonName,
	)
	cacheClient := cache.NewClient(cfg.Cache.URL, tlsClient)
	fetcher := cups.NewFilteredBindingFetcher(
		&cfg.Cache.Blacklist,
		cups.NewBindingFetcher(cfg.BindingsPerAppLimit, cacheClient, m),
		m,
		l,
	)
	bindingManager := binding.NewManager(
		fetcher,
		connector,
		m,
		cfg.Cache.PollingInterval,
		cfg.IdleDrainTimeout,
		l,
	)

	return &SyslogAgent{
		pprofPort:           cfg.DebugPort,
		grpc:                cfg.GRPC,
		metrics:             m,
		log:                 l,
		bindingsPerAppLimit: cfg.BindingsPerAppLimit,
		drainSkipCertVerify: cfg.DrainSkipCertVerify,
		bindingManager:      bindingManager,
	}
}

func (s *SyslogAgent) Run() {
	go http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", s.pprofPort), nil)

	ingressDropped := s.metrics.NewCounter("dropped", metrics.WithMetricTags(map[string]string{"direction": "ingress"}))
	diode := diodes.NewManyToOneEnvelopeV2(10000, gendiodes.AlertFunc(func(missed int) {
		ingressDropped.Add(float64(missed))
	}))

	drainIngress := s.metrics.NewCounter("ingress", metrics.WithMetricTags(map[string]string{"scope": "all_drains"}))
	go s.bindingManager.Run()
	go func() {
		for {
			e := diode.Next()

			drainWriters := s.bindingManager.GetDrains(e.SourceId)
			for _, w := range drainWriters {
				drainIngress.Add(1)

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

	im := s.metrics.NewCounter("ingress", metrics.WithMetricTags(map[string]string{"scope": "agent"}))
	omm := s.metrics.NewCounter("origin_mappings")
	rx := v2.NewReceiver(diode, im, omm)

	srv := v2.NewServer(
		fmt.Sprintf("127.0.0.1:%d", s.grpc.Port),
		rx,
		grpc.Creds(serverCreds),
	)
	srv.Start()
}
