package collector

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/mem"
)

type SystemStat struct {
	MemKB      uint64
	MemPercent float64

	SwapKB      uint64
	SwapPercent float64
}

func Collect() (SystemStat, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	m, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return SystemStat{}, err
	}

	s, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		return SystemStat{}, err
	}

	return SystemStat{
		MemKB:      m.Used / 1024,
		MemPercent: m.UsedPercent,

		SwapKB:      s.Used / 1024,
		SwapPercent: s.UsedPercent,
	}, nil
}
