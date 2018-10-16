package main

import (
	"log"
	_ "net/http/pprof"
	"os"

	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"google.golang.org/grpc"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Print("Starting ExpvarForwarder...")
	defer log.Print("Closing ExpvarForwarder.")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	opts := []metrics.ExpvarForwarderOption{
		metrics.WithExpvarLogger(log.New(os.Stderr, "", log.LstdFlags)),
		metrics.WithExpvarDialOpts(grpc.WithTransportCredentials(cfg.TLS.Credentials())),
		metrics.WithExpvarDefaultSourceId(cfg.DefaultSourceId),
	}

	for _, c := range cfg.Counters.Descriptions {
		opts = append(opts, metrics.AddExpvarCounterTemplate(
			c.Addr,
			c.Name,
			c.SourceId,
			c.Template,
			c.Tags,
		))
	}

	for _, g := range cfg.Gauges.Descriptions {
		opts = append(opts, metrics.AddExpvarGaugeTemplate(
			g.Addr,
			g.Name,
			g.Unit,
			g.SourceId,
			g.Template,
			g.Tags,
		))
	}

	forwarder := metrics.NewExpvarForwarder(
		cfg.LoggregatorAddr,
		opts...,
	)

	forwarder.Start()
}
