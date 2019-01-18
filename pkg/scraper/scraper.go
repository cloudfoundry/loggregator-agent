package scraper

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"github.com/wfernandes/app-metrics-plugin/pkg/parser"
)

type Scraper struct {
	sourceID            string
	addrProvider        func() []string
	metricsEgressClient MetricsEgressClient
	systemMetricsClient MetricsGetter
}

type MetricsEgressClient interface {
	EmitGauge(opts ...loggregator.EmitGaugeOption)
}

type MetricsGetter interface {
	Get(addr string) (*http.Response, error)
}

func New(sourceID string, addrProvider func() []string, c MetricsEgressClient, sc MetricsGetter) *Scraper {
	return &Scraper{
		sourceID:            sourceID,
		addrProvider:        addrProvider,
		metricsEgressClient: c,
		systemMetricsClient: sc,
	}
}

func (s *Scraper) Scrape() error {
	var errs []string
	for _, addr := range s.addrProvider() {
		err := s.scrape(addr)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ","))
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

			s.metricsEgressClient.EmitGauge(
				loggregator.WithGaugeSourceInfo(sourceID, ""),
				loggregator.WithGaugeValue(f.Name, v, ""),
				loggregator.WithEnvelopeTags(mm.Labels),
			)
		}
	}

	return err
}
