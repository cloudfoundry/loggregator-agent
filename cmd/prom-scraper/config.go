package main

import (
	"log"
	"net/url"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
)

type config struct {
	// Loggregator Agent Certs
	ClientKeyPath  string `env:"CLIENT_KEY_PATH, report, required"`
	ClientCertPath string `env:"CLIENT_CERT_PATH, report, required"`
	CACertPath     string `env:"CA_CERT_PATH, report, required"`

	LoggregatorIngressAddr string        `env:"LOGGREGATOR_AGENT_ADDR, report, required"`
	DefaultSourceID        string        `env:"DEFAULT_SOURCE_ID, report, required"`
	MetricsUrls            []*url.URL    `env:"METRICS_URLS, report, required"`
	ScrapeInterval         time.Duration `env:"SCRAPE_INTERVAL, report"`
}

func loadConfig(log *log.Logger) config {
	cfg := config{
		ScrapeInterval: 15 * time.Second,
	}

	if err := envstruct.Load(&cfg); err != nil {
		log.Fatal(err)
	}

	envstruct.WriteReport(&cfg)

	return cfg
}
