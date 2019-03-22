package cups

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"code.cloudfoundry.org/loggregator-agent/pkg/binding"
	"fmt"
	"log"
	"net"

	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
)

var allowedSchemes = []string{"syslog", "syslog-tls", "https"}

type IPChecker interface {
	ParseHost(url string) (string, string, error)
	ResolveAddr(host string) (net.IP, error)
	CheckBlacklist(ip net.IP) error
}

type MetricClient interface {
	NewGauge(name string, opts ...metrics.MetricOption) (metrics.Gauge, error)
}

type FilteredBindingFetcher struct {
	ipChecker         IPChecker
	br                binding.Fetcher
	logger            *log.Logger
	invalidDrains     metrics.Gauge
	blacklistedDrains metrics.Gauge
}

func NewFilteredBindingFetcher(c IPChecker, b binding.Fetcher, m MetricClient, lc *log.Logger) *FilteredBindingFetcher {
	// TODO: err checking
	invalidDrain, _ := m.NewGauge("invalid_drains", metrics.WithMetricTags(map[string]string{"unit": "total"}))
	blacklistedDrains, _ := m.NewGauge("blacklisted_drains", metrics.WithMetricTags(map[string]string{"unit": "total"}))

	return &FilteredBindingFetcher{
		ipChecker:         c,
		br:                b,
		logger:            lc,
		invalidDrains:     invalidDrain,
		blacklistedDrains: blacklistedDrains,
	}
}

func (f FilteredBindingFetcher) DrainLimit() int {
	return f.br.DrainLimit()
}

func (f *FilteredBindingFetcher) FetchBindings() ([]syslog.Binding, error) {
	sourceBindings, err := f.br.FetchBindings()
	if err != nil {
		return nil, err
	}
	newBindings := []syslog.Binding{}

	var invalidDrains float64
	var blacklistedDrains float64
	for _, b := range sourceBindings {
		scheme, host, err := f.ipChecker.ParseHost(b.Drain)
		if err != nil {
			f.logger.Printf("failed to parse host for drain URL: %s", err)
			invalidDrains += 1
			continue
		}

		if invalidScheme(scheme) {
			invalidDrains += 1
			continue
		}

		ip, err := f.ipChecker.ResolveAddr(host)
		if err != nil {
			msg := fmt.Sprintf("failed to resolve syslog drain host: %s", host)
			f.logger.Println(msg, err)
			invalidDrains += 1
			continue
		}

		err = f.ipChecker.CheckBlacklist(ip)
		if err != nil {
			msg := fmt.Sprintf("syslog drain blacklisted: %s (%s)", host, ip)
			f.logger.Println(msg, err)
			invalidDrains += 1
			blacklistedDrains += 1
			continue
		}

		newBindings = append(newBindings, b)
	}

	f.blacklistedDrains.Set(blacklistedDrains)
	f.invalidDrains.Set(invalidDrains)
	return newBindings, nil
}

func invalidScheme(scheme string) bool {
	for _, s := range allowedSchemes {
		if s == scheme {
			return false
		}
	}

	return true
}
