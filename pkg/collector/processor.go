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
		"system_mem_kb": &loggregator_v2.GaugeValue{
			Unit:  "KiB",
			Value: float64(stat.MemKB),
		},
		"system_mem_percent": &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.MemPercent,
		},
		"system_swap_kb": &loggregator_v2.GaugeValue{
			Unit:  "KiB",
			Value: float64(stat.SwapKB),
		},
		"system_swap_percent": &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.SwapPercent,
		},
		"system_load_1m": &loggregator_v2.GaugeValue{
			Unit:  "Load",
			Value: stat.Load1M,
		},
		"system_load_5m": &loggregator_v2.GaugeValue{
			Unit:  "Load",
			Value: stat.Load5M,
		},
		"system_load_15m": &loggregator_v2.GaugeValue{
			Unit:  "Load",
			Value: stat.Load15M,
		},
		"system_cpu_user": &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.User,
		},
		"system_cpu_sys": &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.System,
		},
		"system_cpu_idle": &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.Idle,
		},
		"system_cpu_wait": &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.Wait,
		},
	}

	if stat.SystemDisk.Present {
		metrics["system_disk_system_percent"] = &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.SystemDisk.Percent,
		}

		metrics["system_disk_system_inode_percent"] = &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.SystemDisk.InodePercent,
		}
	}

	if stat.EphemeralDisk.Present {
		metrics["system_disk_ephemeral_percent"] = &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.EphemeralDisk.Percent,
		}

		metrics["system_disk_ephemeral_inode_percent"] = &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.EphemeralDisk.InodePercent,
		}
	}

	if stat.PersistentDisk.Present {
		metrics["system_disk_persistent_percent"] = &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.PersistentDisk.Percent,
		}

		metrics["system_disk_persistent_inode_percent"] = &loggregator_v2.GaugeValue{
			Unit:  "Percent",
			Value: stat.PersistentDisk.InodePercent,
		}
	}

	return &loggregator_v2.Gauge{
		Metrics: metrics,
	}
}
