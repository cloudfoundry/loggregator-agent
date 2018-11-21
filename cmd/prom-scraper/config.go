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
	SourceID               string        `env:"SOURCE_ID, report, required"`
	MetricsURL             []*url.URL    `env:"METRICS_URL, report, required"`
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
