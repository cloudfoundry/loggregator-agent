package collector

import (
	"log"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

const envelopeOrigin = "system-metrics-agent"

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

		ts := time.Now().UnixNano()
		p.out(&loggregator_v2.Envelope{
			Timestamp: ts,
			Message: &loggregator_v2.Envelope_Gauge{
				Gauge: buildGauge(stat),
			},
			Tags: map[string]string{
				"origin": envelopeOrigin,
			},
		})

		for _, network := range stat.Networks {
			p.out(&loggregator_v2.Envelope{
				Timestamp: ts,
				Message: &loggregator_v2.Envelope_Gauge{
					Gauge: buildNetworkGauge(network),
				},
				Tags: map[string]string{
					"network_interface": network.Name,
					"origin":            envelopeOrigin,
				},
			})
		}
	}
}

func buildNetworkGauge(network NetworkStat) *loggregator_v2.Gauge {
	metrics := map[string]*loggregator_v2.GaugeValue{
		"system_network_bytes_sent": &loggregator_v2.GaugeValue{
			Unit:  "Bytes",
			Value: float64(network.BytesSent),
		},
		"system_network_bytes_received": &loggregator_v2.GaugeValue{
			Unit:  "Bytes",
			Value: float64(network.BytesReceived),
		},
		"system_network_packets_sent": &loggregator_v2.GaugeValue{
			Unit:  "Packets",
			Value: float64(network.PacketsSent),
		},
		"system_network_packets_received": &loggregator_v2.GaugeValue{
			Unit:  "Packets",
			Value: float64(network.PacketsReceived),
		},
		"system_network_error_in": &loggregator_v2.GaugeValue{
			Unit:  "Frames",
			Value: float64(network.ErrIn),
		},
		"system_network_error_out": &loggregator_v2.GaugeValue{
			Unit:  "Frames",
			Value: float64(network.ErrOut),
		},
		"system_network_drop_in": &loggregator_v2.GaugeValue{
			Unit:  "Packets",
			Value: float64(network.DropIn),
		},
		"system_network_drop_out": &loggregator_v2.GaugeValue{
			Unit:  "Packets",
			Value: float64(network.DropOut),
		},
	}

	return &loggregator_v2.Gauge{
		Metrics: metrics,
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

	if stat.ProtoCounters.Present {
		metrics["system_network_ip_forwarding"] = &loggregator_v2.GaugeValue{
			Unit:  "",
			Value: float64(stat.ProtoCounters.IPForwarding),
		}

		metrics["system_network_udp_no_ports"] = &loggregator_v2.GaugeValue{
			Unit:  "",
			Value: float64(stat.ProtoCounters.UDPNoPorts),
		}

		metrics["system_network_udp_in_errors"] = &loggregator_v2.GaugeValue{
			Unit:  "",
			Value: float64(stat.ProtoCounters.UDPInErrors),
		}

		metrics["system_network_udp_lite_in_errors"] = &loggregator_v2.GaugeValue{
			Unit:  "",
			Value: float64(stat.ProtoCounters.UDPLiteInErrors),
		}

		metrics["system_network_tcp_active_opens"] = &loggregator_v2.GaugeValue{
			Unit:  "",
			Value: float64(stat.ProtoCounters.TCPActiveOpens),
		}

		metrics["system_network_tcp_curr_estab"] = &loggregator_v2.GaugeValue{
			Unit:  "",
			Value: float64(stat.ProtoCounters.TCPCurrEstab),
		}

		metrics["system_network_tcp_retrans_segs"] = &loggregator_v2.GaugeValue{
			Unit:  "",
			Value: float64(stat.ProtoCounters.TCPRetransSegs),
		}
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

		metrics["system_disk_system_read_bytes"] = &loggregator_v2.GaugeValue{
			Unit:  "Bytes",
			Value: float64(stat.SystemDisk.ReadBytes),
		}

		metrics["system_disk_system_write_bytes"] = &loggregator_v2.GaugeValue{
			Unit:  "Bytes",
			Value: float64(stat.SystemDisk.WriteBytes),
		}

		metrics["system_disk_system_read_time"] = &loggregator_v2.GaugeValue{
			Unit:  "ms",
			Value: float64(stat.SystemDisk.ReadTime),
		}

		metrics["system_disk_system_write_time"] = &loggregator_v2.GaugeValue{
			Unit:  "ms",
			Value: float64(stat.SystemDisk.WriteTime),
		}

		metrics["system_disk_system_io_time"] = &loggregator_v2.GaugeValue{
			Unit:  "ms",
			Value: float64(stat.SystemDisk.IOTime),
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

		metrics["system_disk_ephemeral_read_bytes"] = &loggregator_v2.GaugeValue{
			Unit:  "Bytes",
			Value: float64(stat.EphemeralDisk.ReadBytes),
		}

		metrics["system_disk_ephemeral_write_bytes"] = &loggregator_v2.GaugeValue{
			Unit:  "Bytes",
			Value: float64(stat.EphemeralDisk.WriteBytes),
		}

		metrics["system_disk_ephemeral_read_time"] = &loggregator_v2.GaugeValue{
			Unit:  "ms",
			Value: float64(stat.EphemeralDisk.ReadTime),
		}

		metrics["system_disk_ephemeral_write_time"] = &loggregator_v2.GaugeValue{
			Unit:  "ms",
			Value: float64(stat.EphemeralDisk.WriteTime),
		}

		metrics["system_disk_ephemeral_io_time"] = &loggregator_v2.GaugeValue{
			Unit:  "ms",
			Value: float64(stat.EphemeralDisk.IOTime),
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

		metrics["system_disk_persistent_read_bytes"] = &loggregator_v2.GaugeValue{
			Unit:  "Bytes",
			Value: float64(stat.PersistentDisk.ReadBytes),
		}

		metrics["system_disk_persistent_write_bytes"] = &loggregator_v2.GaugeValue{
			Unit:  "Bytes",
			Value: float64(stat.PersistentDisk.WriteBytes),
		}

		metrics["system_disk_persistent_read_time"] = &loggregator_v2.GaugeValue{
			Unit:  "ms",
			Value: float64(stat.PersistentDisk.ReadTime),
		}

		metrics["system_disk_persistent_write_time"] = &loggregator_v2.GaugeValue{
			Unit:  "ms",
			Value: float64(stat.PersistentDisk.WriteTime),
		}

		metrics["system_disk_persistent_io_time"] = &loggregator_v2.GaugeValue{
			Unit:  "ms",
			Value: float64(stat.PersistentDisk.IOTime),
		}
	}

	if stat.Health.Present {
		var healthValue float64
		if stat.Health.Healthy {
			healthValue = 1.0
		}

		metrics["system_healthy"] = &loggregator_v2.GaugeValue{
			Value: healthValue,
		}
	}

	return &loggregator_v2.Gauge{
		Metrics: metrics,
	}
}
