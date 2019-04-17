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

	downstreamAddrProvider := func() []string {
		return getDownstreamAddresses(p.cfg.DebugPortCfg, p.log)
	}

	s := scraper.New(
		p.cfg.DefaultSourceID,
		downstreamAddrProvider,
		client,
		http.DefaultClient,
	)

	for range time.Tick(p.cfg.ScrapeInterval) {
		if err := s.Scrape(); err != nil {
			p.log.Printf("failed to scrape: %s", err)
		}
	}
}

type portConfig struct {
	Debug string `yaml:"debug"`
}

func getDownstreamAddresses(glob string, l *log.Logger) []string {
	files, err := filepath.Glob(glob)
	if err != nil {
		l.Fatal("Unable to read downstream port location")
	}

	var addrs []string
	for _, f := range files {
		yamlFile, err := ioutil.ReadFile(f)
		if err != nil {
			l.Fatalf("cannot read file: %s", err)
		}

		var c portConfig
		err = yaml.Unmarshal(yamlFile, &c)
		if err != nil {
			l.Fatalf("Unmarshal: %v", err)
		}

		addrs = append(addrs, fmt.Sprintf("http://127.0.0.1:%s/metrics", c.Debug))
	}

	return addrs
}