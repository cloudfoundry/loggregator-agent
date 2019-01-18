package app

import (
	"log"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"code.cloudfoundry.org/loggregator-agent/pkg/scraper"
)

type MetricScraper struct {
	cfg         Config
	log         *log.Logger
	urlProvider func() []string
	doneChan    chan struct{}
}

func NewMetricScraper(cfg Config, l *log.Logger) *MetricScraper {
	return &MetricScraper{
		cfg:         cfg,
		log:         l,
		urlProvider: scraper.NewDNSMetricUrlProvider(cfg.DNSFile, cfg.ScrapePort),
		doneChan:    make(chan struct{}),
	}
}

func (m *MetricScraper) Run() {
	creds, err := loggregator.NewIngressTLSConfig(
		m.cfg.CACertPath,
		m.cfg.ClientCertPath,
		m.cfg.ClientKeyPath,
	)
	if err != nil {
		m.log.Fatal(err)
	}

	client, err := loggregator.NewIngressClient(
		creds,
		loggregator.WithAddr(m.cfg.LoggregatorIngressAddr),
		loggregator.WithLogger(m.log),
	)
	if err != nil {
		m.log.Fatal(err)
	}

	systemMetricsClient := plumbing.NewTLSHTTPClient(
		cfg.MetricsCertPath,
		cfg.MetricsKeyPath,
		cfg.MetricsCACertPath,
		cfg.MetricsCN,
	)

	s := scraper.New(
		m.cfg.DefaultSourceID,
		m.urlProvider,
		client,
		systemMetricsClient,
	)

	t := time.NewTicker(m.cfg.ScrapeInterval)
	for {
		select {
		case <-t.C:
			if err := s.Scrape(); err != nil {
				m.log.Printf("failed to scrape: %s", err)
			}
		case <-m.doneChan:
			return
		}
	}
}

func (m *MetricScraper) Stop() {
	close(m.doneChan)
}
