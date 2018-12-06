package collector_test

import (
	"context"
	"errors"
	"log"

	"code.cloudfoundry.org/loggregator-agent/pkg/collector"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Collector", func() {
	var (
		c   collector.Collector
		src *stubRawCollector
	)

	BeforeEach(func() {
		src = &stubRawCollector{}
		c = collector.New(
			log.New(GinkgoWriter, "", log.LstdFlags),
			collector.WithRawCollector(src),
		)
	})

	It("returns the memory metrics", func() {
		stats, err := c.Collect()
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.MemKB).To(Equal(uint64(2)))
		Expect(stats.MemPercent).To(Equal(40.23))
	})

	It("returns the swap metrics", func() {
		stats, err := c.Collect()
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.SwapKB).To(Equal(uint64(4)))
		Expect(stats.SwapPercent).To(Equal(20.23))
	})

	It("returns load metrics", func() {
		stats, err := c.Collect()
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.Load1M).To(Equal(1.1))
		Expect(stats.Load5M).To(Equal(5.5))
		Expect(stats.Load15M).To(Equal(15.15))
	})

	It("returns cpu metrics", func() {
		stats, err := c.Collect()
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.User).To(Equal(10.0))
		Expect(stats.System).To(Equal(20.0))
		Expect(stats.Idle).To(Equal(30.0))
		Expect(stats.Wait).To(Equal(40.0))
	})

	It("panics on initialization when failing to get initial cpu times", func() {
		src.cpuTimesErr = errors.New("an error")

		Expect(func() {
			_ = collector.New(
				log.New(GinkgoWriter, "", log.LstdFlags),
				collector.WithRawCollector(src),
			)
		}).To(Panic())
	})

	It("returns an error when getting memory fails", func() {
		src.virtualMemoryErr = errors.New("an error")

		_, err := c.Collect()
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when getting swap fails", func() {
		src.swapMemoryErr = errors.New("an error")

		_, err := c.Collect()
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when getting cpu load fails", func() {
		src.cpuLoadErr = errors.New("an error")

		_, err := c.Collect()
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when getting cpu times fails", func() {
		src.cpuTimesErr = errors.New("an error")

		_, err := c.Collect()
		Expect(err).To(HaveOccurred())
	})
})

type stubRawCollector struct {
	timesCallCount float64

	virtualMemoryErr error
	swapMemoryErr    error
	cpuLoadErr       error
	cpuTimesErr      error
}

func (s *stubRawCollector) VirtualMemoryWithContext(context.Context) (*mem.VirtualMemoryStat, error) {
	if s.virtualMemoryErr != nil {
		return nil, s.virtualMemoryErr
	}

	return &mem.VirtualMemoryStat{
		Used:        2048,
		UsedPercent: 40.23,
	}, nil
}

func (s *stubRawCollector) SwapMemoryWithContext(context.Context) (*mem.SwapMemoryStat, error) {
	if s.swapMemoryErr != nil {
		return nil, s.swapMemoryErr
	}

	return &mem.SwapMemoryStat{
		Used:        4096,
		UsedPercent: 20.23,
	}, nil
}

func (s *stubRawCollector) AvgWithContext(context.Context) (*load.AvgStat, error) {
	if s.cpuLoadErr != nil {
		return nil, s.cpuLoadErr
	}

	return &load.AvgStat{
		Load1:  1.1,
		Load5:  5.5,
		Load15: 15.15,
	}, nil
}

func (s *stubRawCollector) TimesWithContext(context.Context, bool) ([]cpu.TimesStat, error) {
	if s.cpuTimesErr != nil {
		return nil, s.cpuTimesErr
	}

	s.timesCallCount += 1.0

	return []cpu.TimesStat{
		{
			User:   500.0 * s.timesCallCount,
			System: 1000.0 * s.timesCallCount,
			Idle:   1500.0 * s.timesCallCount,
			Iowait: 2000.0 * s.timesCallCount,

			Nice:      1000.0,
			Irq:       1000.0,
			Softirq:   1000.0,
			Steal:     1000.0,
			Guest:     1000.0,
			GuestNice: 1000.0,
			Stolen:    1000.0,
		},
	}, nil
}
