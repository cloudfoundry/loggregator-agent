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
			func() []scraper.Target {
				return []scraper.Target{
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.url/metrics",
					},
				}
			},
			spyMetricEmitter,
			spyMetricsGetter.Get,
		)
	})

	Context("gauges", func() {
		It("emits a gauge metric with the target source ID", func() {
			spyMetricsGetter.resp <- &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(promOutput)),
			}

			Expect(s.Scrape()).To(Succeed())

			Expect(spyMetricEmitter.envelopes).To(And(
				ContainElement(buildGauge("some-id", "some-instance-id", "node_timex_pps_frequency_hertz", 3, nil)),
				ContainElement(buildGauge("some-id", "some-instance-id", "node_timex_pps_jitter_seconds", 4, nil)),
			))
		})

		It("emits a gauge metric with tagged source ID", func() {
			spyMetricsGetter.resp <- &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(smallGaugeOutput)),
			}

			Expect(s.Scrape()).To(Succeed())

			Expect(spyMetricEmitter.envelopes).To(And(
				ContainElement(buildGauge("source-1", "some-instance-id", "gauge_1", 1, nil)),
				ContainElement(buildGauge("source-2", "some-instance-id", "gauge_2", 2, nil)),
			))
		})
	})

	Context("counters", func() {
		It("emits a counter metric with a default source ID", func() {
			spyMetricsGetter.resp <- &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(promOutput)),
			}

			Expect(s.Scrape()).To(Succeed())

			Expect(spyMetricEmitter.envelopes).To(And(
				ContainElement(buildCounter("some-id", "some-instance-id", "node_timex_pps_calibration_total", 1, nil)),
				ContainElement(buildCounter("some-id", "some-instance-id", "node_timex_pps_error_total", 2, nil)),
				ContainElement(buildCounter("some-id", "some-instance-id", "node_timex_pps_jitter_total", 5, nil)),
				ContainElement(buildCounter("some-id", "some-instance-id", "promhttp_metric_handler_requests_total", 6, map[string]string{"code": "200"})),
				ContainElement(buildCounter("some-id", "some-instance-id", "promhttp_metric_handler_requests_total", 7, map[string]string{"code": "500"})),
				ContainElement(buildCounter("some-id", "some-instance-id", "promhttp_metric_handler_requests_total", 8, map[string]string{"code": "503"})),
			))

			var addr string
			Eventually(spyMetricsGetter.addrs).Should(Receive(&addr))
			Expect(addr).To(Equal("http://some.url/metrics"))
		})

		It("emits a counter metric with tagged source ID", func() {
			spyMetricsGetter.resp <- &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(smallCounterOutput)),
			}

			Expect(s.Scrape()).To(Succeed())

			Expect(spyMetricEmitter.envelopes).To(And(
				ContainElement(buildCounter("source-1", "some-instance-id", "counter_1", 1, nil)),
				ContainElement(buildCounter("source-2", "some-instance-id", "counter_2", 2, nil)),
			))
		})

		It("ignores counter metrics with float values", func() {
			spyMetricsGetter.resp <- &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(floatCounterOutput)),
			}

			Expect(s.Scrape()).To(Succeed())

			Expect(spyMetricEmitter.envelopes).To(ConsistOf(
				buildCounter("source-1", "some-instance-id", "counter_int", 1, nil),
			))
		})
	})

	Context("histograms", func() {
		It("emits a histogram with a default source ID", func() {
			spyMetricsGetter.resp <- &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(histogramOutput)),
			}

			Expect(s.Scrape()).To(Succeed())

			Expect(spyMetricEmitter.envelopes).To(And(
				ContainElement(buildCounter("some-id", "some-instance-id", "http_request_duration_seconds_bucket", 133988, map[string]string{"le": "1"})),
				ContainElement(buildCounter("some-id", "some-instance-id", "http_request_duration_seconds_bucket", 144320, map[string]string{"le": "+Inf"})),
				ContainElement(buildGauge("some-id", "some-instance-id", "http_request_duration_seconds_sum", 53423, nil)),
				ContainElement(buildCounter("some-id", "some-instance-id", "http_request_duration_seconds_count", 144320, nil)),
			))
		})

		It("emits a histogram with tagged source ID", func() {
			spyMetricsGetter.resp <- &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(multiHistogramOutput)),
			}

			Expect(s.Scrape()).To(Succeed())

			Expect(spyMetricEmitter.envelopes).To(And(
				ContainElement(buildCounter("source-1", "some-instance-id", "histogram_1_bucket", 133988, map[string]string{"le": "1"})),
				ContainElement(buildGauge("source-1", "some-instance-id", "histogram_1_sum", 53423, nil)),
				ContainElement(buildCounter("source-1", "some-instance-id", "histogram_1_count", 133988, nil)),
				ContainElement(buildCounter("source-2", "some-instance-id", "histogram_2_bucket", 133988, map[string]string{"le": "1"})),
				ContainElement(buildGauge("source-2", "some-instance-id", "histogram_2_sum", 53423, nil)),
				ContainElement(buildCounter("source-2", "some-instance-id", "histogram_2_count", 133988, nil)),
			))
		})
	})

	Context("summaries", func() {
		It("emits a summary with a default source ID", func() {
			spyMetricsGetter.resp <- &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(promSummary)),
			}

			Expect(s.Scrape()).To(Succeed())

			Expect(spyMetricEmitter.envelopes).To(And(
				ContainElement(buildGauge("some-id", "some-instance-id", "go_gc_duration_seconds", 9.5e-08, map[string]string{"quantile": "0"})),
				ContainElement(buildGauge("some-id", "some-instance-id", "go_gc_duration_seconds", 0.000157366, map[string]string{"quantile": "0.25"})),
				ContainElement(buildGauge("some-id", "some-instance-id", "go_gc_duration_seconds", 0.000300143, map[string]string{"quantile": "0.5"})),
				ContainElement(buildGauge("some-id", "some-instance-id", "go_gc_duration_seconds", 0.001091972, map[string]string{"quantile": "0.75"})),
				ContainElement(buildGauge("some-id", "some-instance-id", "go_gc_duration_seconds", 0.011609012, map[string]string{"quantile": "1"})),
				ContainElement(buildGauge("some-id", "some-instance-id", "go_gc_duration_seconds_sum", 0.346341323, nil)),
				ContainElement(buildCounter("some-id", "some-instance-id", "go_gc_duration_seconds_count", 331, nil)),
			))

		})
	})

	It("ignores unknown metric types", func() {
		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promUntyped)),
		}
		Expect(s.Scrape()).To(Succeed())
		Expect(spyMetricEmitter.envelopes).To(BeEmpty())
	})

	It("scrapes the given endpoint", func() {
		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(smallCounterOutput)),
		}
		Expect(s.Scrape()).To(Succeed())

		var addr string
		Eventually(spyMetricsGetter.addrs).Should(Receive(&addr))
		Expect(addr).To(Equal("http://some.url/metrics"))
	})

	It("scrapes the given endpoint with given headers", func() {
		headers := map[string]string{
			"header1": "value1",
			"header2": "value2",
		}

		s = scraper.New(
			func() []scraper.Target {
				return []scraper.Target{
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.url/metrics",
						Headers:    headers,
					},
				}
			},
			spyMetricEmitter,
			spyMetricsGetter.Get,
		)

		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promSummary)),
		}

		Expect(s.Scrape()).To(Succeed())
		Eventually(spyMetricsGetter.headers).Should(Receive(Equal(headers)))
	})

	It("scrapes all endpoints even when one fails", func() {
		s = scraper.New(
			func() []scraper.Target {
				return []scraper.Target{
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.url/metrics",
					},
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.other.url/metrics",
					},
				}
			},
			spyMetricEmitter,
			spyMetricsGetter.Get,
		)

		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(promInvalid)),
		}
		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(smallGaugeOutput)),
		}
		Expect(s.Scrape()).To(HaveOccurred())

		Expect(spyMetricEmitter.envelopes).To(And(
			ContainElement(buildGauge("source-1", "some-instance-id", "gauge_1", 1, nil)),
			ContainElement(buildGauge("source-2", "some-instance-id", "gauge_2", 2, nil)),
		))
	})

	It("scrapes endpoints asynchronously", func() {
		s = scraper.New(
			func() []scraper.Target {
				return []scraper.Target{
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.url/metrics",
					},
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.other.url/metrics",
					},
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.other.other.url/metrics",
					},
				}
			},
			spyMetricEmitter,
			spyMetricsGetter.Get,
		)

		for i := 0; i < 3; i++ {
			spyMetricsGetter.delay <- 1 * time.Second
			spyMetricsGetter.resp <- &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(smallGaugeOutput)),
			}
		}

		go s.Scrape()

		Eventually(func() int { return len(spyMetricsGetter.addrs) }, 1).Should(Equal(3))
	})

	It("returns a compilation of errors from scrapes", func() {
		s = scraper.New(
			func() []scraper.Target {
				return []scraper.Target{
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.url/metrics",
					},
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.other.url/metrics",
					},
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.other.other.url/metrics",
					},
				}
			},
			spyMetricEmitter,
			spyMetricsGetter.Get,
		)

		spyMetricsGetter.err <- errors.New("something")
		spyMetricsGetter.err <- errors.New("something")
		spyMetricsGetter.err <- errors.New("something")

		err := s.Scrape()
		Expect(err).To(HaveOccurred())
		Expect(err.(*scraper.ScraperError).Errors).To(ConsistOf(
			&scraper.ScrapeError{
				ID:         "some-id",
				InstanceID: "some-instance-id",
				MetricURL:  "http://some.url/metrics",
				Err: errors.New("something"),
			},
			&scraper.ScrapeError{
				ID:         "some-id",
				InstanceID: "some-instance-id",
				MetricURL:  "http://some.other.url/metrics",
				Err: errors.New("something"),
			},
			&scraper.ScrapeError{
				ID:         "some-id",
				InstanceID: "some-instance-id",
				MetricURL:  "http://some.other.other.url/metrics",
				Err: errors.New("something"),
			},
		))
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
		Expect(s.Scrape()).To(MatchError(ContainSubstring("some-error")))
	})

	It("returns an error if the response is not a 200 OK", func() {
		spyMetricsGetter.resp <- &http.Response{
			StatusCode: 400,
			Body:       ioutil.NopCloser(strings.NewReader("")),
		}
		Expect(s.Scrape()).To(HaveOccurred())
	})

	It("can emit metrics for attempted scrapes", func() {
		spyMetricClient := testhelper.NewMetricClient()
		s = scraper.New(
			func() []scraper.Target {
				return []scraper.Target{
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.url/metrics",
					},
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.other.url/metrics",
					},
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.other.other.url/metrics",
					},
				}
			},
			spyMetricEmitter,
			spyMetricsGetter.Get,
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
			func() []scraper.Target {
				return []scraper.Target{
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.url/metrics",
					},
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.other.url/metrics",
					},
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.other.other.url/metrics",
					},
				}
			},
			spyMetricEmitter,
			spyMetricsGetter.Get,
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
			func() []scraper.Target {
				return []scraper.Target{
					{
						ID:         "some-id",
						InstanceID: "some-instance-id",
						MetricURL:  "http://some.url/metrics",
					},
				}
			},
			spyMetricEmitter,
			spyMetricsGetter.Get,
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
	histogramOutput = `
# A histogram, which has a pretty complex representation in the text format:
# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="1"} 133988
http_request_duration_seconds_bucket{le="+Inf"} 144320
http_request_duration_seconds_sum 53423
http_request_duration_seconds_count 144320
`

	smallGaugeOutput = `
# HELP gauge_1 Example metric
# TYPE gauge_1 gauge
gauge_1{source_id="source-1"} 1
# HELP gauge_2 Example metric
# TYPE gauge_2 gauge
gauge_2{source_id="source-2"} 2
`
	smallCounterOutput = `
# HELP counter_1 Example metric
# TYPE counter_1 counter
counter_1{source_id="source-1"} 1
# HELP counter_2 Example metric
# TYPE counter_2 counter
counter_2{source_id="source-2"} 2
`
	multiHistogramOutput = `
# A histogram, which has a pretty complex representation in the text format:
# HELP histogram_1 A histogram of the request duration.
# TYPE histogram_1 histogram
histogram_1_bucket{le="1", source_id="source-1"} 133988
histogram_1_sum{source_id="source-1"} 53423
histogram_1_count{source_id="source-1"} 133988
# HELP histogram_2 A histogram of the request duration.
# TYPE histogram_2 histogram
histogram_2_bucket{le="1", source_id="source-2"} 133988
histogram_2_sum{source_id="source-2"} 53423
histogram_2_count{source_id="source-2"} 133988
`

	floatCounterOutput = `
# HELP counter_int Example metric
# TYPE counter_int counter
counter_int{source_id="source-1"} 1
# HELP counter_float Example metric
# TYPE counter_float counter
counter_float{source_id="source-2"} 2.2
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
	promUntyped = `
test 9.5e-08
`
)

func buildGauge(sourceID, instanceID, name string, value float64, tags map[string]string) *loggregator_v2.Envelope {
	if tags == nil {
		tags = map[string]string{}
	}

	return &loggregator_v2.Envelope{
		SourceId:   sourceID,
		InstanceId: instanceID,
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

func buildCounter(sourceID, instanceID, name string, value float64, tags map[string]string) *loggregator_v2.Envelope {
	if tags == nil {
		tags = map[string]string{}
	}

	return &loggregator_v2.Envelope{
		SourceId:   sourceID,
		InstanceId: instanceID,
		Message: &loggregator_v2.Envelope_Counter{
			Counter: &loggregator_v2.Counter{
				Name:  name,
				Total: uint64(value),
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

func (s *spyMetricEmitter) EmitCounter(name string, opts ...loggregator.EmitCounterOption) {
	e := &loggregator_v2.Envelope{
		Message: &loggregator_v2.Envelope_Counter{
			Counter: &loggregator_v2.Counter{
				Name: name,
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
	addrs   chan string
	headers chan map[string]string

	resp  chan *http.Response
	delay chan time.Duration
	err   chan error
}

func newSpyMetricsGetter() *spyMetricsGetter {
	return &spyMetricsGetter{
		addrs:   make(chan string, 100),
		headers: make(chan map[string]string, 100),
		resp:    make(chan *http.Response, 100),
		delay:   make(chan time.Duration, 100),
		err:     make(chan error, 100),
	}
}

func (s *spyMetricsGetter) Get(addr string, headers map[string]string) (*http.Response, error) {
	s.addrs <- addr
	s.headers <- headers

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
