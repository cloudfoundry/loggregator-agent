package cups

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
)

// Metrics is the client used to expose gauge and counter metrics.
type Metrics interface {
	NewGauge(string) func(float64)
	NewCounter(name string) func(uint64)
}

// Getter is configured to fetch HTTP responses
type Getter interface {
	Get(nextID int) (resp *http.Response, err error)
}

// BindingFetcher uses a Getter to fetch and decode Bindings
type BindingFetcher struct {
	getter     Getter
	mu         sync.RWMutex
	drainCount int

	refreshCount        func(uint64)
	refreshFailureCount func(uint64)
	requestCount        func(float64)
	maxLatency          func(float64)
}

type response struct {
	Results map[string]struct {
		Drains   []string
		Hostname string
	}
	NextID int `json:"next_id"`
}

// NewBindingFetcher returns a new BindingFetcher
func NewBindingFetcher(g Getter, m Metrics) *BindingFetcher {
	return &BindingFetcher{
		getter:              g,
		refreshCount:        m.NewCounter("BindingRefreshCount"),
		refreshFailureCount: m.NewCounter("BindingRefreshFailureCount"),
		requestCount:        m.NewGauge("RequestCountForLastBindingRefresh"),
		maxLatency:          m.NewGauge("MaxLatencyForLastBindingRefreshMS"),
	}
}

// FetchBindings reaches out to the syslog drain binding provider via the Getter and decodes
// the response. If it does not get a 200, it returns an error.
func (f *BindingFetcher) FetchBindings() ([]syslog.Binding, error) {
	bindings := []syslog.Binding{}
	nextID := 0

	var requestCount int
	var maxLatency int64
	defer func() {
		f.refreshCount(1)
		f.requestCount(float64(requestCount))
		f.maxLatency(toMilliseconds(maxLatency))
	}()

	for {
		requestCount++
		start := time.Now()
		resp, err := f.getter.Get(nextID)
		d := time.Since(start)
		if err != nil {
			f.refreshFailureCount(1)
			return nil, err
		}

		if d.Nanoseconds() > maxLatency {
			maxLatency = d.Nanoseconds()
		}

		if resp.StatusCode != http.StatusOK {
			f.refreshFailureCount(1)
			return nil, fmt.Errorf("received %d status code from syslog drain binding API", resp.StatusCode)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			f.refreshFailureCount(1)
			return nil, err
		}
		defer resp.Body.Close()

		var r response
		if err = json.Unmarshal(body, &r); err != nil {
			f.refreshFailureCount(1)
			return nil, fmt.Errorf("invalid API response body")
		}

		for appID, bindingData := range r.Results {
			hostname := bindingData.Hostname
			for _, drainURL := range bindingData.Drains {
				// TODO: remove prefix when forwarder-agent is no longer
				// feature-flagged
				u, err := url.Parse(drainURL)
				if err != nil {
					continue
				}

				if strings.HasSuffix(u.Scheme, "-v3") {
					u.Scheme = strings.TrimSuffix(u.Scheme, "-v3")

					bindings = append(bindings, syslog.Binding{
						Hostname: hostname,
						Drain:    u.String(),
						AppId:    appID,
					})
				}
			}
		}

		if r.NextID == 0 {
			return bindings, nil
		}
		nextID = r.NextID
	}
}

// toMilliseconds truncates the calculated milliseconds float to microsecond
// precision.
func toMilliseconds(num int64) float64 {
	f := float64(num) / float64(time.Millisecond)
	output := math.Pow(10, float64(3))
	return float64(roundFloat64(f*output)) / output
}

func roundFloat64(num float64) int {
	return int(num + math.Copysign(0.5, num))
}
