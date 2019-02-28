package cups

import (
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

// Metrics is the client used to expose gauge and counter metrics.
type metrics interface {
	NewGauge(string) func(float64)
}

type FilteredBindingFetcher struct {
	ipChecker         IPChecker
	br                binding.Fetcher
	logger            *log.Logger
	invalidDrains     func(float64)
	blacklistedDrains func(float64)
}

func NewFilteredBindingFetcher(c IPChecker, b binding.Fetcher, m metrics, lc *log.Logger) *FilteredBindingFetcher {
	return &FilteredBindingFetcher{
		ipChecker:         c,
		br:                b,
		logger:            lc,
		invalidDrains:     m.NewGauge("InvalidDrains"),
		blacklistedDrains: m.NewGauge("BlacklistedDrains"),
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

	f.blacklistedDrains(blacklistedDrains)
	f.invalidDrains(invalidDrains)
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
