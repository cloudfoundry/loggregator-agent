package app

import (
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/loggregator-agent/pkg/collector"
)

type SystemMetricsAgent struct {
	cfg      Config
	log      *log.Logger
	debugLis net.Listener
}

func NewSystemMetricsAgent(cfg Config, log *log.Logger) *SystemMetricsAgent {
	return &SystemMetricsAgent{
		cfg: cfg,
		log: log,
	}
}

func (a *SystemMetricsAgent) Run(blocking bool) {
	var err error
	a.debugLis, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", a.cfg.DebugPort))
	if err != nil {
		a.log.Panicf("failed to start debug listener: %s", err)
	}

	if blocking {
		a.run()
		return
	}

	go a.run()
}

func (a *SystemMetricsAgent) DebugAddr() string {
	if a.debugLis == nil {
		return ""
	}

	return a.debugLis.Addr().String()
}

func (a *SystemMetricsAgent) run() {
	go http.Serve(a.debugLis, nil)

	ic, err := loggregator.NewIngressClient(
		a.cfg.TLS.Config(),
		loggregator.WithAddr(a.cfg.LoggregatorAddr),
		loggregator.WithLogger(a.log),
	)
	if err != nil {
		a.log.Panicf("failed to initialize ingress client: %s", err)
	}

	collector.NewProcessor(
		collector.Collect,
		ic.Emit,
		a.cfg.MetricInterval,
		a.log,
	).Run()
}
