package collector

import (
	"log"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

type InputFunc func() (SystemStat, error)
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
	metrics := map[string]*loggregator_v2.GaugeValue{
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
		"system.cpu.user": &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.User,
		},
		"system.cpu.sys": &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.System,
		},
		"system.cpu.idle": &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.Idle,
		},
		"system.cpu.wait": &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.Wait,
		},
	}

	if stat.SystemDisk.Present {
		metrics["system.disk.system.percent"] = &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.SystemDisk.Percent,
		}

		metrics["system.disk.system.inode_percent"] = &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.SystemDisk.InodePercent,
		}
	}

	if stat.EphemeralDisk.Present {
		metrics["system.disk.ephemeral.percent"] = &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.EphemeralDisk.Percent,
		}

		metrics["system.disk.ephemeral.inode_percent"] = &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.EphemeralDisk.InodePercent,
		}
	}

	if stat.PersistentDisk.Present {
		metrics["system.disk.persistent.percent"] = &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.PersistentDisk.Percent,
		}

		metrics["system.disk.persistent.inode_percent"] = &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.PersistentDisk.InodePercent,
		}
	}

	return &loggregator_v2.Gauge{
		Metrics: metrics,
	}
}
