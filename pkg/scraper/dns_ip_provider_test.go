package scraper_test

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/scraper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DnsIpProvider", func() {
	It("returns metrics urls from the ips returned from the lookup", func() {
		urlProvider := scraper.NewDNSMetricUrlProvider("testdata/records.json", 9100)
		urls := urlProvider()

		Expect(urls).To(ConsistOf(
			"http://10.0.16.26:9100/metrics",
			"http://10.0.16.27:9100/metrics",
		))
	})
})
