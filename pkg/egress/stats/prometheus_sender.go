package stats

import "code.cloudfoundry.org/loggregator-agent/pkg/collector"

type Gauge interface {
	Set(float64)
}

type GaugeRegistry interface {
	Get(gaugeName, origin, unit string, tags map[string]string) Gauge
}

type PromSender struct {
	registry GaugeRegistry
	origin   string
}

func NewPromSender(registry GaugeRegistry, origin string) PromSender {
	return PromSender{
		registry: registry,
		origin:   origin,
	}
}

func (p PromSender) Send(stats collector.SystemStat) {
	p.setSystemStats(stats)
	p.setSystemDiskGauges(stats)
	p.setEphemeralDiskGauges(stats)
	p.setPersistentDiskGauges(stats)

	for _, network := range stats.Networks {
		p.setNetworkGauges(network)
	}
}

func (p PromSender) setSystemStats(stats collector.SystemStat) {
	gauge := p.registry.Get("system_cpu_sys", p.origin, "Percent", nil)
	gauge.Set(float64(stats.CPUStat.System))

	gauge = p.registry.Get("system_cpu_wait", p.origin, "Percent", nil)
	gauge.Set(float64(stats.CPUStat.Wait))

	gauge = p.registry.Get("system_cpu_idle", p.origin, "Percent", nil)
	gauge.Set(float64(stats.CPUStat.Idle))

	gauge = p.registry.Get("system_cpu_user", p.origin, "Percent", nil)
	gauge.Set(float64(stats.CPUStat.User))

	gauge = p.registry.Get("system_mem_kb", p.origin, "KiB", nil)
	gauge.Set(float64(stats.MemKB))

	gauge = p.registry.Get("system_mem_percent", p.origin, "Percent", nil)
	gauge.Set(float64(stats.MemPercent))

	gauge = p.registry.Get("system_swap_kb", p.origin, "KiB", nil)
	gauge.Set(float64(stats.SwapKB))

	gauge = p.registry.Get("system_swap_percent", p.origin, "Percent", nil)
	gauge.Set(float64(stats.SwapPercent))

	gauge = p.registry.Get("system_load_1m", p.origin, "Load", nil)
	gauge.Set(float64(stats.Load1M))

	gauge = p.registry.Get("system_load_5m", p.origin, "Load", nil)
	gauge.Set(float64(stats.Load5M))

	gauge = p.registry.Get("system_load_15m", p.origin, "Load", nil)
	gauge.Set(float64(stats.Load15M))

	if stats.Health.Present {
		var healthValue float64
		if stats.Health.Healthy {
			healthValue = 1.0
		}

		gauge = p.registry.Get("system_healthy", p.origin, "", nil)
		gauge.Set(healthValue)
	}
}

func (p PromSender) setNetworkGauges(network collector.NetworkStat) {
	tags := map[string]string{"network_interface": network.Name}

	gauge := p.registry.Get("system_network_bytes_sent", p.origin, "Bytes", tags)
	gauge.Set(float64(network.BytesSent))

	gauge = p.registry.Get("system_network_bytes_received", p.origin, "Bytes", tags)
	gauge.Set(float64(network.BytesReceived))

	gauge = p.registry.Get("system_network_packets_sent", p.origin, "Packets", tags)
	gauge.Set(float64(network.PacketsSent))

	gauge = p.registry.Get("system_network_packets_received", p.origin, "Packets", tags)
	gauge.Set(float64(network.PacketsReceived))

	gauge = p.registry.Get("system_network_error_in", p.origin, "Frames", tags)
	gauge.Set(float64(network.ErrIn))

	gauge = p.registry.Get("system_network_error_out", p.origin, "Frames", tags)
	gauge.Set(float64(network.ErrOut))

	gauge = p.registry.Get("system_network_drop_in", p.origin, "Packets", tags)
	gauge.Set(float64(network.DropIn))

	gauge = p.registry.Get("system_network_drop_out", p.origin, "Packets", tags)
	gauge.Set(float64(network.DropOut))
}

func (p PromSender) setSystemDiskGauges(stats collector.SystemStat) {
	if !stats.SystemDisk.Present {
		return
	}

	gauge := p.registry.Get("system_disk_system_percent", p.origin, "Percent", nil)
	gauge.Set(float64(stats.SystemDisk.Percent))

	gauge = p.registry.Get("system_disk_system_inode_percent", p.origin, "Percent", nil)
	gauge.Set(float64(stats.SystemDisk.InodePercent))

	gauge = p.registry.Get("system_disk_system_read_bytes", p.origin, "Bytes", nil)
	gauge.Set(float64(stats.SystemDisk.ReadBytes))

	gauge = p.registry.Get("system_disk_system_write_bytes", p.origin, "Bytes", nil)
	gauge.Set(float64(stats.SystemDisk.WriteBytes))

	gauge = p.registry.Get("system_disk_system_read_time", p.origin, "ms", nil)
	gauge.Set(float64(stats.SystemDisk.ReadTime))

	gauge = p.registry.Get("system_disk_system_write_time", p.origin, "ms", nil)
	gauge.Set(float64(stats.SystemDisk.WriteTime))

	gauge = p.registry.Get("system_disk_system_io_time", p.origin, "ms", nil)
	gauge.Set(float64(stats.SystemDisk.IOTime))
}

func (p PromSender) setEphemeralDiskGauges(stats collector.SystemStat) {
	if !stats.EphemeralDisk.Present {
		return
	}

	gauge := p.registry.Get("system_disk_ephemeral_percent", p.origin, "Percent", nil)
	gauge.Set(float64(stats.EphemeralDisk.Percent))

	gauge = p.registry.Get("system_disk_ephemeral_inode_percent", p.origin, "Percent", nil)
	gauge.Set(float64(stats.EphemeralDisk.InodePercent))

	gauge = p.registry.Get("system_disk_ephemeral_read_bytes", p.origin, "Bytes", nil)
	gauge.Set(float64(stats.EphemeralDisk.ReadBytes))

	gauge = p.registry.Get("system_disk_ephemeral_write_bytes", p.origin, "Bytes", nil)
	gauge.Set(float64(stats.EphemeralDisk.WriteBytes))

	gauge = p.registry.Get("system_disk_ephemeral_read_time", p.origin, "ms", nil)
	gauge.Set(float64(stats.EphemeralDisk.ReadTime))

	gauge = p.registry.Get("system_disk_ephemeral_write_time", p.origin, "ms", nil)
	gauge.Set(float64(stats.EphemeralDisk.WriteTime))

	gauge = p.registry.Get("system_disk_ephemeral_io_time", p.origin, "ms", nil)
	gauge.Set(float64(stats.EphemeralDisk.IOTime))
}

func (p PromSender) setPersistentDiskGauges(stats collector.SystemStat) {
	if !stats.PersistentDisk.Present {
		return
	}

	gauge := p.registry.Get("system_disk_persistent_percent", p.origin, "Percent", nil)
	gauge.Set(float64(stats.PersistentDisk.Percent))

	gauge = p.registry.Get("system_disk_persistent_inode_percent", p.origin, "Percent", nil)
	gauge.Set(float64(stats.PersistentDisk.InodePercent))

	gauge = p.registry.Get("system_disk_persistent_read_bytes", p.origin, "Bytes", nil)
	gauge.Set(float64(stats.PersistentDisk.ReadBytes))

	gauge = p.registry.Get("system_disk_persistent_write_bytes", p.origin, "Bytes", nil)
	gauge.Set(float64(stats.PersistentDisk.WriteBytes))

	gauge = p.registry.Get("system_disk_persistent_read_time", p.origin, "ms", nil)
	gauge.Set(float64(stats.PersistentDisk.ReadTime))

	gauge = p.registry.Get("system_disk_persistent_write_time", p.origin, "ms", nil)
	gauge.Set(float64(stats.PersistentDisk.WriteTime))

	gauge = p.registry.Get("system_disk_persistent_io_time", p.origin, "ms", nil)
	gauge.Set(float64(stats.PersistentDisk.IOTime))
}
