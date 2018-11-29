package main

import (
	"expvar"
	"log"
	"os"
	"time"

	"code.cloudfoundry.org/loggregator-agent/cmd/syslog-agent/app"
	"code.cloudfoundry.org/loggregator-agent/pkg/binding"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/api"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"code.cloudfoundry.org/loggregator-agent/pkg/timeoutwaitgroup"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Println("starting syslog-agent")
	defer log.Println("stopping syslog-agent")

	cfg := app.LoadConfig()

	apiTLSConfig, err := api.NewMutualTLSConfig(
		cfg.APICertFile,
		cfg.APIKeyFile,
		cfg.APICAFile,
		cfg.APICommonName,
	)
	if err != nil {
		log.Fatalf("Invalid TLS config: %s", err)
	}

	apiClient := api.Client{
		Client:    api.NewHTTPSClient(apiTLSConfig, 5*time.Second),
		Addr:      cfg.APIURL,
		BatchSize: 1000,
	}

	m := metrics.New(expvar.NewMap("SyslogAgent"))

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

	bindingManager := binding.NewManager(
		cups.NewBindingFetcher(cfg.BindingPerAppLimit, apiClient, m),
		connector,
		m,
		cfg.APIPollingInterval,
		log,
	)

	app.NewSyslogAgent(
		cfg.DebugPort,
		m,
		bindingManager,
		cfg.GRPC,
		log,
	).Run(true)
}
