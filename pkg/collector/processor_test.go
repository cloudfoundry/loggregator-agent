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
		}

		var env *loggregator_v2.Envelope
		Eventually(stub.outEnvs).Should(Receive(&env))

		Expect(env.Timestamp).ToNot(BeZero())
		Expect(env.Tags["origin"]).To(Equal("system-metrics-agent"))

		Expect(env.GetGauge().Metrics).To(HaveLen(7))
		Expect(proto.Equal(
			env.GetGauge().Metrics["system.mem.kb"],
			&loggregator_v2.GaugeValue{Unit: "KiB", Value: 1025.0},
		)).To(BeTrue())

		Expect(proto.Equal(
			env.GetGauge().Metrics["system.mem.percent"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 10.01},
		)).To(BeTrue())

		Expect(proto.Equal(
			env.GetGauge().Metrics["system.swap.kb"],
			&loggregator_v2.GaugeValue{Unit: "KiB", Value: 2049.0},
		)).To(BeTrue())

		Expect(proto.Equal(
			env.GetGauge().Metrics["system.swap.percent"],
			&loggregator_v2.GaugeValue{Unit: "Percent", Value: 20.01},
		)).To(BeTrue())

		Expect(proto.Equal(
			env.GetGauge().Metrics["system.load.1m"],
			&loggregator_v2.GaugeValue{Unit: "Load", Value: 1.1},
		)).To(BeTrue())

		Expect(proto.Equal(
			env.GetGauge().Metrics["system.load.5m"],
			&loggregator_v2.GaugeValue{Unit: "Load", Value: 5.5},
		)).To(BeTrue())

		Expect(proto.Equal(
			env.GetGauge().Metrics["system.load.15m"],
			&loggregator_v2.GaugeValue{Unit: "Load", Value: 15.15},
		)).To(BeTrue())
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
