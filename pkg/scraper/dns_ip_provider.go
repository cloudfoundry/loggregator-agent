package scraper

import (
	"fmt"
	"net"

	"log"
)

func NewDNSMetricUrlProvider(lookup func(string) ([]net.IP, error), port int) func() []string {
	allBoshVms := "q-s4.*.bosh"
	return func() []string {
		var metricAddrs []string
		ips, err := lookup(allBoshVms)
		if err != nil {
			log.Print(err)
			return nil
		}

		for _, ip := range ips {
			metricAddrs = append(metricAddrs, fmt.Sprintf("http://%s:%d/metrics", ip, port))
		}

		return metricAddrs
	}
}
