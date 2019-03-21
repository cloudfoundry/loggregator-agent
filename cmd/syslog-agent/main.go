package main

import (
	"log"
	"os"

	"code.cloudfoundry.org/loggregator-agent/cmd/syslog-agent/app"
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Println("starting syslog-agent")
	defer log.Println("stopping syslog-agent")

	cfg := app.LoadConfig()

	dt := map[string]string {
		"metrics_version": "2.0",
	}
	m := metrics.NewPromRegistry("syslog_agent", int(cfg.DebugPort), log, metrics.WithDefaultTags(dt))

	app.NewSyslogAgent(cfg, m, log).Run()
}
