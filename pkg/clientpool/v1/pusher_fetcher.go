package v1

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"context"
	"fmt"
	"io"
	"log"

	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"

	"google.golang.org/grpc"
)

type MetricClient interface {
	NewGauge(name string, opts ...metrics.MetricOption) (metrics.Gauge, error)
}

type PusherFetcher struct {
	opts               []grpc.DialOption
	dopplerConnections metrics.Gauge
	dopplerV1Streams   metrics.Gauge
}

func NewPusherFetcher(mc MetricClient, opts ...grpc.DialOption) *PusherFetcher {
	dopplerConnections, _ := mc.NewGauge("DopplerConnections")
	dopplerV1Streams, _ := mc.NewGauge("DopplerV1Streams")
	return &PusherFetcher{
		opts:               opts,
		dopplerConnections: dopplerConnections,
		dopplerV1Streams:   dopplerV1Streams,
	}
}

func (p *PusherFetcher) Fetch(addr string) (io.Closer, plumbing.DopplerIngestor_PusherClient, error) {
	conn, err := grpc.Dial(addr, p.opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing ingestor stream to %s: %s", addr, err)
	}
	p.dopplerConnections.Add(1)

	client := plumbing.NewDopplerIngestorClient(conn)

	pusher, err := client.Pusher(context.Background())
	if err != nil {
		p.dopplerConnections.Add(-1)
		conn.Close()
		return nil, nil, fmt.Errorf("error establishing ingestor stream to %s: %s", addr, err)
	}
	p.dopplerV1Streams.Add(1)

	log.Printf("successfully established a stream to doppler %s", addr)

	closer := &decrementingCloser{
		closer:             conn,
		dopplerConnections: p.dopplerConnections,
		dopplerV1Streams:   p.dopplerV1Streams,
	}
	return closer, pusher, err
}

type decrementingCloser struct {
	closer             io.Closer
	dopplerConnections metrics.Gauge
	dopplerV1Streams   metrics.Gauge
}

func (d *decrementingCloser) Close() error {
	d.dopplerConnections.Add(-1)
	d.dopplerV1Streams.Add(-1)

	return d.closer.Close()
}
