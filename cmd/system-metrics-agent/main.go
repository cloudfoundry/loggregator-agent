package main

import (
	"log"
	"os"

	"code.cloudfoundry.org/loggregator-agent/cmd/system-metrics-agent/app"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Println("starting system-metrics-agent")
	defer log.Println("stopping system-metrics-agent")

	cfg := app.LoadConfig()

	app.NewSystemMetricsAgent(cfg, log).Run(true)
}
