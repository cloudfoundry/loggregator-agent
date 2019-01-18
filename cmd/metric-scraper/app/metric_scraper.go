package app

import (
	"log"
	"net/http"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/loggregator-agent/pkg/scraper"
)

type MetricScraper struct {
	cfg         Config
	log         *log.Logger
	urlProvider func() []string
}

func NewMetricScraper(cfg Config, l *log.Logger) *MetricScraper {
	return &MetricScraper{
		cfg:         cfg,
		log:         l,
		urlProvider: scraper.NewDNSMetricUrlProvider(cfg.DNSFile, cfg.ScrapePort),
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

	s := scraper.New(
		m.cfg.DefaultSourceID,
		m.urlProvider,
		client,
		http.DefaultClient,
	)

	for range time.Tick(m.cfg.ScrapeInterval) {
		if err := s.Scrape(); err != nil {
			m.log.Printf("failed to scrape: %s", err)
		}
	}
}
