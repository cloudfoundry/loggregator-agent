package main

import (
	"expvar"
	"log"
	"os"
	"time"

	"code.cloudfoundry.org/loggregator-agent/cmd/forwarder-agent/app"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/api"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Println("starting forwarder-agent")
	defer log.Println("stopping forwarder-agent")

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
	apiTLSConfig.InsecureSkipVerify = cfg.APISkipCertVerify

	apiClient := api.Client{
		Client:    api.NewHTTPSClient(apiTLSConfig, 5*time.Second),
		Addr:      cfg.APIURL,
		BatchSize: 1000,
	}

	bf := cups.NewBindingFetcher(apiClient)

	app.NewForwarderAgent(
		cfg.DebugPort,
		metrics.New(expvar.NewMap("ForwarderAgent")),
		bf,
		cfg.APIPollingInterval,
	).Run(true)
}
