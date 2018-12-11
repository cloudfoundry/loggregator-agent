package collector_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"syscall"

	"code.cloudfoundry.org/loggregator-agent/pkg/collector"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"

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

	It("returns network metrics", func() {
		stats, err := c.Collect()
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.Networks).To(HaveLen(1))

		stat := stats.Networks[0]
		Expect(stat.Name).To(Equal("eth0"))
		Expect(stat.BytesSent).To(Equal(uint64(10)))
		Expect(stat.BytesReceived).To(Equal(uint64(20)))
		Expect(stat.PacketsSent).To(Equal(uint64(30)))
		Expect(stat.PacketsReceived).To(Equal(uint64(40)))
		Expect(stat.ErrIn).To(Equal(uint64(50)))
		Expect(stat.ErrOut).To(Equal(uint64(60)))
		Expect(stat.DropIn).To(Equal(uint64(70)))
		Expect(stat.DropOut).To(Equal(uint64(80)))
	})

	It("returns disk metrics", func() {
		stats, err := c.Collect()
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.SystemDisk.Percent).To(Equal(65.0))
		Expect(stats.SystemDisk.InodePercent).To(Equal(75.0))
		Expect(stats.SystemDisk.Present).To(BeTrue())
		Expect(stats.EphemeralDisk.Percent).To(Equal(85.0))
		Expect(stats.EphemeralDisk.InodePercent).To(Equal(95.0))
		Expect(stats.EphemeralDisk.Present).To(BeTrue())
		Expect(stats.PersistentDisk.Percent).To(Equal(105.0))
		Expect(stats.PersistentDisk.InodePercent).To(Equal(115.0))
		Expect(stats.PersistentDisk.Present).To(BeTrue())
	})

	It("shows the system disk is not present if directory does not exist", func() {
		src.systemDiskUsageError = syscall.ENOENT

		stats, err := c.Collect()
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.SystemDisk.Percent).To(Equal(0.0))
		Expect(stats.SystemDisk.InodePercent).To(Equal(0.0))
		Expect(stats.SystemDisk.Present).To(BeFalse())
	})

	It("shows the ephemeral disk is not present if directory does not exist", func() {
		src.ephemeralDiskUsageError = syscall.ENOENT

		stats, err := c.Collect()
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.EphemeralDisk.Percent).To(Equal(0.0))
		Expect(stats.EphemeralDisk.InodePercent).To(Equal(0.0))
		Expect(stats.EphemeralDisk.Present).To(BeFalse())
	})

	It("shows the persistent disk is not present if directory does not exist", func() {
		src.persistentDiskUsageError = syscall.ENOENT

		stats, err := c.Collect()
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.PersistentDisk.Percent).To(Equal(0.0))
		Expect(stats.PersistentDisk.InodePercent).To(Equal(0.0))
		Expect(stats.PersistentDisk.Present).To(BeFalse())
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

	It("returns an error when getting networks fails", func() {
		src.ioCountersErr = errors.New("an error")

		_, err := c.Collect()
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when getting system disk usage fails", func() {
		src.systemDiskUsageError = errors.New("an error")

		_, err := c.Collect()
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when getting ephemeral disk usage fails", func() {
		src.ephemeralDiskUsageError = errors.New("an error")

		_, err := c.Collect()
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when getting persistent disk usage fails", func() {
		src.persistentDiskUsageError = errors.New("an error")

		_, err := c.Collect()
		Expect(err).To(HaveOccurred())
	})
})

type stubRawCollector struct {
	timesCallCount float64

	virtualMemoryErr         error
	swapMemoryErr            error
	cpuLoadErr               error
	cpuTimesErr              error
	ioCountersErr            error
	systemDiskUsageError     error
	ephemeralDiskUsageError  error
	persistentDiskUsageError error
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

func (s *stubRawCollector) IOCountersWithContext(context.Context, bool) ([]net.IOCountersStat, error) {
	if s.ioCountersErr != nil {
		return nil, s.ioCountersErr
	}

	return []net.IOCountersStat{
		{
			Name:        "blah",
			BytesSent:   1,
			BytesRecv:   2,
			PacketsSent: 3,
			PacketsRecv: 4,
			Errin:       5,
			Errout:      6,
			Dropin:      7,
			Dropout:     8,
		},
		{
			Name:        "eth0",
			BytesSent:   10,
			BytesRecv:   20,
			PacketsSent: 30,
			PacketsRecv: 40,
			Errin:       50,
			Errout:      60,
			Dropin:      70,
			Dropout:     80,
		},
	}, nil
}

func (s *stubRawCollector) UsageWithContext(_ context.Context, path string) (*disk.UsageStat, error) {
	switch path {
	case "/":
		if s.systemDiskUsageError != nil {
			return nil, s.systemDiskUsageError
		}

		return &disk.UsageStat{
			UsedPercent:       65.0,
			InodesUsedPercent: 75.0,
		}, nil
	case "/var/vcap/data":
		if s.ephemeralDiskUsageError != nil {
			return nil, s.ephemeralDiskUsageError
		}

		return &disk.UsageStat{
			UsedPercent:       85.0,
			InodesUsedPercent: 95.0,
		}, nil
	case "/var/vcap/store":
		if s.persistentDiskUsageError != nil {
			return nil, s.persistentDiskUsageError
		}

		return &disk.UsageStat{
			UsedPercent:       105.0,
			InodesUsedPercent: 115.0,
		}, nil
	}

	panic(fmt.Sprintf("requested usage for forbidden path: %s", path))
}
