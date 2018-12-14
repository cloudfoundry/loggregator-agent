package main

import (
	"expvar"
	"log"
	"os"

	"code.cloudfoundry.org/loggregator-agent/cmd/syslog-agent/app"
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Println("starting syslog-agent")
	defer log.Println("stopping syslog-agent")

	m := metrics.New(expvar.NewMap("SyslogAgent"))

	cfg := app.LoadConfig()

	app.NewSyslogAgent(cfg, m, log).Run()
}
