package v2

import (
	"context"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"google.golang.org/grpc"
)

type Nexter interface {
	Next() *loggregator_v2.Envelope
}

type EnvelopeForwarder struct {
	addr        string
	buffer      Nexter
	dialOptions []grpc.DialOption
	batchSize   int
}

func NewEnvelopeForwarder(
	addr string,
	buffer Nexter,
	opts ...ForwarderOption,
) *EnvelopeForwarder {
	f := &EnvelopeForwarder{
		addr:   addr,
		buffer: buffer,
	}

	for _, o := range opts {
		o(f)
	}

	return f
}

func (f *EnvelopeForwarder) Run(blocking bool) {
	if blocking {
		f.run()
		return
	}

	go f.run()
}

func (f *EnvelopeForwarder) run() {
	b := batching.NewV2EnvelopeBatcher(
		t.batchSize,
		t.batchInterval,
		batching.V2EnvelopeWriterFunc(t.write),
	)

	conn, err := grpc.Dial(f.addr, f.dialOptions...)
	if err != nil {
		panic(err) // TODO: Don't panic!
	}

	client := loggregator_v2.NewIngressClient(conn)
	stream, err := client.BatchSender(context.Background())
	if err != nil {
		panic(err) // TODO: Don't panic!
	}

	for {
		env := f.buffer.Next()

		stream.Send(&loggregator_v2.EnvelopeBatch{
			Batch: []*loggregator_v2.Envelope{env},
		})
	}
}

func (f *EnvelopeForwarder) write(batch []*loggregator_v2.Envelope) {
	if err := f.writer.Write(batch); err != nil {
		// metric-documentation-v2: (loggregator.metron.dropped) Number of messages
		// dropped when failing to write to Dopplers v2 API
		f.droppedMetric.Increment(uint64(len(batch)))
		return
	}

	// metric-documentation-v2: (loggregator.metron.egress)
	// Number of messages written to Doppler's v2 API
	// f.egressMetric.Increment(uint64(len(batch)))
}

type ForwarderOption func(*EnvelopeForwarder)

func WithEnvelopeForwarderBatchSize(size int) ForwarderOption {
	return func(f *EnvelopeForwarder) {
		f.batchSize = size
	}
}

func WithEnvelopeForwarderDialOptions(dialOptions ...grpc.DialOption) ForwarderOption {
	return func(f *EnvelopeForwarder) {
		f.dialOptions = dialOptions
	}
}
