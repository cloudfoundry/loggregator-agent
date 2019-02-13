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

type FilteredBindingFetcher struct {
	ipChecker IPChecker
	br        binding.Fetcher
	logger    *log.Logger
}

func NewFilteredBindingFetcher(c IPChecker, b binding.Fetcher, lc *log.Logger) *FilteredBindingFetcher {
	return &FilteredBindingFetcher{
		ipChecker: c,
		br:        b,
		logger:    lc,
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

	for _, binding := range sourceBindings {
		scheme, host, err := f.ipChecker.ParseHost(binding.Drain)
		if err != nil {
			f.logger.Printf("failed to parse host for drain URL: %s", err)
			continue
		}

		if invalidScheme(scheme) {
			continue
		}

		ip, err := f.ipChecker.ResolveAddr(host)
		if err != nil {
			msg := fmt.Sprintf("failed to resolve syslog drain host: %s", host)
			f.logger.Println(msg, err)
			continue
		}

		err = f.ipChecker.CheckBlacklist(ip)
		if err != nil {
			msg := fmt.Sprintf("syslog drain blacklisted: %s (%s)", host, ip)
			f.logger.Println(msg, err)
			continue
		}

		newBindings = append(newBindings, binding)
	}

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
