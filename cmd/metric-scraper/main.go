package main

import (
	"log"
	"net/http"
	"os"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/loggregator-agent/pkg/scraper"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Printf("starting Metrics Scraper...")
	defer log.Printf("closing Metrics Scraper...")

	cfg := loadConfig(log)

	creds, err := loggregator.NewIngressTLSConfig(
		cfg.CACertPath,
		cfg.ClientCertPath,
		cfg.ClientKeyPath,
	)
	if err != nil {
		log.Fatal(err)
	}

	client, err := loggregator.NewIngressClient(
		creds,
		loggregator.WithAddr(cfg.LoggregatorIngressAddr),
		loggregator.WithLogger(log),
	)
	if err != nil {
		log.Fatal(err)
	}

	var scrapers []*scraper.Scraper
	scrapers = append(scrapers, scraper.New(
		cfg.SourceID,
		"http://127.0.0.1:"+cfg.ScrapePort+"/metrics",
		client,
		http.DefaultClient,
	))

	for range time.Tick(cfg.ScrapeInterval) {
		for _, scraper := range scrapers {
			if err := scraper.Scrape(); err != nil {
				log.Printf("failed to scrape: %s", err)
			}
		}
	}
}
