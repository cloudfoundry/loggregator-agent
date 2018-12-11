package collector_test

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/collector"
)

var _ = Describe("Processor", func() {
	It("receives stats and sends an envelope on an interval", func() {
		stub := newStubInputOutput()
		processor := collector.NewProcessor(
			stub.input,
			stub.output,
			10*time.Millisecond,
			log.New(GinkgoWriter, "", log.LstdFlags),
		)

		go processor.Run()

		stub.inStats <- collector.SystemStat{
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
				Percent:      35.0,
				InodePercent: 45.0,
				Present:      true,
			},

			EphemeralDisk: collector.DiskStat{
				Percent:      55.0,
				InodePercent: 65.0,
				Present:      true,
			},

			PersistentDisk: collector.DiskStat{
				Percent:      75.0,
				InodePercent: 85.0,
				Present:      true,
			},
		}

		var env *loggregator_v2.Envelope
		Eventually(stub.outEnvs).Should(Receive(&env))

		Expect(env.Timestamp).ToNot(BeZero())
		Expect(env.Tags["origin"]).To(Equal("system-metrics-agent"))

		metrics := env.GetGauge().Metrics
		Expect(metrics).To(HaveLen(17))

		Expect(proto.Equal(
			metrics["system_mem_kb"],
			&loggregator_v2.GaugeValue{Unit: "KiB", Value: 1025.0},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_mem_percent"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 10.01},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_swap_kb"],
			&loggregator_v2.GaugeValue{Unit: "KiB", Value: 2049.0},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_swap_percent"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 20.01},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_load_1m"],
			&loggregator_v2.GaugeValue{Unit: "Load", Value: 1.1},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_load_5m"],
			&loggregator_v2.GaugeValue{Unit: "Load", Value: 5.5},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_load_15m"],
			&loggregator_v2.GaugeValue{Unit: "Load", Value: 15.15},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_cpu_user"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 25.25},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_cpu_sys"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 52.52},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_cpu_idle"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 10.10},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_cpu_wait"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 22.22},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_disk_system_percent"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 35.0},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_disk_system_inode_percent"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 45.0},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_disk_ephemeral_percent"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 55.0},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_disk_ephemeral_inode_percent"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 65.0},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_disk_persistent_percent"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 75.0},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_disk_persistent_inode_percent"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 85.0},
		)).To(BeTrue())
	})

	It("receives stats and sends an envelope on an interval", func() {
		stub := newStubInputOutput()
		processor := collector.NewProcessor(
			stub.input,
			stub.output,
			10*time.Millisecond,
			log.New(GinkgoWriter, "", log.LstdFlags),
		)

		go processor.Run()

		stub.inStats <- collector.SystemStat{
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

		var env *loggregator_v2.Envelope
		Eventually(stub.outEnvs).Should(Receive(&env))
		Expect(env.GetGauge().Metrics).To(HaveKey("system_mem_kb"))

		Eventually(stub.outEnvs).Should(Receive(&env))
		Expect(env.GetTimestamp()).To(BeNumerically(">", 0))
		Expect(env.GetTags()).To(HaveKeyWithValue("origin", "system-metrics-agent"))
		Expect(env.GetTags()).To(HaveKeyWithValue("network_interface", "eth0"))

		Eventually(stub.outEnvs).Should(Receive(&env))
		Expect(env.GetTimestamp()).To(BeNumerically(">", 0))
		Expect(env.GetTags()).To(HaveKeyWithValue("origin", "system-metrics-agent"))
		Expect(env.GetTags()).To(HaveKeyWithValue("network_interface", "eth1"))

		metrics := env.GetGauge().Metrics
		Expect(metrics).To(HaveLen(8))

		Expect(proto.Equal(
			metrics["system_network_bytes_sent"],
			&loggregator_v2.GaugeValue{Unit: "Bytes", Value: 10},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_network_bytes_received"],
			&loggregator_v2.GaugeValue{Unit: "Bytes", Value: 20},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_network_packets_sent"],
			&loggregator_v2.GaugeValue{Unit: "Packets", Value: 30},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_network_packets_received"],
			&loggregator_v2.GaugeValue{Unit: "Packets", Value: 40},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_network_error_in"],
			&loggregator_v2.GaugeValue{Unit: "Frames", Value: 50},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_network_error_out"],
			&loggregator_v2.GaugeValue{Unit: "Frames", Value: 60},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_network_drop_in"],
			&loggregator_v2.GaugeValue{Unit: "Packets", Value: 70},
		)).To(BeTrue())

		Expect(proto.Equal(
			metrics["system_network_drop_out"],
			&loggregator_v2.GaugeValue{Unit: "Packets", Value: 80},
		)).To(BeTrue())
	})

	It("does not have disk metrics if disk is not present", func() {
		stub := newStubInputOutput()
		processor := collector.NewProcessor(
			stub.input,
			stub.output,
			10*time.Millisecond,
			log.New(GinkgoWriter, "", log.LstdFlags),
		)

		go processor.Run()

		stub.inStats <- collector.SystemStat{}

		var env *loggregator_v2.Envelope
		Eventually(stub.outEnvs).Should(Receive(&env))

		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_disk_system_percent"))
		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_disk_system_inode_percent"))
		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_disk_ephemeral_percent"))
		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_disk_ephemeral_inode_percent"))
		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_disk_persistent_percent"))
		Expect(env.GetGauge().Metrics).ToNot(HaveKey("system_disk_persistent_inode_percent"))
	})
})

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
