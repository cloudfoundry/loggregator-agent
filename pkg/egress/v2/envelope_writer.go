package v2

import "code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"

type Writer interface {
	Write(*loggregator_v2.Envelope) error
}

type EnvelopeProcessor interface {
	Process(*loggregator_v2.Envelope) error
}

type EnvelopeWriter struct {
	writer     Writer
	processors []EnvelopeProcessor
}

func NewEnvelopeWriter(w Writer, ps ...EnvelopeProcessor) EnvelopeWriter {
	return EnvelopeWriter{
		writer:     w,
		processors: ps,
	}
}

func (ew EnvelopeWriter) Write(env *loggregator_v2.Envelope) error {
	for _, processor := range ew.processors {
		processor.Process(env)
	}

	return ew.writer.Write(env)
}
