package cups

import (
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"code.cloudfoundry.org/loggregator-agent/pkg/binding"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
)

// Metrics is the client used to expose gauge and counter metrics.
type Metrics interface {
	NewGauge(string) func(float64)
	NewCounter(name string) func(uint64)
}

// Getter is configured to fetch HTTP responses
type Getter interface {
	Get() ([]binding.Binding, error)
}

// BindingFetcher uses a Getter to fetch and decode Bindings
type BindingFetcher struct {
	refreshCount  func(uint64)
	maxLatency    func(float64)
	limit         int
	getter        Getter
}

// NewBindingFetcher returns a new BindingFetcher
func NewBindingFetcher(limit int, g Getter, m Metrics) *BindingFetcher {
	return &BindingFetcher{
		limit:         limit,
		getter:        g,
		refreshCount:  m.NewCounter("BindingRefreshCount"),
		maxLatency:    m.NewGauge("LatencyForLastBindingRefreshMS"),
	}
}

// FetchBindings reaches out to the syslog drain binding provider via the Getter and decodes
// the response. If it does not get a 200, it returns an error.
func (f *BindingFetcher) FetchBindings() ([]syslog.Binding, error) {
	var latency int64
	defer func() {
		f.refreshCount(1)
		f.maxLatency(toMilliseconds(latency))
	}()

	start := time.Now()

	bindings, err := f.getter.Get()
	if err != nil {
		return nil, err
	}
	latency = time.Since(start).Nanoseconds()
	syslogBindings := f.toSyslogBindings(bindings, f.limit)

	return syslogBindings, nil
}

func (f *BindingFetcher) DrainLimit() int {
	return f.limit
}

func (f *BindingFetcher) toSyslogBindings(bs []binding.Binding, perAppLimit int) []syslog.Binding {
	var bindings []syslog.Binding
	for _, b := range bs {
		drains := b.Drains
		sort.Strings(drains)

		if perAppLimit < len(drains) {
			drains = drains[:perAppLimit]
		}

		for _, d := range drains {
			// TODO: remove prefix when forwarder-agent is no longer
			// feature-flagged

			u, err := url.Parse(d)
			if err != nil {
				continue
			}

			if strings.HasSuffix(u.Scheme, "-v3") {
				u.Scheme = strings.TrimSuffix(u.Scheme, "-v3")

				binding := syslog.Binding{
					AppId:    b.AppID,
					Hostname: b.Hostname,
					Drain:    u.String(),
				}
				bindings = append(bindings, binding)
			}
		}
	}

	return bindings
}

// toMilliseconds truncates the calculated milliseconds float to microsecond
// precision.
func toMilliseconds(num int64) float64 {
	millis := float64(num) / float64(time.Millisecond)
	microsPerMilli := 1000.0
	return roundFloat64(millis*microsPerMilli) / microsPerMilli
}

func roundFloat64(num float64) float64 {
	return float64(int(num + math.Copysign(0.5, num)))
}
