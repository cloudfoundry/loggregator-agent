package collector

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

const (
	systemDiskPath     = "/"
	ephemeralDiskPath  = "/var/vcap/data"
	persistentDiskPath = "/var/vcap/store"
)

type SystemStat struct {
	CPUStat

	MemKB      uint64
	MemPercent float64

	SwapKB      uint64
	SwapPercent float64

	Load1M  float64
	Load5M  float64
	Load15M float64

	SystemDisk     DiskStat
	EphemeralDisk  DiskStat
	PersistentDisk DiskStat

	Networks []NetworkStat
}

type CPUStat struct {
	User   float64
	System float64
	Wait   float64
	Idle   float64
}

type DiskStat struct {
	Percent      float64
	InodePercent float64
	Present      bool
}

type Collector struct {
	rawCollector  RawCollector
	prevTimesStat cpu.TimesStat
}

type NetworkStat struct {
	Name            string
	BytesSent       uint64
	BytesReceived   uint64
	PacketsSent     uint64
	PacketsReceived uint64
	ErrIn           uint64
	ErrOut          uint64
	DropIn          uint64
	DropOut         uint64
}

func New(log *log.Logger, opts ...CollectorOption) Collector {
	c := Collector{
		rawCollector: defaultRawCollector{},
	}

	for _, o := range opts {
		o(&c)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	firstTS, err := c.rawCollector.TimesWithContext(ctx, false)
	if err != nil {
		log.Panicf("failed to collect initial CPU times: %s", err)
	}
	c.prevTimesStat = firstTS[0]

	return c
}

func (c Collector) Collect() (SystemStat, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	m, err := c.rawCollector.VirtualMemoryWithContext(ctx)
	if err != nil {
		return SystemStat{}, err
	}

	s, err := c.rawCollector.SwapMemoryWithContext(ctx)
	if err != nil {
		return SystemStat{}, err
	}

	l, err := c.rawCollector.AvgWithContext(ctx)
	if err != nil {
		return SystemStat{}, err
	}

	ts, err := c.rawCollector.TimesWithContext(ctx, false)
	if err != nil {
		return SystemStat{}, err
	}

	cpu := calculateCPUStat(c.prevTimesStat, ts[0])
	c.prevTimesStat = ts[0]

	sdisk, err := c.diskStat(ctx, systemDiskPath)
	if err != nil {
		return SystemStat{}, err
	}

	edisk, err := c.diskStat(ctx, ephemeralDiskPath)
	if err != nil {
		return SystemStat{}, err
	}

	pdisk, err := c.diskStat(ctx, persistentDiskPath)
	if err != nil {
		return SystemStat{}, err
	}

	networks, err := c.networkStat(ctx)
	if err != nil {
		return SystemStat{}, err
	}

	return SystemStat{
		CPUStat: cpu,

		MemKB:      m.Used / 1024,
		MemPercent: m.UsedPercent,

		SwapKB:      s.Used / 1024,
		SwapPercent: s.UsedPercent,

		Load1M:  l.Load1,
		Load5M:  l.Load5,
		Load15M: l.Load15,

		SystemDisk:     sdisk,
		EphemeralDisk:  edisk,
		PersistentDisk: pdisk,

		Networks: networks,
	}, nil
}

func (c Collector) diskStat(ctx context.Context, path string) (DiskStat, error) {
	disk, err := c.rawCollector.UsageWithContext(ctx, path)
	if err != nil && os.IsNotExist(err) {
		return DiskStat{}, nil
	}

	if err != nil {
		return DiskStat{}, err
	}

	return DiskStat{
		Percent:      disk.UsedPercent,
		InodePercent: disk.InodesUsedPercent,
		Present:      true,
	}, nil
}

func (c Collector) networkStat(ctx context.Context) ([]NetworkStat, error) {
	counters, err := c.rawCollector.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, err
	}

	var ns []NetworkStat
	for _, c := range counters {
		if strings.HasPrefix(c.Name, "eth") {
			ns = append(ns, NetworkStat{
				Name:            c.Name,
				BytesSent:       c.BytesSent,
				BytesReceived:   c.BytesRecv,
				PacketsSent:     c.PacketsSent,
				PacketsReceived: c.PacketsRecv,
				ErrIn:           c.Errin,
				ErrOut:          c.Errout,
				DropIn:          c.Dropin,
				DropOut:         c.Dropout,
			})
		}
	}

	return ns, nil
}

func calculateCPUStat(previous, current cpu.TimesStat) CPUStat {
	totalDiff := current.Total() - previous.Total()

	return CPUStat{
		User:   (current.User - previous.User) / totalDiff * 100.0,
		System: (current.System - previous.System) / totalDiff * 100.0,
		Idle:   (current.Idle - previous.Idle) / totalDiff * 100.0,
		Wait:   (current.Iowait - previous.Iowait) / totalDiff * 100.0,
	}
}

type RawCollector interface {
	VirtualMemoryWithContext(context.Context) (*mem.VirtualMemoryStat, error)
	SwapMemoryWithContext(context.Context) (*mem.SwapMemoryStat, error)
	AvgWithContext(context.Context) (*load.AvgStat, error)
	TimesWithContext(context.Context, bool) ([]cpu.TimesStat, error)
	UsageWithContext(context.Context, string) (*disk.UsageStat, error)
	IOCountersWithContext(context.Context, bool) ([]net.IOCountersStat, error)
}

type CollectorOption func(*Collector)

func WithRawCollector(c RawCollector) CollectorOption {
	return func(cs *Collector) {
		cs.rawCollector = c
	}
}

type defaultRawCollector struct{}

func (s defaultRawCollector) VirtualMemoryWithContext(ctx context.Context) (*mem.VirtualMemoryStat, error) {
	return mem.VirtualMemoryWithContext(ctx)
}

func (s defaultRawCollector) SwapMemoryWithContext(ctx context.Context) (*mem.SwapMemoryStat, error) {
	return mem.SwapMemoryWithContext(ctx)
}

func (s defaultRawCollector) AvgWithContext(ctx context.Context) (*load.AvgStat, error) {
	return load.AvgWithContext(ctx)
}

func (s defaultRawCollector) TimesWithContext(ctx context.Context, perCPU bool) ([]cpu.TimesStat, error) {
	return cpu.TimesWithContext(ctx, perCPU)
}

func (s defaultRawCollector) UsageWithContext(ctx context.Context, path string) (*disk.UsageStat, error) {
	return disk.UsageWithContext(ctx, path)
}

func (s defaultRawCollector) IOCountersWithContext(ctx context.Context, pernic bool) ([]net.IOCountersStat, error) {
	return net.IOCountersWithContext(ctx, pernic)
}
