package main

import (
	"log"
	"os"

	"code.cloudfoundry.org/loggregator-agent/cmd/metric-scraper/app"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Printf("starting MetricClient Scraper...")
	defer log.Printf("closing MetricClient Scraper...")

	cfg := app.LoadConfig(log)
	app.NewMetricScraper(cfg, log).Run()
}
