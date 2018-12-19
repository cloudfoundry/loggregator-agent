package v2

import (
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing/batching"
)

type Nexter interface {
	TryNext() (*loggregator_v2.Envelope, bool)
}

type BatchWriter interface {
	Write(msgs []*loggregator_v2.Envelope) error
}

// MetricClient creates new CounterMetrics.
type MetricClient interface {
	NewCounter(name string) func(uint64)
}

type Transponder struct {
	nexter        Nexter
	writer        BatchWriter
	batcher       *batching.V2EnvelopeBatcher
	batchSize     int
	batchInterval time.Duration
	droppedMetric func(uint64)
	egressMetric  func(uint64)
}

func NewTransponder(
	n Nexter,
	w BatchWriter,
	batchSize int,
	batchInterval time.Duration,
	metricClient MetricClient,
) *Transponder {
	return &Transponder{
		nexter:        n,
		writer:        w,
		droppedMetric: metricClient.NewCounter("DroppedEgressV2"),
		egressMetric:  metricClient.NewCounter("EgressV2"),
		batchSize:     batchSize,
		batchInterval: batchInterval,
	}
}

func (t *Transponder) Start() {
	b := batching.NewV2EnvelopeBatcher(
		t.batchSize,
		t.batchInterval,
		batching.V2EnvelopeWriterFunc(t.write),
	)

	for {
		envelope, ok := t.nexter.TryNext()
		if !ok {
			b.Flush()
			time.Sleep(100 * time.Millisecond)
			continue
		}

		b.Write(envelope)
	}
}

func (t *Transponder) write(batch []*loggregator_v2.Envelope) {
	if err := t.writer.Write(batch); err != nil {
		// metric-documentation-v2: (loggregator.metron.dropped) Number of messages
		// dropped when failing to write to Dopplers v2 API
		t.droppedMetric(uint64(len(batch)))
		return
	}

	// metric-documentation-v2: (loggregator.metron.egress)
	// Number of messages written to Doppler's v2 API
	t.egressMetric(uint64(len(batch)))
}
