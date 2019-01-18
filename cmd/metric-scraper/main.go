package main

import (
	"log"
	"os"

	"code.cloudfoundry.org/loggregator-agent/cmd/metric-scraper/app"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Printf("starting Metrics Scraper...")
	defer log.Printf("closing Metrics Scraper...")

	cfg := app.LoadConfig(log)
	app.NewMetricScraper(cfg, log).Run()
}
