package main

import (
	"expvar"
	"log"
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

	tlsClient := plumbing.NewTLSHTTPClient(
		cfg.CacheCertFile,
		cfg.CacheKeyFile,
		cfg.CacheCAFile,
		cfg.CacheCommonName,
	)
	cacheClient := cache.NewClient(cfg.CacheURL, tlsClient)
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
