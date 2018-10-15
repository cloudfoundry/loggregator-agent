package v2

import (
	"strconv"
	"time"

	"code.cloudfoundry.org/go-loggregator/pulseemitter"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing/batching"
)

type TryNexter interface {
	TryNext() (*loggregator_v2.Envelope, bool)
}

type Writer interface {
	Write(msgs []*loggregator_v2.Envelope) error
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounterMetric(name string, opts ...pulseemitter.MetricOption) pulseemitter.CounterMetric
}

type Transponder struct {
	nexter        TryNexter
	writer        Writer
	tags          map[string]string
	batcher       *batching.V2EnvelopeBatcher
	batchSize     int
	batchInterval time.Duration
	droppedMetric pulseemitter.CounterMetric
	egressMetric  pulseemitter.CounterMetric
}

func NewTransponder(
	n TryNexter,
	w Writer,
	tags map[string]string,
	batchSize int,
	batchInterval time.Duration,
	metricClient MetricClient,
) *Transponder {
	droppedMetric := metricClient.NewCounterMetric("dropped",
		pulseemitter.WithVersion(2, 0),
		pulseemitter.WithTags(map[string]string{"direction": "egress"}),
	)

	egressMetric := metricClient.NewCounterMetric("egress",
		pulseemitter.WithVersion(2, 0),
	)

	return &Transponder{
		nexter:        n,
		writer:        w,
		tags:          tags,
		droppedMetric: droppedMetric,
		egressMetric:  egressMetric,
		batchSize:     batchSize,
		batchInterval: batchInterval,
	}
}

func (t *Transponder) Start() {^
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
	for _, e := range batch {
		t.addTags(e)
	}

	if err := t.writer.Write(batch); err != nil {
		// metric-documentation-v2: (loggregator.metron.dropped) Number of messages
		// dropped when failing to write to Dopplers v2 API
		t.droppedMetric.Increment(uint64(len(batch)))
		return
	}

	// metric-documentation-v2: (loggregator.metron.egress)
	// Number of messages written to Doppler's v2 API
	t.egressMetric.Increment(uint64(len(batch)))
}

func (t *Transponder) addTags(e *loggregator_v2.Envelope) {
	if e.Tags == nil {
		e.Tags = make(map[string]string)
	}

	// Move deprecated tags to tags.
	for k, v := range e.GetDeprecatedTags() {
		switch v.Data.(type) {
		case *loggregator_v2.Value_Text:
			e.Tags[k] = v.GetText()
		case *loggregator_v2.Value_Integer:
			e.Tags[k] = strconv.FormatInt(v.GetInteger(), 10)
		case *loggregator_v2.Value_Decimal:
			e.Tags[k] = strconv.FormatFloat(v.GetDecimal(), 'f', -1, 64)
		default:
			e.Tags[k] = v.String()
		}
	}

	for k, v := range t.tags {
		if _, ok := e.Tags[k]; !ok {
			e.Tags[k] = v
		}
	}

	e.DeprecatedTags = nil
}
