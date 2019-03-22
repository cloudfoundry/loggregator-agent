package v2

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"context"
	"fmt"
	"io"
	"log"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"google.golang.org/grpc"
)

type MetricClient interface {
	NewGauge(name string, opts ...metrics.MetricOption) (metrics.Gauge, error)
}

type SenderFetcher struct {
	opts               []grpc.DialOption
	dopplerConnections metrics.Gauge
	dopplerV2Streams   metrics.Gauge
}

func NewSenderFetcher(mc MetricClient, opts ...grpc.DialOption) *SenderFetcher {
	//TODO: Err handling
	dopplerConnections, _ := mc.NewGauge("DopplerConnections")
	dopplerV2Streams, _ := mc.NewGauge("DopplerV2Streams")
	return &SenderFetcher{
		opts:               opts,
		dopplerConnections: dopplerConnections,
		dopplerV2Streams:   dopplerV2Streams,
	}
}

func (p *SenderFetcher) Fetch(addr string) (io.Closer, loggregator_v2.Ingress_BatchSenderClient, error) {
	conn, err := grpc.Dial(addr, p.opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing ingestor stream to %s: %s", addr, err)
	}

	client := loggregator_v2.NewIngressClient(conn)
	sender, err := client.BatchSender(context.Background())
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to establish stream to doppler (%s): %s", addr, err)
	}

	p.dopplerConnections.Set(1)
	p.dopplerV2Streams.Set(1)

	log.Printf("successfully established a stream to doppler %s", addr)

	closer := &decrementingCloser{
		closer:             conn,
		dopplerConnections: p.dopplerConnections,
		dopplerV2Streams:   p.dopplerV2Streams,
	}
	return closer, sender, err
}

type decrementingCloser struct {
	closer             io.Closer
	dopplerConnections metrics.Gauge
	dopplerV2Streams   metrics.Gauge
}

func (d *decrementingCloser) Close() error {
	d.dopplerConnections.Add(-1)
	d.dopplerV2Streams.Add(-1)

	return d.closer.Close()
}
