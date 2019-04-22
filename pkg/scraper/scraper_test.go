package scraper_test

import (
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/scraper"
)

var _ = Describe("Scraper", func() {
	var (
		spyMetricsGetter *spyMetricsGetter
		spyMetricEmitter *spyMetricEmitter
		s                *scraper.Scraper
	)

	BeforeEach(func() {
		spyMetricsGetter = newSpyMetricsGetter()
		spyMetricEmitter = newSpyMetricEmitter()
		s = scraper.New(
			"some-id",
			func() []string {
				return []string{"http://some.url/metrics"}
			},
			spyMetricEmitter,
			spyMetricsGetter,
		)
	})

	It("emits a gauge metric with a default source ID", func() {
		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promOutput)),
		}

		Expect(s.Scrape()).To(Succeed())

		Expect(spyMetricEmitter.envelopes).To(And(
			ContainElement(buildEnvelope("some-id", "node_timex_pps_calibration_total", 1, nil)),
			ContainElement(buildEnvelope("some-id", "node_timex_pps_error_total", 2, nil)),
			ContainElement(buildEnvelope("some-id", "node_timex_pps_frequency_hertz", 3, nil)),
			ContainElement(buildEnvelope("some-id", "node_timex_pps_jitter_seconds", 4, nil)),
			ContainElement(buildEnvelope("some-id", "node_timex_pps_jitter_total", 5, nil)),
			ContainElement(buildEnvelope("some-id", "promhttp_metric_handler_requests_total", 6, map[string]string{"code": "200"})),
			ContainElement(buildEnvelope("some-id", "promhttp_metric_handler_requests_total", 7, map[string]string{"code": "500"})),
			ContainElement(buildEnvelope("some-id", "promhttp_metric_handler_requests_total", 8, map[string]string{"code": "503"})),
		))

		var addr string
		Eventually(spyMetricsGetter.a).Should(Receive(&addr))
		Expect(addr).To(Equal("http://some.url/metrics"))
	})

	It("emits a gauge metric with the default source ID", func() {
		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(smallPromOutput)),
		}
		Expect(s.Scrape()).To(Succeed())

		Expect(spyMetricEmitter.envelopes).To(And(
			ContainElement(buildEnvelope("source-1", "node_timex_pps_calibration_total", 1, nil)),
			ContainElement(buildEnvelope("source-2", "node_timex_pps_error_total", 2, nil)),
		))

		var addr string
		Eventually(spyMetricsGetter.a).Should(Receive(&addr))
		Expect(addr).To(Equal("http://some.url/metrics"))
	})

	It("scrapes all endpoints even when one fails", func() {
		s = scraper.New(
			"some-id",
			func() []string {
				return []string{
					"http://some.url/metrics",
					"http://some.other.url/metrics",
				}
			},
			spyMetricEmitter,
			spyMetricsGetter,
		)

		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promInvalid)),
		}
		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(smallPromOutput)),
		}
		Expect(s.Scrape()).To(HaveOccurred())

		Expect(spyMetricEmitter.envelopes).To(And(
			ContainElement(buildEnvelope("source-1", "node_timex_pps_calibration_total", 1, nil)),
			ContainElement(buildEnvelope("source-2", "node_timex_pps_error_total", 2, nil)),
		))
	})

	It("scrapes endpoints asynchronously", func() {
		s = scraper.New(
			"some-id",
			func() []string {
				return []string{
					"http://some.url/metrics",
					"http://some.other.url/metrics",
					"http://some.other.other.url/metrics",
				}
			},
			spyMetricEmitter,
			spyMetricsGetter,
		)

		for i := 0; i < 3; i++ {
			spyMetricsGetter.delay <- 1 * time.Second
			spyMetricsGetter.resp <- &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(smallPromOutput)),
			}
		}

		go s.Scrape()

		Eventually(func() int { return len(spyMetricsGetter.a) }, 1).Should(Equal(3))
	})

	It("returns a compilation of errors from scrapes", func() {
		s = scraper.New(
			"some-id",
			func() []string {
				return []string{
					"http://some.url/metrics",
					"http://some.other.url/metrics",
					"http://some.other.other.url/metrics",
				}
			},
			spyMetricEmitter,
			spyMetricsGetter,
		)

		spyMetricsGetter.err <- errors.New("something")
		spyMetricsGetter.err <- errors.New("something")
		spyMetricsGetter.err <- errors.New("something")

		err := s.Scrape()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("something,something,something"))
	})

	It("returns an error if the parser fails", func() {
		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promInvalid)),
		}
		Expect(s.Scrape()).To(HaveOccurred())
	})

	It("returns an error if MetricsGetter returns an error", func() {
		spyMetricsGetter.err <- errors.New("some-error")
		Expect(s.Scrape()).To(MatchError("some-error"))
	})

	It("returns an error if the response is not a 200 OK", func() {
		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 400,
			Body:       ioutil.NopCloser(strings.NewReader("")),
		}
		Expect(s.Scrape()).To(HaveOccurred())
	})

	It("ignores unknown metric types", func() {
		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promSummary)),
		}
		Expect(s.Scrape()).To(Succeed())
		Expect(spyMetricEmitter.envelopes).To(BeEmpty())
	})

	It("can emit metrics for attempted scrapes", func() {
		spyMetricClient := testhelper.NewMetricClient()
		s = scraper.New(
			"some-id",
			func() []string {
				return []string{
					"http://some.url/metrics",
					"http://some.other.url/metrics",
					"http://some.other.other.url/metrics",
				}
			},
			spyMetricEmitter,
			spyMetricsGetter,
			scraper.WithMetricsClient(spyMetricClient),
		)

		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promSummary)),
		}

		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promSummary)),
		}

		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promSummary)),
		}

		err := s.Scrape()
		Expect(err).ToNot(HaveOccurred())

		metric := spyMetricClient.GetMetric("last_total_attempted_scrapes", map[string]string{"unit": "total"})
		Eventually(metric.Value).Should(BeNumerically("==", 3))
	})

	It("can emit metrics for failed scrapes", func() {
		spyMetricClient := testhelper.NewMetricClient()
		s = scraper.New(
			"some-id",
			func() []string {
				return []string{
					"http://some.url/metrics",
					"http://some.other.url/metrics",
					"http://some.other.other.url/metrics",
				}
			},
			spyMetricEmitter,
			spyMetricsGetter,
			scraper.WithMetricsClient(spyMetricClient),
		)

		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promSummary)),
		}

		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 500,
			Body:       ioutil.NopCloser(strings.NewReader("")),
		}

		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promSummary)),
		}

		err := s.Scrape()
		Expect(err).To(HaveOccurred())

		metric := spyMetricClient.GetMetric("last_total_failed_scrapes", map[string]string{"unit": "total"})
		Eventually(metric.Value).Should(BeNumerically("==", 1))
	})

	It("can emit metrics for scrape duration", func() {
		spyMetricClient := testhelper.NewMetricClient()
		s = scraper.New(
			"some-id",
			func() []string {
				return []string{
					"http://some.url/metrics",
				}
			},
			spyMetricEmitter,
			spyMetricsGetter,
			scraper.WithMetricsClient(spyMetricClient),
		)

		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promSummary)),
		}

		spyMetricsGetter.delay <- 10 * time.Millisecond

		err := s.Scrape()
		Expect(err).ToNot(HaveOccurred())

		metric := spyMetricClient.GetMetric("last_total_scrape_duration", map[string]string{"unit": "ms"})
		Eventually(metric.Value).Should(BeNumerically(">=", 10))
	})
})

const (
	promOutput = `
# HELP node_timex_pps_calibration_total Pulse per second count of calibration intervals.
# TYPE node_timex_pps_calibration_total counter
node_timex_pps_calibration_total 1
# HELP node_timex_pps_error_total Pulse per second count of calibration errors.
# TYPE node_timex_pps_error_total counter
node_timex_pps_error_total 2
# HELP node_timex_pps_frequency_hertz Pulse per second frequency.
# TYPE node_timex_pps_frequency_hertz gauge
node_timex_pps_frequency_hertz 3
# HELP node_timex_pps_jitter_seconds Pulse per second jitter.
# TYPE node_timex_pps_jitter_seconds gauge
node_timex_pps_jitter_seconds 4
# HELP node_timex_pps_jitter_total Pulse per second count of jitter limit exceeded events.
# TYPE node_timex_pps_jitter_total counter
node_timex_pps_jitter_total 5
# HELP promhttp_metric_handler_requests_total Total number of scrapes by HTTP status code.
# TYPE promhttp_metric_handler_requests_total counter
promhttp_metric_handler_requests_total{code="200"} 6
promhttp_metric_handler_requests_total{code="500"} 7
promhttp_metric_handler_requests_total{code="503"} 8
`
	smallPromOutput = `
# HELP node_timex_pps_calibration_total Pulse per second count of calibration intervals.
# TYPE node_timex_pps_calibration_total counter
node_timex_pps_calibration_total{source_id="source-1"} 1
# HELP node_timex_pps_error_total Pulse per second count of calibration errors.
# TYPE node_timex_pps_error_total counter
node_timex_pps_error_total{source_id="source-2"} 2
`

	promInvalid = `
# HELP garbage Pulse per second count of jitter limit exceeded events.
# TYPE garbage counter
garbage invalid
`

	promSummary = `
# HELP go_gc_duration_seconds A summary of the GC invocation durations.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0"} 9.5e-08
go_gc_duration_seconds{quantile="0.25"} 0.000157366
go_gc_duration_seconds{quantile="0.5"} 0.000300143
go_gc_duration_seconds{quantile="0.75"} 0.001091972
go_gc_duration_seconds{quantile="1"} 0.011609012
go_gc_duration_seconds_sum 0.346341323
go_gc_duration_seconds_count 331
`
)

func buildEnvelope(sourceID, name string, value float64, tags map[string]string) *loggregator_v2.Envelope {
	if tags == nil {
		tags = map[string]string{}
	}

	return &loggregator_v2.Envelope{
		SourceId: sourceID,
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					name: {Value: value},
				},
			},
		},
		Tags: tags,
	}
}

type spyMetricEmitter struct {
	envelopes []*loggregator_v2.Envelope
	mu        sync.Mutex
}

func newSpyMetricEmitter() *spyMetricEmitter {
	return &spyMetricEmitter{}
}

func (s *spyMetricEmitter) EmitGauge(opts ...loggregator.EmitGaugeOption) {
	e := &loggregator_v2.Envelope{
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: make(map[string]*loggregator_v2.GaugeValue),
			},
		},
		Tags: map[string]string{},
	}

	for _, o := range opts {
		o(e)
	}

	s.mu.Lock()
	s.envelopes = append(s.envelopes, e)
	s.mu.Unlock()
}

type spyMetricsGetter struct {
	a     chan string
	resp  chan *http.Response
	delay chan time.Duration
	err   chan error
}

func newSpyMetricsGetter() *spyMetricsGetter {
	return &spyMetricsGetter{
		a:     make(chan string, 100),
		resp:  make(chan *http.Response, 100),
		delay: make(chan time.Duration, 100),
		err:   make(chan error, 100),
	}
}

func (s *spyMetricsGetter) Get(addr string) (*http.Response, error) {
	s.a <- addr

	if len(s.delay) > 0 {
		time.Sleep(<-s.delay)
	}

	var err error
	if len(s.err) > 0 {
		err = <-s.err
	}

	var resp *http.Response
	select {
	case resp = <-s.resp:
		return resp, err
	default:
		return nil, err
	}
}
