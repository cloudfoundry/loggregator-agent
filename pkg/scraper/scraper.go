package scraper

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"errors"
	"fmt"
	"github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/go-loggregator"
)

type Scraper struct {
	sourceID            string
	addrProvider        func() []string
	metricsEmitter      MetricsEmitter
	systemMetricsClient MetricsGetter
	urlsScraped         metrics.Gauge
	failedScrapes       metrics.Gauge
	scrapeDuration      metrics.Gauge
}

type ScrapeOption func(s *Scraper)

type MetricsEmitter interface {
	EmitGauge(opts ...loggregator.EmitGaugeOption)
	EmitCounter(name string, opts ...loggregator.EmitCounterOption)
}

type metricsClient interface {
	NewGauge(name string, opts ...metrics.MetricOption) metrics.Gauge
}

type MetricsGetter interface {
	Get(addr string) (*http.Response, error)
}

func New(
	sourceID string,
	addrProvider func() []string,
	e MetricsEmitter,
	sc MetricsGetter,
	opts ...ScrapeOption,
) *Scraper {
	scraper := &Scraper{
		sourceID:            sourceID,
		addrProvider:        addrProvider,
		metricsEmitter:      e,
		systemMetricsClient: sc,
		urlsScraped:         &defaultGauge{},
		scrapeDuration:      &defaultGauge{},
		failedScrapes:       &defaultGauge{},
	}

	for _, o := range opts {
		o(scraper)
	}

	return scraper
}

func WithMetricsClient(m metricsClient) ScrapeOption {
	return func(s *Scraper) {
		s.urlsScraped = m.NewGauge(
			"last_total_attempted_scrapes",
			metrics.WithMetricTags(map[string]string{"unit": "total"}),
		)

		s.failedScrapes = m.NewGauge(
			"last_total_failed_scrapes",
			metrics.WithMetricTags(map[string]string{"unit": "total"}),
		)

		s.scrapeDuration = m.NewGauge(
			"last_total_scrape_duration",
			metrics.WithMetricTags(map[string]string{"unit": "ms"}),
		)
	}
}

func (s *Scraper) Scrape() error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		s.scrapeDuration.Set(float64(duration / time.Millisecond))
	}()

	addrList := s.addrProvider()
	errs := make(chan string, len(addrList))
	var wg sync.WaitGroup

	s.urlsScraped.Set(float64(len(addrList)))
	for _, a := range addrList {
		wg.Add(1)

		go func(addr string) {
			scrapeResult, err := s.scrape(addr)
			if err != nil {
				errs <- err.Error()
			}

			s.emitMetrics(scrapeResult)
			wg.Done()
		}(a)
	}

	wg.Wait()
	close(errs)

	s.failedScrapes.Set(float64(len(errs)))
	if len(errs) > 0 {
		var errorsSlice []string
		for e := range errs {
			errorsSlice = append(errorsSlice, e)
		}
		return errors.New(strings.Join(errorsSlice, ","))
	}

	return nil
}

func (s *Scraper) scrape(addr string) (map[string]*io_prometheus_client.MetricFamily, error) {
	resp, err := s.systemMetricsClient.Get(addr)
	if err != nil {
		return nil, err
	}

	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, body)
	}

	p := &expfmt.TextParser{}
	res, err := p.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, err
	}

	return res, err
}

func (s *Scraper) emitMetrics(res map[string]*io_prometheus_client.MetricFamily) {
	for _, family := range res {
		name := family.GetName()

		for _, metric := range family.GetMetric() {
			switch family.GetType() {
			case io_prometheus_client.MetricType_GAUGE:
				s.emitGauge(name, metric)
			case io_prometheus_client.MetricType_COUNTER:
				s.emitCounter(name, metric)
			case io_prometheus_client.MetricType_HISTOGRAM:
				s.emitHistogram(name, metric)
			case io_prometheus_client.MetricType_SUMMARY:
				s.emitSummary(name, metric)
			default:
				return
			}
		}
	}
}

func (s *Scraper) emitGauge(name string, metric *io_prometheus_client.Metric) {
	val := metric.GetGauge().GetValue()
	sourceID, tags := parseTags(s.sourceID, metric)

	s.metricsEmitter.EmitGauge(
		loggregator.WithGaugeValue(name, val, ""),
		loggregator.WithGaugeSourceInfo(sourceID, ""),
		loggregator.WithEnvelopeTags(tags),
	)
}

func (s *Scraper) emitCounter(name string, m *io_prometheus_client.Metric) {
	val := m.GetCounter().GetValue()
	if val != float64(uint64(val)) {
		return
	}

	sourceID, tags := parseTags(s.sourceID, m)
	s.metricsEmitter.EmitCounter(
		name,
		loggregator.WithTotal(uint64(val)),
		loggregator.WithCounterSourceInfo(sourceID, ""),
		loggregator.WithEnvelopeTags(tags),
	)
}

func (s *Scraper) emitHistogram(name string, m *io_prometheus_client.Metric) {
	histogram := m.GetHistogram()

	sourceID, tags := parseTags(s.sourceID, m)
	s.metricsEmitter.EmitGauge(
		loggregator.WithGaugeValue(name+"_sum", histogram.GetSampleSum(), ""),
		loggregator.WithGaugeSourceInfo(sourceID, ""),
		loggregator.WithEnvelopeTags(tags),
	)
	s.metricsEmitter.EmitCounter(
		name+"_count",
		loggregator.WithTotal(histogram.GetSampleCount()),
		loggregator.WithCounterSourceInfo(sourceID, ""),
		loggregator.WithEnvelopeTags(tags),
	)
	for _, bucket := range histogram.GetBucket() {
		s.metricsEmitter.EmitGauge(
			loggregator.WithGaugeValue(name+"_bucket", float64(bucket.GetCumulativeCount()), ""),
			loggregator.WithGaugeSourceInfo(sourceID, ""),
			loggregator.WithEnvelopeTags(tags),
			loggregator.WithEnvelopeTag("le", strconv.FormatFloat(bucket.GetUpperBound(), 'g', -1, 64)),
		)
	}
}

func (s *Scraper) emitSummary(name string, m *io_prometheus_client.Metric) {
	summary := m.GetSummary()
	sourceID, tags := parseTags(s.sourceID, m)
	s.metricsEmitter.EmitGauge(
		loggregator.WithGaugeValue(name+"_sum", summary.GetSampleSum(), ""),
		loggregator.WithGaugeSourceInfo(sourceID, ""),
		loggregator.WithEnvelopeTags(tags),
	)
	s.metricsEmitter.EmitCounter(
		name+"_count",
		loggregator.WithTotal(summary.GetSampleCount()),
		loggregator.WithCounterSourceInfo(sourceID, ""),
		loggregator.WithEnvelopeTags(tags),
	)
	for _, quantile := range summary.GetQuantile() {
		s.metricsEmitter.EmitGauge(
			loggregator.WithGaugeValue(name, float64(quantile.GetValue()), ""),
			loggregator.WithGaugeSourceInfo(sourceID, ""),
			loggregator.WithEnvelopeTags(tags),
			loggregator.WithEnvelopeTag("quantile", strconv.FormatFloat(quantile.GetQuantile(), 'g', -1, 64)),
		)
	}
}

func parseTags(defaultSourceID string, m *io_prometheus_client.Metric) (string, map[string]string) {
	labels := make(map[string]string)
	sourceID := defaultSourceID
	for _, l := range m.GetLabel() {
		if l.GetName() == "source_id" {
			sourceID = l.GetValue()
			continue
		}
		labels[l.GetName()] = l.GetValue()
	}
	return sourceID, labels
}

type defaultGauge struct{}

func (g *defaultGauge) Set(float642 float64) {}
func (g *defaultGauge) Add(float642 float64) {}
