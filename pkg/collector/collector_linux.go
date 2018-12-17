// +build !windows

package collector

import (
	"context"

	"github.com/shirou/gopsutil/load"
)

func (s defaultRawCollector) AvgWithContext(ctx context.Context) (*load.AvgStat, error) {
	return load.AvgWithContext(ctx)
}
