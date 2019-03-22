package v2

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"log"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"golang.org/x/net/context"
)

type DataSetter interface {
	Set(e *loggregator_v2.Envelope)
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, ots ...metrics.MetricOption) (metrics.Counter, error)
}

type Receiver struct {
	dataSetter           DataSetter
	ingressMetric        metrics.Counter
	originMappingsMetric metrics.Counter
}

func NewReceiver(dataSetter DataSetter, metricClient MetricClient, opts ...metrics.MetricOption) *Receiver {
	ingressMetric, _ := metricClient.NewCounter("ingress", opts...)
	//TODO:
	//if err != nil {
	//	panic(err)
	//}

	originMappingsMetric, _ := metricClient.NewCounter("origin_mappings", opts...)
	//TODO:
	//if err != nil {
	//	panic(err)
	//}

	return &Receiver{
		dataSetter:           dataSetter,
		ingressMetric:        ingressMetric,
		originMappingsMetric: originMappingsMetric,
	}
}

func (s *Receiver) Sender(sender loggregator_v2.Ingress_SenderServer) error {
	for {
		e, err := sender.Recv()
		if err != nil {
			log.Printf("Failed to receive data: %s", err)
			return err
		}
		e.SourceId = s.sourceID(e)
		s.dataSetter.Set(e)
		s.ingressMetric.Add(1)
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
			e.SourceId = s.sourceID(e)
			s.dataSetter.Set(e)
		}
		s.ingressMetric.Add(float64(len(envelopes.Batch)))
	}

	return nil
}

func (s *Receiver) Send(_ context.Context, b *loggregator_v2.EnvelopeBatch) (*loggregator_v2.SendResponse, error) {
	for _, e := range b.Batch {
		e.SourceId = s.sourceID(e)
		s.dataSetter.Set(e)
	}

	s.ingressMetric.Add(float64(len(b.Batch)))

	return &loggregator_v2.SendResponse{}, nil
}

func (r *Receiver) sourceID(e *loggregator_v2.Envelope) string {
	if e.SourceId != "" {
		return e.SourceId
	}

	if id, ok := e.GetTags()["origin"]; ok {
		r.originMappingsMetric.Add(1)
		return id
	}

	if id, ok := e.GetDeprecatedTags()["origin"]; ok {
		r.originMappingsMetric.Add(1)
		return id.GetText()
	}

	return ""
}
