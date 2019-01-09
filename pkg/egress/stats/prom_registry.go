package stats

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type PromRegistry struct {
	Registerer prometheus.Registerer
	gauges     map[string]prometheus.Gauge
}

func NewPromRegistry(r prometheus.Registerer) *PromRegistry {
	return &PromRegistry{
		Registerer: r,
		gauges:     make(map[string]prometheus.Gauge),
	}
}

func (r *PromRegistry) Get(name, origin, unit string, tags map[string]string) Gauge {
	gaugeName := gaugeName(name, origin, unit, tags)
	g, ok := r.gauges[gaugeName]
	if ok {
		return g
	}

	if tags == nil {
		tags = make(map[string]string)
	}
	tags["unit"] = unit

	g = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   origin,
			Subsystem:   "",
			Name:        name,
			Help:        "vm metric",
			ConstLabels: tags,
		},
	)
	r.gauges[gaugeName] = g
	r.Registerer.Register(g)
	return g
}

func gaugeName(name, origin, unit string, tags map[string]string) string {
	return fmt.Sprintf("%s_%s_%s", name, origin, tags)
}
