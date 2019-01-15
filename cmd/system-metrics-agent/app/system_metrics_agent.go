package app

import (
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"

	"code.cloudfoundry.org/loggregator-agent/pkg/collector"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/stats"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const statOrigin = "system_metrics_agent"

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

	promRegisterer := prometheus.NewRegistry()
	promRegistry := stats.NewPromRegistry(promRegisterer)
	promSender := stats.NewPromSender(promRegistry, statOrigin)

	metricsURL := fmt.Sprintf("127.0.0.1:%d", a.cfg.MetricPort)
	startMetricsServer(metricsURL, promRegisterer)

	c := collector.New(a.log)
	collector.NewProcessor(
		c.Collect,
		[]collector.StatsSender{promSender},
		a.cfg.SampleInterval,
		a.log,
	).Run()
}

func startMetricsServer(addr string, gatherer prometheus.Gatherer) net.Listener {
	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))

	server := http.Server{
		Addr:         addr,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      router,
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Unable to setup metrics endpoint (%s): %s", addr, err)
	}

	go func() {
		log.Printf("Metrics endpoint is listening on %s", lis.Addr().String())
		log.Printf("Metrics server closing: %s", server.Serve(lis))
	}()
	return lis
}
