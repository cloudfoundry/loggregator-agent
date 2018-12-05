package collector

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
)

type SystemStat struct {
	MemKB      uint64
	MemPercent float64

	SwapKB      uint64
	SwapPercent float64

	Load1M  float64
	Load5M  float64
	Load15M float64
}

func Collect(opts ...CollectOption) (SystemStat, error) {
	cfg := CollectConfig{
		rawCollector: defaultRawCollector{},
	}

	for _, o := range opts {
		o(&cfg)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	m, err := cfg.rawCollector.VirtualMemoryWithContext(ctx)
	if err != nil {
		return SystemStat{}, err
	}

	s, err := cfg.rawCollector.SwapMemoryWithContext(ctx)
	if err != nil {
		return SystemStat{}, err
	}

	l, err := cfg.rawCollector.AvgWithContext(ctx)
	if err != nil {
		return SystemStat{}, err
	}

	return SystemStat{
		MemKB:      m.Used / 1024,
		MemPercent: m.UsedPercent,

		SwapKB:      s.Used / 1024,
		SwapPercent: s.UsedPercent,

		Load1M:  l.Load1,
		Load5M:  l.Load5,
		Load15M: l.Load15,
	}, nil
}

type RawCollector interface {
	VirtualMemoryWithContext(context.Context) (*mem.VirtualMemoryStat, error)
	SwapMemoryWithContext(context.Context) (*mem.SwapMemoryStat, error)
	AvgWithContext(context.Context) (*load.AvgStat, error)
}

type CollectConfig struct {
	rawCollector RawCollector
}

type CollectOption func(*CollectConfig)

func WithRawCollector(c RawCollector) CollectOption {
	return func(cs *CollectConfig) {
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
