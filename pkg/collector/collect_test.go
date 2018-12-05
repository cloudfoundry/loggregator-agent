package collector_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/loggregator-agent/pkg/collector"
)

var _ = Describe("Collect", func() {
	It("returns the memory metrics", func() {
		stats, err := collector.Collect()
		Expect(err).ToNot(HaveOccurred())

		Expect(stats.MemPercent).To(BeNumerically(">", 0))
		Expect(stats.MemKB).To(BeNumerically(">", 0))
	})

	It("returns the swap metrics", func() {
		stats, err := collector.Collect()
		Expect(err).ToNot(HaveOccurred())

		// Unable to test that the values are not 0 due to system not being
		// configured with swap.
		_ = stats.SwapPercent
		_ = stats.SwapKB
	})
})
