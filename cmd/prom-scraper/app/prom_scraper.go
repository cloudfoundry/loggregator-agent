package app

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/loggregator-agent/pkg/scraper"
	"gopkg.in/yaml.v2"
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

	scrapeTargetProvider := func() []scraper.Target {
		return scrapeTargetsFromFiles(p.cfg.MetricPortCfg, p.log)
	}

	s := scraper.New(
		scrapeTargetProvider,
		client,
		http.DefaultClient,
	)

	for range time.Tick(p.cfg.ScrapeInterval) {
		if err := s.Scrape(); err != nil {
			p.log.Printf("failed to scrape: %s", err)
		}
	}
}

func scrapeTargetsFromFiles(glob string, l *log.Logger) []scraper.Target {
	files, err := filepath.Glob(glob)
	if err != nil {
		l.Fatal("Unable to read metric port location")
	}

	var targets []scraper.Target

	for _, f := range files {
		yamlFile, err := ioutil.ReadFile(f)
		if err != nil {
			l.Fatalf("cannot read file: %s", err)
		}

		var c struct{
			Port string `yaml:"port"`
			SourceID string `yaml:"source_id"`
			InstanceID string `yaml:"instance_id"`
		}
		err = yaml.Unmarshal(yamlFile, &c)
		if err != nil {
			l.Fatalf("Unmarshal: %v", err)
		}

		target := scraper.Target{
			ID: c.SourceID,
			InstanceID: c.InstanceID,
			MetricURL: fmt.Sprintf("http://127.0.0.1:%s/metrics", c.Port),
		}

		targets = append(targets, target)
	}

	return targets
}
