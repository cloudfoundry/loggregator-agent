package scraper_test

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/scraper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DnsIpProvider", func() {
	It("returns metrics urls from the ips returned from the lookup", func() {
		scrapeTargets := scraper.NewDNSScrapeTargetProvider("default-source", "testdata/records.json", 9100)
		targets := scrapeTargets()

		var urls []string
		for _, t := range targets {
			urls = append(urls, t.MetricURL)
		}

		Expect(urls).To(ConsistOf(
			"https://10.0.16.26:9100/metrics",
			"https://10.0.16.27:9100/metrics",
		))
	})
})
