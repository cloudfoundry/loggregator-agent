package main

import (
	"expvar"
	"log"
	"os"
	"time"

	"code.cloudfoundry.org/loggregator-agent/cmd/syslog-agent/app"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/api"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
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

	metrics := metrics.New(expvar.NewMap("SyslogAgent"))
	bf := cups.NewBindingFetcher(cfg.BindingPerAppLimit, apiClient, metrics)

	app.NewSyslogAgent(
		cfg.DebugPort,
		metrics,
		bf,
		cfg.APIPollingInterval,
		cfg.GRPC,
		cfg.DrainSkipCertVerify,
		log,
	).Run(true)
}
