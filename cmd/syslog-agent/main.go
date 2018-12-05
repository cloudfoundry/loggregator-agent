package main

import (
	"expvar"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/loggregator-agent/cmd/syslog-agent/app"
	"code.cloudfoundry.org/loggregator-agent/pkg/binding"
	"code.cloudfoundry.org/loggregator-agent/pkg/cache"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"code.cloudfoundry.org/loggregator-agent/pkg/timeoutwaitgroup"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Println("starting syslog-agent")
	defer log.Println("stopping syslog-agent")

	m := metrics.New(expvar.NewMap("SyslogAgent"))

	cfg := app.LoadConfig()

	connector := syslog.NewSyslogConnector(
		syslog.NetworkTimeoutConfig{
			Keepalive:    10 * time.Second,
			DialTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		cfg.DrainSkipCertVerify,
		timeoutwaitgroup.New(time.Minute),
		syslog.NewWriterFactory(m),
		m,
	)

	cacheClient := cache.NewClient(cfg.CacheURL, newTLSHTTPClient(cfg))
	bindingManager := binding.NewManager(
		cups.NewBindingFetcher(cfg.BindingsPerAppLimit, cacheClient, m),
		connector,
		m,
		cfg.CachePollingInterval,
		log,
	)

	app.NewSyslogAgent(
		cfg.DebugPort,
		m,
		bindingManager,
		cfg.GRPC,
		log,
	).Run()
}

//TODO:// We do this twice, make a helper
func newTLSHTTPClient(cfg app.Config) *http.Client {
	//TODO: We have several ways to create TSL configs, which is correct?
	tlsConfig, err := plumbing.NewClientMutualTLSConfig(
		cfg.CacheCertFile,
		cfg.CacheKeyFile,
		cfg.CacheCAFile,
		cfg.CacheCommonName,
	)
	if err != nil {
		log.Panicf("failed to load API client certificates: %s", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
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
	}
}
