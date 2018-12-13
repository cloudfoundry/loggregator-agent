package testhelper

import (
	"fmt"
	"sync"
)

type SpyMetricClient struct {
	metrics map[string]*SpyMetric
}

func NewMetricClient() *SpyMetricClient {
	return &SpyMetricClient{
		metrics: make(map[string]*SpyMetric),
	}
}

func (s *SpyMetricClient) NewCounter(name string) func(uint64) {
	m := &SpyMetric{}
	s.metrics[name] = m

	return func(delta uint64) {
		m.Increment(delta)
	}
}

func (s *SpyMetricClient) NewGauge(name string) func(float64) {
	m := &SpyMetric{}
	s.metrics[name] = m

	return func(value float64) {
		m.Set(value)
	}
}

func (s *SpyMetricClient) NewSumGauge(name string) func(float64) {
	m := &SpyMetric{}
	s.metrics[name] = m

	return func(value float64) {
		m.Add(value)
	}
}

func (s *SpyMetricClient) GetMetric(name string) *SpyMetric {
	if m, ok := s.metrics[name]; ok {
		return m
	}

	panic(fmt.Sprintf("unknown metric: %s", name))
}

func (s *SpyMetricClient) HasMetric(name string) bool {
	_, ok := s.metrics[name]
	return ok
}

type SpyMetric struct {
	mu         sync.Mutex
	delta      uint64
	gaugeValue float64
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
