package v2

import (
	"context"
	"fmt"
	"io"
	"log"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"google.golang.org/grpc"
)

type MetricClient interface {
	NewSumGauge(string) func(float64)
}

type SenderFetcher struct {
	opts               []grpc.DialOption
	dopplerConnections func(float64)
	dopplerV2Streams   func(float64)
}

func NewSenderFetcher(mc MetricClient, opts ...grpc.DialOption) *SenderFetcher {
	return &SenderFetcher{
		opts:               opts,
		dopplerConnections: mc.NewSumGauge("DopplerConnections"),
		dopplerV2Streams:   mc.NewSumGauge("DopplerV2Streams"),
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

	p.dopplerConnections(1)
	p.dopplerV2Streams(1)

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
	dopplerConnections func(float64)
	dopplerV2Streams   func(float64)
}

func (d *decrementingCloser) Close() error {
	d.dopplerConnections(-1)
	d.dopplerV2Streams(-1)

	return d.closer.Close()
}
