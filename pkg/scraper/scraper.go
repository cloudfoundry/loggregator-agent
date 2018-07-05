package scraper

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"github.com/wfernandes/app-metrics-plugin/pkg/parser"
)

type Scraper struct {
	doer         Doer
	metricClient MetricClient
	sourceID     string
	addr         string
}

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type MetricClient interface {
	EmitGauge(opts ...loggregator.EmitGaugeOption)
}

func New(sourceID, addr string, c MetricClient, d Doer) *Scraper {
	return &Scraper{
		doer:         d,
		sourceID:     sourceID,
		addr:         addr,
		metricClient: c,
	}
}

func (s *Scraper) Scrape() error {
	req, err := http.NewRequest(http.MethodGet, s.addr, nil)
	if err != nil {
		return err
	}

	resp, err := s.doer.Do(req)
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

			s.metricClient.EmitGauge(
				loggregator.WithGaugeSourceInfo(s.sourceID, ""),
				loggregator.WithGaugeValue(f.Name, v, ""),
				loggregator.WithEnvelopeTags(mm.Labels),
			)
		}
	}

	return nil
}
