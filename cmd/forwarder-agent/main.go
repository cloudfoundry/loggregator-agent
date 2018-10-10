package main

import (
	"log"
	"os"

	"code.cloudfoundry.org/loggregator-agent/cmd/forwarder-agent/app"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Println("starting forwarder-agent")
	defer log.Println("stopping forwarder-agent")

	cfg := app.LoadConfig()

	app.NewForwarderAgent(cfg).Run(true)
}
