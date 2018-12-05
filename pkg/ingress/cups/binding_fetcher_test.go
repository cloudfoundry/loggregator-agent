package cups_test

import (
	"time"

	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/binding"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BindingFetcher", func() {
	var (
		getter    *SpyGetter
		fetcher   *cups.BindingFetcher
		metrics   *testhelper.SpyMetricClient
		maxDrains = 3
	)

	BeforeEach(func() {
		getter = &SpyGetter{}
		metrics = testhelper.NewMetricClient()
		fetcher = cups.NewBindingFetcher(maxDrains, getter, metrics)
	})

	BeforeEach(func() {
		getter.bindings = []binding.Binding{
			{
				AppID: "9be15160-4845-4f05-b089-40e827ba61f1",
				Drains: []string{
					"syslog://some.url",
					"syslog-v3://v3.zzz-not-included.url",
					"syslog://some.other.url",
					"syslog-v3://v3.other.url",
					"syslog-v3://v3.zzz-not-included-again.url",
					"https-v3://v3.other.url",
					"syslog-v3://v3.other-included.url",
				},
				Hostname: "org.space.logspinner",
			},
		}
	})

	It("returns limited v3 bindings", func() {
		bindings, err := fetcher.FetchBindings()
		Expect(err).ToNot(HaveOccurred())
		Expect(bindings).To(HaveLen(3))

		appID := "9be15160-4845-4f05-b089-40e827ba61f1"
		Expect(bindings).To(Equal([]syslog.Binding{
			syslog.Binding{
				AppId:    appID,
				Hostname: "org.space.logspinner",
				Drain:    "https://v3.other.url",
			},
			syslog.Binding{
				AppId:    appID,
				Hostname: "org.space.logspinner",
				Drain:    "syslog://v3.other-included.url",
			},
			syslog.Binding{
				AppId:    appID,
				Hostname: "org.space.logspinner",
				Drain:    "syslog://v3.other.url",
			},
		}))
	})

	It("tracks the number of binding refreshes", func() {
		_, err := fetcher.FetchBindings()
		Expect(err).ToNot(HaveOccurred())

		Expect(
			metrics.GetMetric("BindingRefreshCount").Delta(),
		).To(Equal(uint64(1)))
	})

	It("tracks the max latency of the requests", func() {
		_, err := fetcher.FetchBindings()
		Expect(err).ToNot(HaveOccurred())

		Expect(
			metrics.GetMetric("LatencyForLastBindingRefreshMS").GaugeValue(),
		).To(BeNumerically(">", 0))
	})

	It("returns all the bindings when there are fewer bindings than the limit", func() {
		getter.bindings = []binding.Binding{
			{
				AppID: "9be15160-4845-4f05-b089-40e827ba61f1",
				Drains: []string{
					"syslog-v3://v3.other.url",
				},
				Hostname: "org.space.logspinner",
			},
		}
		fetcher = cups.NewBindingFetcher(2, getter, metrics)
		bindings, err := fetcher.FetchBindings()
		Expect(err).ToNot(HaveOccurred())
		Expect(bindings).To(HaveLen(1))

		Expect(bindings).To(Equal([]syslog.Binding{
			syslog.Binding{
				AppId:    "9be15160-4845-4f05-b089-40e827ba61f1",
				Hostname: "org.space.logspinner",
				Drain:    "syslog://v3.other.url",
			},
		}))
	})
})

type SpyGetter struct {
	bindings []binding.Binding
}

func (s *SpyGetter) Get() []binding.Binding {
	time.Sleep(10 * time.Millisecond)
	return s.bindings
}
