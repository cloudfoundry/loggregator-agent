package app

import (
	"log"
	"net/http"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/loggregator-agent/pkg/scraper"
)

type PromScraper struct {
	cfg Config
	log *log.Logger
}

func NewPromScraper(cfg Config, log *log.Logger) *PromScraper {
	return &PromScraper{
		cfg: cfg,
		log: log,
	}
}

func (p *PromScraper) Run() {
	creds, err := loggregator.NewIngressTLSConfig(
		p.cfg.CACertPath,
		p.cfg.ClientCertPath,
		p.cfg.ClientKeyPath,
	)
	if err != nil {
		p.log.Fatal(err)
	}

	client, err := loggregator.NewIngressClient(
		creds,
		loggregator.WithAddr(p.cfg.LoggregatorIngressAddr),
		loggregator.WithLogger(p.log),
	)
	if err != nil {
		p.log.Fatal(err)
	}

	var URLs []string
	for _, u := range p.cfg.MetricsUrls {
		URLs = append(URLs, u.String())
	}

	s := scraper.New(
		p.cfg.DefaultSourceID,
		func() []string { return URLs },
		client,
		http.DefaultClient,
	)

	for range time.Tick(p.cfg.ScrapeInterval) {
		if err := s.Scrape(); err != nil {
			p.log.Printf("failed to scrape: %s", err)
		}
	}
}
