package cups_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BindingFetcher", func() {
	var (
		getter  *SpyGetter
		fetcher *cups.BindingFetcher
		metrics *testhelper.SpyMetricClient
	)

	BeforeEach(func() {
		getter = &SpyGetter{}
		metrics = testhelper.NewMetricClient()
		fetcher = cups.NewBindingFetcher(3, getter, metrics)
	})

	BeforeEach(func() {
		getter.getResponses = []*http.Response{
			&http.Response{
				StatusCode: http.StatusOK,

				// The zzz-not-included should be left out because the limit
				// is 3. Therefore after sorting and limiting, those are
				// omitted.
				Body: ioutil.NopCloser(strings.NewReader(`
							{
							  "results": {
								"9be15160-4845-4f05-b089-40e827ba61f1": {
								  "drains": [
									"syslog://some.url",
									"syslog-v3://v3.zzz-not-included.url",
									"syslog://some.other.url",
									"syslog-v3://v3.other.url",
									"syslog-v3://v3.zzz-not-included-again.url",
									"https-v3://v3.other.url",
									"syslog-v3://v3.other-included.url"
								  ],
								  "hostname": "org.space.logspinner"
								}
							  },
							  "next_id": 50
							}
					`)),
			},

			&http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(strings.NewReader(`{ "results": { }, "next_id": null }`)),
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

	It("fetches all the pages", func() {
		fetcher.FetchBindings()
		Expect(getter.getCalled).To(Equal(2))
		Expect(getter.getNextID[0]).To(Equal(0))
		Expect(getter.getNextID[1]).To(Equal(50))
	})

	It("tracks the number of binding refreshes", func() {
		_, err := fetcher.FetchBindings()
		Expect(err).ToNot(HaveOccurred())

		Expect(
			metrics.GetMetric("BindingRefreshCount").Delta(),
		).To(Equal(uint64(1)))
	})

	It("tracks the number of requests binding", func() {
		_, err := fetcher.FetchBindings()
		Expect(err).ToNot(HaveOccurred())

		Expect(
			metrics.GetMetric("RequestCountForLastBindingRefresh").GaugeValue(),
		).To(Equal(2.0))
	})

	It("tracks the max latency of the requests", func() {
		_, err := fetcher.FetchBindings()
		Expect(err).ToNot(HaveOccurred())

		Expect(
			metrics.GetMetric("MaxLatencyForLastBindingRefreshMS").GaugeValue(),
		).To(BeNumerically(">", 0))
	})

	Context("when the status code is 200 and the body is invalid json", func() {
		BeforeEach(func() {
			getter.getResponses = []*http.Response{
				&http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(strings.NewReader("invalid")),
				},
			}
		})

		It("returns an error", func() {
			_, err := fetcher.FetchBindings()
			Expect(err).To(HaveOccurred())
		})

		It("tracks the number of binding refresh failures", func() {
			_, err := fetcher.FetchBindings()
			Expect(err).To(HaveOccurred())

			Expect(
				metrics.GetMetric("BindingRefreshFailureCount").Delta(),
			).To(Equal(uint64(1)))
		})
	})

	Context("when the status code is not 200", func() {
		BeforeEach(func() {
			getter.getResponses = []*http.Response{{StatusCode: http.StatusBadRequest}}
		})

		It("returns an error", func() {
			_, err := fetcher.FetchBindings()
			Expect(err).To(HaveOccurred())
		})

		It("tracks the number of binding refresh failures", func() {
			_, err := fetcher.FetchBindings()
			Expect(err).To(HaveOccurred())

			Expect(
				metrics.GetMetric("BindingRefreshFailureCount").Delta(),
			).To(Equal(uint64(1)))
		})
	})

	Context("when the getter fails", func() {
		BeforeEach(func() {
			getter.getResponses = []*http.Response{{StatusCode: 500}}
			getter.getError = errors.New("some-error")
		})

		It("returns an error", func() {
			_, err := fetcher.FetchBindings()
			Expect(err).To(HaveOccurred())
		})

		It("tracks the number of binding refresh failures", func() {
			_, err := fetcher.FetchBindings()
			Expect(err).To(HaveOccurred())

			Expect(
				metrics.GetMetric("BindingRefreshFailureCount").Delta(),
			).To(Equal(uint64(1)))
		})
	})
})

type SpyGetter struct {
	currentResponse int
	getCalled       int
	getNextID       []int
	getResponses    []*http.Response
	getError        error
}

func (s *SpyGetter) Get(nextID int) (*http.Response, error) {
	s.getCalled++
	s.getNextID = append(s.getNextID, nextID)
	resp := s.getResponses[s.currentResponse]
	s.currentResponse++

	if s.getError != nil {
		return nil, s.getError
	}

	// This is to ensure that the truncated max latency is at least 1 microsecond
	time.Sleep(time.Microsecond)

	return resp, nil
}
