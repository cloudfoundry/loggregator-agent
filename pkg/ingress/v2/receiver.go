package v2

import (
	"log"

	"code.cloudfoundry.org/go-loggregator/pulseemitter"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"golang.org/x/net/context"
)

type DataSetter interface {
	Set(e *loggregator_v2.Envelope)
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounterMetric(name string, opts ...pulseemitter.MetricOption) pulseemitter.CounterMetric
}

type Receiver struct {
	dataSetter    DataSetter
	ingressMetric pulseemitter.CounterMetric
}

func NewReceiver(dataSetter DataSetter, metricClient MetricClient) *Receiver {
	// metric-documentation-v2: (loggregator.metron.ingress) The number of
	// received messages over Metrons V2 gRPC API.
	ingressMetric := metricClient.NewCounterMetric("ingress",
		pulseemitter.WithVersion(2, 0),
	)

	return &Receiver{
		dataSetter:    dataSetter,
		ingressMetric: ingressMetric,
	}
}

func (s *Receiver) Sender(sender loggregator_v2.Ingress_SenderServer) error {
	for {
		e, err := sender.Recv()
		if err != nil {
			log.Printf("Failed to receive data: %s", err)
			return err
		}

		e.SourceId = sourceID(e)
		s.dataSetter.Set(e)
		s.ingressMetric.Increment(1)
	}

	return nil
}

func (s *Receiver) BatchSender(sender loggregator_v2.Ingress_BatchSenderServer) error {
	for {
		envelopes, err := sender.Recv()
		if err != nil {
			log.Printf("Failed to receive data: %s", err)
			return err
		}

		for _, e := range envelopes.Batch {
			e.SourceId = sourceID(e)
			s.dataSetter.Set(e)
		}
		s.ingressMetric.Increment(uint64(len(envelopes.Batch)))
	}

	return nil
}

func (s *Receiver) Send(_ context.Context, b *loggregator_v2.EnvelopeBatch) (*loggregator_v2.SendResponse, error) {
	for _, e := range b.Batch {
		e.SourceId = sourceID(e)
		s.dataSetter.Set(e)
	}

	s.ingressMetric.Increment(uint64(len(b.Batch)))

	return &loggregator_v2.SendResponse{}, nil
}

func sourceID(e *loggregator_v2.Envelope) string {
	if e.SourceId != "" {
		return e.SourceId
	}

	if id, ok := e.GetTags()["origin"]; ok {
		return id
	}

	if id, ok := e.GetDeprecatedTags()["origin"]; ok {
		return id.GetText()
	}

	return ""
}
