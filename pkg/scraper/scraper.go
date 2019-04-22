package scraper

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"github.com/wfernandes/app-metrics-plugin/pkg/parser"
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
			err := s.scrape(addr)

			if err != nil {
				errs <- err.Error()
			}
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

func (s *Scraper) scrape(addr string) error {
	resp, err := s.systemMetricsClient.Get(addr)
	if err != nil {
		return err
	}

	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, body)
	}

	p := parser.NewPrometheus()

	res, err := p.Parse(body)
	if err != nil {
		return err
	}

	for _, v := range res {
		f, ok := v.(*parser.Family)
		if !ok {
			continue
		}

		for _, m := range f.Metrics {
			mm, ok := m.(parser.Metric)
			if !ok {
				continue
			}
			v, err := strconv.ParseFloat(mm.Value, 64)
			if err != nil {
				continue
			}

			sourceID, ok := mm.Labels["source_id"]
			if !ok {
				sourceID = s.sourceID
			}
			delete(mm.Labels, "source_id")

			s.metricsEmitter.EmitGauge(
				loggregator.WithGaugeSourceInfo(sourceID, ""),
				loggregator.WithGaugeValue(f.Name, v, ""),
				loggregator.WithEnvelopeTags(mm.Labels),
			)
		}
	}

	return err
}

type defaultGauge struct{}

func (g *defaultGauge) Set(float642 float64) {}
func (g *defaultGauge) Add(float642 float64) {}
