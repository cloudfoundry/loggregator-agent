package collector_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"

	"code.cloudfoundry.org/loggregator-agent/pkg/collector"
)

var _ = Describe("Collect", func() {
	It("returns the memory metrics", func() {
		stats, err := collector.Collect(
			collector.WithRawCollector(stubRawCollector{}),
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.MemKB).To(Equal(uint64(2)))
		Expect(stats.MemPercent).To(Equal(40.23))
	})

	It("returns the swap metrics", func() {
		stats, err := collector.Collect(
			collector.WithRawCollector(stubRawCollector{}),
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.SwapKB).To(Equal(uint64(4)))
		Expect(stats.SwapPercent).To(Equal(20.23))
	})

	It("returns load metrics", func() {
		stats, err := collector.Collect(
			collector.WithRawCollector(&stubRawCollector{}),
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.Load1M).To(Equal(1.1))
		Expect(stats.Load5M).To(Equal(5.5))
		Expect(stats.Load15M).To(Equal(15.15))
	})
})

type stubRawCollector struct{}

func (s stubRawCollector) VirtualMemoryWithContext(context.Context) (*mem.VirtualMemoryStat, error) {
	return &mem.VirtualMemoryStat{
		Used:        2048,
		UsedPercent: 40.23,
	}, nil
}

func (s stubRawCollector) SwapMemoryWithContext(context.Context) (*mem.SwapMemoryStat, error) {
	return &mem.SwapMemoryStat{
		Used:        4096,
		UsedPercent: 20.23,
	}, nil
}

func (s stubRawCollector) AvgWithContext(context.Context) (*load.AvgStat, error) {
	return &load.AvgStat{
		Load1:  1.1,
		Load5:  5.5,
		Load15: 15.15,
	}, nil
}
