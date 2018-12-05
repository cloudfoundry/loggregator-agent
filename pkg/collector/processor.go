package collector

import (
	"log"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

type InputFunc func(...CollectOption) (SystemStat, error)
type OutputFunc func(*loggregator_v2.Envelope)

type Processor struct {
	in       InputFunc
	out      OutputFunc
	interval time.Duration
	log      *log.Logger
}

func NewProcessor(
	in InputFunc,
	out OutputFunc,
	interval time.Duration,
	log *log.Logger,
) *Processor {
	return &Processor{
		in:       in,
		out:      out,
		interval: interval,
		log:      log,
	}
}

func (p *Processor) Run() {
	t := time.NewTicker(p.interval)

	for range t.C {
		stat, err := p.in()
		if err != nil {
			p.log.Printf("failed to read stats: %s", err)
			continue
		}

		p.out(&loggregator_v2.Envelope{
			Timestamp: time.Now().UnixNano(),
			Message: &loggregator_v2.Envelope_Gauge{
				Gauge: buildGauge(stat),
			},
			Tags: map[string]string{
				"origin": "system-metrics-agent",
			},
		})
	}
}

func buildGauge(stat SystemStat) *loggregator_v2.Gauge {
	return &loggregator_v2.Gauge{
		Metrics: map[string]*loggregator_v2.GaugeValue{
			"system.mem.kb": &loggregator_v2.GaugeValue{
				Unit:  "KiB",
				Value: float64(stat.MemKB),
			},
			"system.mem.percent": &loggregator_v2.GaugeValue{
				Unit:  "Percent",
				Value: stat.MemPercent,
			},
			"system.swap.kb": &loggregator_v2.GaugeValue{
				Unit:  "KiB",
				Value: float64(stat.SwapKB),
			},
			"system.swap.percent": &loggregator_v2.GaugeValue{
				Unit:  "Percent",
				Value: stat.SwapPercent,
			},
			"system.load.1m": &loggregator_v2.GaugeValue{
				Unit:  "Load",
				Value: stat.Load1M,
			},
			"system.load.5m": &loggregator_v2.GaugeValue{
				Unit:  "Load",
				Value: stat.Load5M,
			},
			"system.load.15m": &loggregator_v2.GaugeValue{
				Unit:  "Load",
				Value: stat.Load15M,
			},
		},
	}
}
