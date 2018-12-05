package app

import (
	"log"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/loggregator-agent/pkg/collector"
)

type SystemMetricsAgent struct {
	cfg Config
	log *log.Logger
}

func NewSystemMetricsAgent(cfg Config, log *log.Logger) *SystemMetricsAgent {
	return &SystemMetricsAgent{
		cfg: cfg,
		log: log,
	}
}

func (a *SystemMetricsAgent) Run(blocking bool) {
	if blocking {
		a.run()
		return
	}

	go a.run()
}

func (a *SystemMetricsAgent) Stop() {
}

func (a *SystemMetricsAgent) run() {
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
