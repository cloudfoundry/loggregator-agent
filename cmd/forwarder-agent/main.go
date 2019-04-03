package main

import (
	"log"
	"os"

	"code.cloudfoundry.org/loggregator-agent/cmd/forwarder-agent/app"
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
)

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	logger.Println("starting forwarder-agent")
	defer logger.Println("stopping forwarder-agent")

	cfg := app.LoadConfig()
	dt := map[string]string {
		"metrics_version": "2.0",
	}

	metrics := metrics.NewPromRegistry("forwarder_agent", int(cfg.MetricsPort), logger, metrics.WithDefaultTags(dt))

	app.NewForwarderAgent(
		cfg,
		metrics,
		logger,
	).Run()
}
