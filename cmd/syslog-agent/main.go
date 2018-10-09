package main

import (
	"log"
	"os"

	"code.cloudfoundry.org/loggregator-agent/cmd/syslog-agent/app"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Println("starting syslog-agent")
	defer log.Println("stopping syslog-agent")

	cfg := app.LoadConfig()

	app.NewSyslogAgent(cfg).Run(true)
}
