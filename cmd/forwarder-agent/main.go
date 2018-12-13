package main

import (
	"expvar"
	"log"
	"os"

	"code.cloudfoundry.org/loggregator-agent/cmd/forwarder-agent/app"
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Println("starting forwarder-agent")
	defer log.Println("stopping forwarder-agent")

	cfg := app.LoadConfig()
	metrics := metrics.New(expvar.NewMap("ForwarderAgent"))

	app.NewForwarderAgent(
		cfg,
		metrics,
		log,
	).Run()
}
