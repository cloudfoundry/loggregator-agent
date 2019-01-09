package app

import (
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/loggregator-agent/pkg/collector"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/stats"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const statOrigin = "system-metrics-agent"

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
	ic, err := loggregator.NewIngressClient(
		a.cfg.TLS.Config(),
		loggregator.WithAddr(a.cfg.LoggregatorAddr),
		loggregator.WithLogger(a.log),
	)
	if err != nil {
		a.log.Panicf("failed to initialize ingress client: %s", err)
	}

	loggregatorSender := stats.NewLoggregatorSender(ic.Emit, statOrigin)

	promRegisterer := prometheus.NewRegistry()
	promRegistry := stats.NewPromRegistry(promRegisterer)
	promSender := stats.NewPromSender(promRegistry, statOrigin)

	go http.Serve(a.debugLis, promhttp.HandlerFor(promRegisterer, promhttp.HandlerOpts{}))

	c := collector.New(a.log)
	collector.NewProcessor(
		c.Collect,
		[]collector.StatsSender{loggregatorSender, promSender},
		a.cfg.SampleInterval,
		a.log,
	).Run()
}
