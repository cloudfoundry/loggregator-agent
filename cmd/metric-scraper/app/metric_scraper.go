package app

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"log"
	"net"
	"net/http"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"code.cloudfoundry.org/loggregator-agent/pkg/scraper"
)

type MetricScraper struct {
	cfg         Config
	log         *log.Logger
	urlProvider func() []string
	doneChan    chan struct{}
	stoppedChan chan struct{}
	metrics     metricsClient
}

type metricsClient interface {
	NewCounter(name string, opts ...metrics.MetricOption) metrics.Counter
	NewGauge(name string, opts ...metrics.MetricOption) metrics.Gauge
}

func NewMetricScraper(cfg Config, l *log.Logger, m metricsClient) *MetricScraper {
	return &MetricScraper{
		cfg:         cfg,
		log:         l,
		urlProvider: scraper.NewDNSMetricUrlProvider(cfg.DNSFile, cfg.ScrapePort),
		doneChan:    make(chan struct{}),
		metrics:     m,
		stoppedChan: make(chan struct{}),
	}
}

func (m *MetricScraper) Run() {
	m.scrape()
}

func (m *MetricScraper) scrape() {
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
		newTLSClient(m.cfg),
	)

	attemptedScrapes := m.metrics.NewCounter("last_total_attempted_scrapes")
	failedScrapes := m.metrics.NewCounter("last_total_failed_scrapes")

	leadershipClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	// TODO: maybe change this to be scrape_cycles?
	numScrapes := m.metrics.NewCounter("num_scrapes")
	scrapeDuration := m.metrics.NewGauge("last_total_scrape_duration")
	t := time.NewTicker(m.cfg.ScrapeInterval)
	for {
		select {
		case <-t.C:
			resp, err := leadershipClient.Get(m.cfg.LeadershipServerAddr)
			if err == nil && resp.StatusCode == http.StatusLocked {
				continue
			}

			// Count the number of URLs, which represents the number of VMs
			attemptedScrapes.Add(float64(len(m.urlProvider())))
			start := time.Now()
			if err := s.Scrape(); err != nil {
				failedScrapes.Add(1.0)
				m.log.Printf("failed to scrape: %s", err)
			}
			end := time.Since(start)
			scrapeDuration.Set(float64(end.Nanoseconds()))
			numScrapes.Add(1.0)
		case <-m.doneChan:
			close(m.stoppedChan)
			return
		}
	}
}

func (m *MetricScraper) Stop() {
	close(m.doneChan)
	<-m.stoppedChan
}

func newTLSClient(cfg Config) *http.Client {
	tlsConfig, err := plumbing.NewClientMutualTLSConfig(
		cfg.MetricsCertPath,
		cfg.MetricsKeyPath,
		cfg.MetricsCACertPath,
		cfg.MetricsCN,
	)
	if err != nil {
		log.Panicf("failed to load API client certificates: %s", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.ScrapeTimeout,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       tlsConfig,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cfg.ScrapeTimeout,
	}
}
