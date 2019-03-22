package testhelper

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"sort"
	"sync"
)

type SpyMetricClient struct {
	mu      sync.Mutex
	metrics map[string]*SpyMetric
}

func NewMetricClient() *SpyMetricClient {
	return &SpyMetricClient{
		metrics: make(map[string]*SpyMetric),
	}
}

func (s *SpyMetricClient) NewCounter(name string, opts ...metrics.MetricOption) (metrics.Counter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m := newSpyMetric(name, opts)
	s.addMetric(m)

	return m, nil
}

func (s *SpyMetricClient) NewGauge(name string, opts ...metrics.MetricOption) (metrics.Gauge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m := newSpyMetric(name, opts)
	s.addMetric(m)

	return m, nil
}

func (s *SpyMetricClient) addMetric(sm *SpyMetric) {
	n := getMetricName(sm.name, sm.Opts.ConstLabels)

	s.metrics[n] = sm
}

func (s *SpyMetricClient) GetMetric(name string, tags map[string]string) *SpyMetric {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := getMetricName(name, tags)

	if m, ok := s.metrics[n]; ok {
		return m
	}

	panic(fmt.Sprintf("unknown metric: %s", name))
}

func (s *SpyMetricClient) HasMetric(name string, tags map[string]string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := getMetricName(name, tags)
	_, ok := s.metrics[n]
	return ok
}

func newSpyMetric(name string, opts []metrics.MetricOption) *SpyMetric {
	sm := &SpyMetric{
		name: name,
		Opts: &prometheus.Opts{
			ConstLabels: make(prometheus.Labels),
		},
	}

	for _, o := range opts {
		o(sm.Opts)
	}

	for k, _ := range sm.Opts.ConstLabels {
		sm.keys = append(sm.keys, k)
	}
	sort.Strings(sm.keys)

	return sm
}

type SpyMetric struct {
	mu         sync.Mutex
	delta      uint64
	gaugeValue float64
	name       string

	keys []string
	Opts *prometheus.Opts
}

func (s *SpyMetric) Increment(c uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delta += c
}

func (s *SpyMetric) Set(c float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gaugeValue = c
}

func (s *SpyMetric) Add(c float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gaugeValue += c
}

func (s *SpyMetric) Delta() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.delta
}

func (s *SpyMetric) GaugeValue() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gaugeValue
}

func getMetricName(name string, tags map[string]string) string {
	n := name

	k := make([]string, len(tags))
	for t := range tags {
		k = append(k, t)
	}
	sort.Strings(k)

	for _, key := range k {
		n += fmt.Sprintf("%s_%s", key, tags[key])
	}

	return n
}
