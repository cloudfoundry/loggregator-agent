package stats_test

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/collector"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/stats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Prometheus Sender", func() {
	var (
		sender   stats.PromSender
		registry *stubRegistry
	)

	BeforeEach(func() {
		registry = newStubRegistry()
		sender = stats.NewPromSender(registry, "test-origin")
	})

	It("gets the correct number of metrics from the registry", func() {
		sender.Send(defaultInput)

		Expect(registry.gaugeCount).To(Equal(40))
	})

	DescribeTable("default metrics", func(name, origin, unit string, value float64) {
		sender.Send(defaultInput)

		gauge := registry.gauges[name+origin+unit]

		Expect(gauge.value).To(BeNumerically("==", value))
	},
		Entry("system_mem_kb", "system_mem_kb", "test-origin", "KiB", 1025.0),
		Entry("system_mem_percent", "system_mem_percent", "test-origin", "Percent", 10.01),
		Entry("system_swap_kb", "system_swap_kb", "test-origin", "KiB", 2049.0),
		Entry("system_swap_percent", "system_swap_percent", "test-origin", "Percent", 20.01),
		Entry("system_load_1m", "system_load_1m", "test-origin", "Load", 1.1),
		Entry("system_load_5m", "system_load_5m", "test-origin", "Load", 5.5),
		Entry("system_load_15m", "system_load_15m", "test-origin", "Load", 15.15),
		Entry("system_cpu_user", "system_cpu_user", "test-origin", "Percent", 25.25),
		Entry("system_cpu_sys", "system_cpu_sys", "test-origin", "Percent", 52.52),
		Entry("system_cpu_idle", "system_cpu_idle", "test-origin", "Percent", 10.10),
		Entry("system_cpu_wait", "system_cpu_wait", "test-origin", "Percent", 22.22),
		Entry("system_disk_system_percent", "system_disk_system_percent", "test-origin", "Percent", 35.0),
		Entry("system_disk_system_inode_percent", "system_disk_system_inode_percent", "test-origin", "Percent", 45.0),
		Entry("system_disk_system_read_bytes", "system_disk_system_read_bytes", "test-origin", "Bytes", 10.0),
		Entry("system_disk_system_write_bytes", "system_disk_system_write_bytes", "test-origin", "Bytes", 20.0),
		Entry("system_disk_system_read_time", "system_disk_system_read_time", "test-origin", "ms", 30.0),
		Entry("system_disk_system_write_time", "system_disk_system_write_time", "test-origin", "ms", 40.0),
		Entry("system_disk_system_io_time", "system_disk_system_io_time", "test-origin", "ms", 50.0),
		Entry("system_disk_ephemeral_percent", "system_disk_ephemeral_percent", "test-origin", "Percent", 55.0),
		Entry("system_disk_ephemeral_inode_percent", "system_disk_ephemeral_inode_percent", "test-origin", "Percent", 65.0),
		Entry("system_disk_ephemeral_read_bytes", "system_disk_ephemeral_read_bytes", "test-origin", "Bytes", 100.0),
		Entry("system_disk_ephemeral_write_bytes", "system_disk_ephemeral_write_bytes", "test-origin", "Bytes", 200.0),
		Entry("system_disk_ephemeral_read_time", "system_disk_ephemeral_read_time", "test-origin", "ms", 300.0),
		Entry("system_disk_ephemeral_write_time", "system_disk_ephemeral_write_time", "test-origin", "ms", 400.0),
		Entry("system_disk_ephemeral_io_time", "system_disk_ephemeral_io_time", "test-origin", "ms", 500.0),
		Entry("system_disk_persistent_percent", "system_disk_persistent_percent", "test-origin", "Percent", 75.0),
		Entry("system_disk_persistent_inode_percent", "system_disk_persistent_inode_percent", "test-origin", "Percent", 85.0),
		Entry("system_disk_persistent_read_bytes", "system_disk_persistent_read_bytes", "test-origin", "Bytes", 1000.0),
		Entry("system_disk_persistent_write_bytes", "system_disk_persistent_write_bytes", "test-origin", "Bytes", 2000.0),
		Entry("system_disk_persistent_read_time", "system_disk_persistent_read_time", "test-origin", "ms", 3000.0),
		Entry("system_disk_persistent_write_time", "system_disk_persistent_write_time", "test-origin", "ms", 4000.0),
		Entry("system_disk_persistent_io_time", "system_disk_persistent_io_time", "test-origin", "ms", 5000.0),
		Entry("system_healthy", "system_healthy", "test-origin", "", 1.0),
	)

	DescribeTable("network metrics", func(name, origin, unit, networkName string, value float64) {
		sender.Send(networkInput)

		gauge, exists := registry.gauges[name+origin+unit+networkName]
		Expect(exists).To(BeTrue())

		Expect(gauge.value).To(BeNumerically("==", value))
	},
		Entry("system_network_bytes_sent", "system_network_bytes_sent", "test-origin", "Bytes", "eth0", 1.0),
		Entry("system_network_bytes_received", "system_network_bytes_received", "test-origin", "Bytes", "eth0", 2.0),
		Entry("system_network_packets_sent", "system_network_packets_sent", "test-origin", "Packets", "eth0", 3.0),
		Entry("system_network_packets_received", "system_network_packets_received", "test-origin", "Packets", "eth0", 4.0),
		Entry("system_network_error_in", "system_network_error_in", "test-origin", "Frames", "eth0", 5.0),
		Entry("system_network_error_out", "system_network_error_out", "test-origin", "Frames", "eth0", 6.0),
		Entry("system_network_drop_in", "system_network_drop_in", "test-origin", "Packets", "eth0", 7.0),
		Entry("system_network_drop_out", "system_network_drop_out", "test-origin", "Packets", "eth0", 8.0),

		Entry("system_network_bytes_sent", "system_network_bytes_sent", "test-origin", "Bytes", "eth1", 10.0),
		Entry("system_network_bytes_received", "system_network_bytes_received", "test-origin", "Bytes", "eth1", 20.0),
		Entry("system_network_packets_sent", "system_network_packets_sent", "test-origin", "Packets", "eth1", 30.0),
		Entry("system_network_packets_received", "system_network_packets_received", "test-origin", "Packets", "eth1", 40.0),
		Entry("system_network_error_in", "system_network_error_in", "test-origin", "Frames", "eth1", 50.0),
		Entry("system_network_error_out", "system_network_error_out", "test-origin", "Frames", "eth1", 60.0),
		Entry("system_network_drop_in", "system_network_drop_in", "test-origin", "Packets", "eth1", 70.0),
		Entry("system_network_drop_out", "system_network_drop_out", "test-origin", "Packets", "eth1", 80.0),
	)

	DescribeTable("does not have disk metrics if disk is not present", func(name, origin, unit string) {
		sender.Send(collector.SystemStat{})

		_, exists := registry.gauges[name+origin+unit]
		Expect(exists).To(BeFalse())
	},
		Entry("system_disk_system_percent", "system_disk_system_percent", "test-origin", "Percent"),
		Entry("system_disk_system_inode_percent", "system_disk_system_inode_percent", "test-origin", "Percent"),
		Entry("system_disk_system_read_bytes", "system_disk_system_read_bytes", "test-origin", "Bytes"),
		Entry("system_disk_system_write_bytes", "system_disk_system_write_bytes", "test-origin", "Bytes"),
		Entry("system_disk_system_read_time", "system_disk_system_read_time", "test-origin", "ms"),
		Entry("system_disk_system_write_time", "system_disk_system_write_time", "test-origin", "ms"),
		Entry("system_disk_system_io_time", "system_disk_system_io_time", "test-origin", "ms"),
		Entry("system_disk_ephemeral_percent", "system_disk_ephemeral_percent", "test-origin", "Percent"),
		Entry("system_disk_ephemeral_inode_percent", "system_disk_ephemeral_inode_percent", "test-origin", "Percent"),
		Entry("system_disk_ephemeral_read_bytes", "system_disk_ephemeral_read_bytes", "test-origin", "Bytes"),
		Entry("system_disk_ephemeral_write_bytes", "system_disk_ephemeral_write_bytes", "test-origin", "Bytes"),
		Entry("system_disk_ephemeral_read_time", "system_disk_ephemeral_read_time", "test-origin", "ms"),
		Entry("system_disk_ephemeral_write_time", "system_disk_ephemeral_write_time", "test-origin", "ms"),
		Entry("system_disk_ephemeral_io_time", "system_disk_ephemeral_io_time", "test-origin", "ms"),
		Entry("system_disk_persistent_percent", "system_disk_persistent_percent", "test-origin", "Percent"),
		Entry("system_disk_persistent_inode_percent", "system_disk_persistent_inode_percent", "test-origin", "Percent"),
		Entry("system_disk_persistent_read_bytes", "system_disk_persistent_read_bytes", "test-origin", "Bytes"),
		Entry("system_disk_persistent_write_bytes", "system_disk_persistent_write_bytes", "test-origin", "Bytes"),
		Entry("system_disk_persistent_read_time", "system_disk_persistent_read_time", "test-origin", "ms"),
		Entry("system_disk_persistent_write_time", "system_disk_persistent_write_time", "test-origin", "ms"),
		Entry("system_disk_persistent_io_time", "system_disk_persistent_io_time", "test-origin", "ms"),
	)

	It("returns 0 for an unhealthy instance", func() {
		sender.Send(unhealthyInstanceInput)

		gauge, exists := registry.gauges["system_healthy"+"test-origin"]
		Expect(exists).To(BeTrue())
		Expect(gauge.value).To(Equal(0.0))
	})

	It("excludes system_healthy if health precence is false", func() {
		sender.Send(collector.SystemStat{})

		_, exists := registry.gauges["system_healthy"+"test-origin"]
		Expect(exists).To(BeFalse())
	})
})

type spyGauge struct {
	value float64
}

func (g *spyGauge) Set(value float64) {
	g.value = value
}

type stubRegistry struct {
	gaugeCount int
	gauges     map[string]*spyGauge
}

func newStubRegistry() *stubRegistry {
	return &stubRegistry{
		gauges: make(map[string]*spyGauge),
	}
}

func (r *stubRegistry) Get(gaugeName, origin, unit string, tags map[string]string) stats.Gauge {
	r.gaugeCount++

	networkName := ""
	if tags != nil {
		networkName = tags["network_interface"]
	}
	key := gaugeName + origin + unit + networkName

	r.gauges[key] = &spyGauge{}

	return r.gauges[key]
}
