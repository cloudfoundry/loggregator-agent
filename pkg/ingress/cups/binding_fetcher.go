package cups

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
)

// Getter is configured to fetch HTTP responses
type Getter interface {
	Get(nextID int) (resp *http.Response, err error)
}

// BindingFetcher uses a Getter to fetch and decode Bindings
type BindingFetcher struct {
	getter     Getter
	mu         sync.RWMutex
	drainCount int
}

type response struct {
	Results map[string]struct {
		Drains   []string
		Hostname string
	}
	NextID int `json:"next_id"`
}

// NewBindingFetcher returns a new BindingFetcher
func NewBindingFetcher(g Getter) *BindingFetcher {
	return &BindingFetcher{
		getter: g,
	}
}

// FetchBindings reaches out to the syslog drain binding provider via the Getter and decodes
// the response. If it does not get a 200, it returns an error.
func (f *BindingFetcher) FetchBindings() ([]syslog.Binding, error) {
	bindings := []syslog.Binding{}
	nextID := 0

	for {
		resp, err := f.getter.Get(nextID)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("received %d status code from syslog drain binding API", resp.StatusCode)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var r response
		if err = json.Unmarshal(body, &r); err != nil {
			return nil, fmt.Errorf("invalid API response body")
		}

		for appID, bindingData := range r.Results {
			hostname := bindingData.Hostname
			for _, drainURL := range bindingData.Drains {
				// TODO: remove prefix when forwarder-agent is no longer
				// feature-flagged
				if strings.HasPrefix(drainURL, "v3-") {
					bindings = append(bindings, syslog.Binding{
						Hostname: hostname,
						Drain:    strings.TrimPrefix(drainURL, "v3-"),
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
