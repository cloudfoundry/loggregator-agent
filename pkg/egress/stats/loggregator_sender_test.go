package stats_test

import (
	"sync/atomic"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/collector"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/stats"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Loggregator Sender", func() {
	var (
		stub   *stubInputOutput
		sender *stats.LoggregatorSender
	)

	BeforeEach(func() {
		stub = newStubInputOutput()
		sender = stats.NewLoggregatorSender(stub.output, "system-metrics-agent")
	})

	It("receives stats and sends the correct number of metrics", func() {
		sender.Send(defaultInput)

		var env *loggregator_v2.Envelope
		Eventually(stub.outEnvs).Should(Receive(&env))

		Expect(env.Timestamp).ToNot(BeZero())
		Expect(env.Tags["origin"]).To(Equal("system-metrics-agent"))

		metrics := env.GetGauge().Metrics
		Expect(metrics).To(HaveLen(40))
	})

	DescribeTable("default metrics", func(name, unit string, value float64) {
		sender.Send(defaultInput)

		var env *loggregator_v2.Envelope
		Eventually(stub.outEnvs).Should(Receive(&env))

		metrics := env.GetGauge().Metrics

		Expect(proto.Equal(
			metrics[name],
			&loggregator_v2.GaugeValue{Unit: unit, Value: value},
		)).To(BeTrue())
	},
		Entry("system_mem_kb", "system_mem_kb", "KiB", 1025.0),
		Entry("system_mem_percent", "system_mem_percent", "Percent", 10.01),
		Entry("system_swap_kb", "system_swap_kb", "KiB", 2049.0),
		Entry("system_swap_percent", "system_swap_percent", "Percent", 20.01),
		Entry("system_load_1m", "system_load_1m", "Load", 1.1),
		Entry("system_load_5m", "system_load_5m", "Load", 5.5),
		Entry("system_load_15m", "system_load_15m", "Load", 15.15),
		Entry("system_cpu_user", "system_cpu_user", "Percent", 25.25),
		Entry("system_cpu_sys", "system_cpu_sys", "Percent", 52.52),
		Entry("system_cpu_idle", "system_cpu_idle", "Percent", 10.10),
		Entry("system_cpu_wait", "system_cpu_wait", "Percent", 22.22),
		Entry("system_disk_system_percent", "system_disk_system_percent", "Percent", 35.0),
		Entry("system_disk_system_inode_percent", "system_disk_system_inode_percent", "Percent", 45.0),
		Entry("system_disk_system_read_bytes", "system_disk_system_read_bytes", "Bytes", 10.0),
		Entry("system_disk_system_write_bytes", "system_disk_system_write_bytes", "Bytes", 20.0),
		Entry("system_disk_system_read_time", "system_disk_system_read_time", "ms", 30.0),
		Entry("system_disk_system_write_time", "system_disk_system_write_time", "ms", 40.0),
		Entry("system_disk_system_io_time", "system_disk_system_io_time", "ms", 50.0),
		Entry("system_disk_ephemeral_percent", "system_disk_ephemeral_percent", "Percent", 55.0),
		Entry("system_disk_ephemeral_inode_percent", "system_disk_ephemeral_inode_percent", "Percent", 65.0),
		Entry("system_disk_ephemeral_read_bytes", "system_disk_ephemeral_read_bytes", "Bytes", 100.0),
		Entry("system_disk_ephemeral_write_bytes", "system_disk_ephemeral_write_bytes", "Bytes", 200.0),
		Entry("system_disk_ephemeral_read_time", "system_disk_ephemeral_read_time", "ms", 300.0),
		Entry("system_disk_ephemeral_write_time", "system_disk_ephemeral_write_time", "ms", 400.0),
		Entry("system_disk_ephemeral_io_time", "system_disk_ephemeral_io_time", "ms", 500.0),
		Entry("system_disk_persistent_percent", "system_disk_persistent_percent", "Percent", 75.0),
		Entry("system_disk_persistent_inode_percent", "system_disk_persistent_inode_percent", "Percent", 85.0),
		Entry("system_disk_persistent_read_bytes", "system_disk_persistent_read_bytes", "Bytes", 1000.0),
		Entry("system_disk_persistent_write_bytes", "system_disk_persistent_write_bytes", "Bytes", 2000.0),
		Entry("system_disk_persistent_read_time", "system_disk_persistent_read_time", "ms", 3000.0),
		Entry("system_disk_persistent_write_time", "system_disk_persistent_write_time", "ms", 4000.0),
		Entry("system_disk_persistent_io_time", "system_disk_persistent_io_time", "ms", 5000.0),
		Entry("system_healthy", "system_healthy", "", 1.0),
		Entry("system_network_ip_forwarding", "system_network_ip_forwarding", "", 1.0),
		Entry("system_network_udp_no_ports", "system_network_udp_no_ports", "", 2.0),
		Entry("system_network_udp_in_errors", "system_network_udp_in_errors", "", 3.0),
		Entry("system_network_udp_lite_in_errors", "system_network_udp_lite_in_errors", "", 4.0),
		Entry("system_network_tcp_active_opens", "system_network_tcp_active_opens", "", 5.0),
		Entry("system_network_tcp_curr_estab", "system_network_tcp_curr_estab", "", 6.0),
		Entry("system_network_tcp_retrans_segs", "system_network_tcp_retrans_segs", "", 7.0),
	)

	It("receives stats and sends an envelope on an interval", func() {
		sender.Send(networkInput)

		var env *loggregator_v2.Envelope
		Eventually(stub.outEnvs).Should(Receive(&env))
		Expect(env.GetGauge().Metrics).To(HaveKey("system_mem_kb"))

		Eventually(stub.outEnvs).Should(Receive(&env))
		Expect(env.GetTimestamp()).To(BeNumerically(">", 0))
		Expect(env.GetTags()).To(HaveKeyWithValue("origin", "system-metrics-agent"))

		interfaceName := env.GetTags()["network_interface"]
		Expect(interfaceName).To(Or(Equal("eth0"), Equal("eth1")))

		Eventually(stub.outEnvs).Should(Receive(&env))
		Expect(env.GetTimestamp()).To(BeNumerically(">", 0))
		Expect(env.GetTags()).To(HaveKeyWithValue("origin", "system-metrics-agent"))

		interfaceName = env.GetTags()["network_interface"]
		Expect(interfaceName).To(Or(Equal("eth0"), Equal("eth1")))

		metrics := env.GetGauge().Metrics
		Expect(metrics).To(HaveLen(8))
	})

	DescribeTable("network metrics", func(name, unit string, value float64) {
		sender.Send(singleNetworkInput)

		var env *loggregator_v2.Envelope
		Eventually(stub.outEnvs).Should(Receive(&env))
		Eventually(stub.outEnvs).Should(Receive(&env)) //network envelopes always come after system_stat envelopes

		metrics := env.GetGauge().Metrics
		Expect(proto.Equal(
			metrics[name],
			&loggregator_v2.GaugeValue{Unit: unit, Value: value},
		)).To(BeTrue())
	},
		Entry("system_network_bytes_sent", "system_network_bytes_sent", "Bytes", 1.0),
		Entry("system_network_bytes_received", "system_network_bytes_received", "Bytes", 2.0),
		Entry("system_network_packets_sent", "system_network_packets_sent", "Packets", 3.0),
		Entry("system_network_packets_received", "system_network_packets_received", "Packets", 4.0),
		Entry("system_network_error_in", "system_network_error_in", "Frames", 5.0),
		Entry("system_network_error_out", "system_network_error_out", "Frames", 6.0),
		Entry("system_network_drop_in", "system_network_drop_in", "Packets", 7.0),
		Entry("system_network_drop_out", "system_network_drop_out", "Packets", 8.0),
	)

	It("does not have disk metrics if disk is not present", func() {
		sender.Send(collector.SystemStat{})

		var env *loggregator_v2.Envelope
		Eventually(stub.outEnvs).Should(Receive(&env))

		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_disk_system_percent"))
		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_disk_system_inode_percent"))
		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_disk_ephemeral_percent"))
		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_disk_ephemeral_inode_percent"))
		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_disk_persistent_percent"))
		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_disk_persistent_inode_percent"))
	})

	It("returns 0 for an unhealthy instance", func() {
		sender.Send(unhealthyInstanceInput)

		var env *loggregator_v2.Envelope
		Eventually(stub.outEnvs).Should(Receive(&env))
		Expect(env.GetGauge().Metrics["system_healthy"].Value).To(Equal(0.0))
	})

	It("excludes system_healthy if health precence is false", func() {
		sender.Send(collector.SystemStat{})

		var env *loggregator_v2.Envelope
		Eventually(stub.outEnvs).Should(Receive(&env))

		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_healthy"))
	})
})

var (
	defaultInput = collector.SystemStat{
		MemKB:      1025,
		MemPercent: 10.01,

		SwapKB:      2049,
		SwapPercent: 20.01,

		Load1M:  1.1,
		Load5M:  5.5,
		Load15M: 15.15,

		CPUStat: collector.CPUStat{
			User:   25.25,
			System: 52.52,
			Idle:   10.10,
			Wait:   22.22,
		},

		SystemDisk: collector.DiskStat{
			Present: true,

			Percent:      35.0,
			InodePercent: 45.0,

			ReadBytes:  10,
			WriteBytes: 20,
			ReadTime:   30,
			WriteTime:  40,
			IOTime:     50,
		},

		EphemeralDisk: collector.DiskStat{
			Present: true,

			Percent:      55.0,
			InodePercent: 65.0,

			ReadBytes:  100,
			WriteBytes: 200,
			ReadTime:   300,
			WriteTime:  400,
			IOTime:     500,
		},

		PersistentDisk: collector.DiskStat{
			Present: true,

			Percent:      75.0,
			InodePercent: 85.0,

			ReadBytes:  1000,
			WriteBytes: 2000,
			ReadTime:   3000,
			WriteTime:  4000,
			IOTime:     5000,
		},

		ProtoCounters: collector.ProtoCountersStat{
			IPForwarding:    1,
			UDPNoPorts:      2,
			UDPInErrors:     3,
			UDPLiteInErrors: 4,
			TCPActiveOpens:  5,
			TCPCurrEstab:    6,
			TCPRetransSegs:  7,
		},

		Health: collector.HealthStat{
			Present: true,
			Healthy: true,
		},
	}

	networkInput = collector.SystemStat{
		Networks: []collector.NetworkStat{
			{
				Name:            "eth0",
				BytesSent:       1,
				BytesReceived:   2,
				PacketsSent:     3,
				PacketsReceived: 4,
				ErrIn:           5,
				ErrOut:          6,
				DropIn:          7,
				DropOut:         8,
			},
			{
				Name:            "eth1",
				BytesSent:       10,
				BytesReceived:   20,
				PacketsSent:     30,
				PacketsReceived: 40,
				ErrIn:           50,
				ErrOut:          60,
				DropIn:          70,
				DropOut:         80,
			},
		},
	}

	singleNetworkInput = collector.SystemStat{
		Networks: []collector.NetworkStat{
			{
				Name:            "eth0",
				BytesSent:       1,
				BytesReceived:   2,
				PacketsSent:     3,
				PacketsReceived: 4,
				ErrIn:           5,
				ErrOut:          6,
				DropIn:          7,
				DropOut:         8,
			},
		},
	}

	unhealthyInstanceInput = collector.SystemStat{
		Health: collector.HealthStat{
			Present: true,
			Healthy: false,
		},
	}
)

type stubInputOutput struct {
	inStats chan collector.SystemStat
	outEnvs chan *loggregator_v2.Envelope

	inCount  int64
	outCount int64
}

func newStubInputOutput() *stubInputOutput {
	return &stubInputOutput{
		inStats: make(chan collector.SystemStat, 100),
		outEnvs: make(chan *loggregator_v2.Envelope, 100),
	}
}

func (s *stubInputOutput) input() (collector.SystemStat, error) {
	atomic.AddInt64(&s.inCount, 1)

	return <-s.inStats, nil
}

func (s *stubInputOutput) output(env *loggregator_v2.Envelope) {
	atomic.AddInt64(&s.outCount, 1)

	s.outEnvs <- env
}
