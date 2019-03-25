package metrics

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type PromRegistry struct {
	port        string
	registry    *prometheus.Registry
	defaultTags map[string]string
}

type Counter interface {
	Add(float64)
}

type Gauge interface {
	Add(float64)
	Set(float64)
}

func NewPromRegistry(defaultSourceID string, port int, logger *log.Logger, opts ...RegistryOption) *PromRegistry {
	registry := prometheus.NewRegistry()
	p := serveRegistry(registry, port, logger)

	pr := &PromRegistry{
		registry:    registry,
		defaultTags: map[string]string{"source_id": defaultSourceID, "origin": defaultSourceID},
		port:        p,
	}

	for _, o := range opts {
		o(pr)
	}

	return pr
}

type RegistryOption func(r *PromRegistry)

func WithDefaultTags(tags map[string]string) RegistryOption {
	return func(r *PromRegistry) {
		for k, v := range tags {
			r.defaultTags[k] = v
		}
	}
}

func serveRegistry(registry *prometheus.Registry, port int, logger *log.Logger) string {
	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	s := http.Server{
		Addr:         addr,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      router,
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatalf("Unable to setup metrics endpoint (%s): %s", addr, err)
	}
	logger.Printf("Metrics endpoint is listening on %s", lis.Addr().String())

	go s.Serve(lis)

	parts := strings.Split(lis.Addr().String(), ":")
	return parts[len(parts)-1]
}

func (p *PromRegistry) NewCounter(name string, opts ...MetricOption) (Counter, error) {
	opt := p.newMetricOpt(name, "counter metric", opts...)
	counter := prometheus.NewCounter(prometheus.CounterOpts(opt))

	err := p.registry.Register(counter)
	if err != nil {
		return nil, err
	}
	return counter, nil
}

func (p *PromRegistry) NewGauge(name string, opts ...MetricOption) (Gauge, error) {
	opt := p.newMetricOpt(name, "gauge metric", opts...)
	gauge := prometheus.NewGauge(prometheus.GaugeOpts(opt))

	err := p.registry.Register(gauge)
	if err != nil {
		return nil, err
	}
	return gauge, nil
}

func (p *PromRegistry) Port() string {
	return fmt.Sprint(p.port)
}

func (p *PromRegistry) newMetricOpt(name, helpText string, mOpts ...MetricOption) prometheus.Opts {
	opt := prometheus.Opts{
		Name:        name,
		Help:        helpText,
		ConstLabels: make(map[string]string),
	}

	for _, o := range mOpts {
		o(&opt)
	}

	for k, v := range p.defaultTags {
		opt.ConstLabels[k] = v
	}

	return opt
}

func WithMetricTags(tags map[string]string) MetricOption {
	return func(o *prometheus.Opts) {
		for k, v := range tags {
			o.ConstLabels[k] = v
		}
	}
}

func WithHelpText(helpText string) MetricOption {
	return func(o *prometheus.Opts) {
		o.Help = helpText
	}
}

type MetricOption func(o *prometheus.Opts)