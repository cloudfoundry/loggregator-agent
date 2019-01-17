package scraper_test

import (
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/loggregator-agent/pkg/scraper"
)

var _ = Describe("DnsIpProvider", func() {
	It("returns metrics urls from the ips returned from the lookup", func() {
		var lookedUp string
		dnsLookup := func(addr string) ([]net.IP, error) {
			lookedUp = addr
			ips := []net.IP{
				net.ParseIP("127.0.0.1"),
				net.ParseIP("127.0.0.2"),
			}
			return ips, nil
		}

		urlProvider := scraper.NewDNSMetricUrlProvider(dnsLookup, 9100)
		urls := urlProvider()

		Expect(lookedUp).To(Equal("q-s4.*.bosh"))
		Expect(urls).To(ConsistOf(
			"http://127.0.0.1:9100/metrics",
			"http://127.0.0.2:9100/metrics",
		))
	})
})
