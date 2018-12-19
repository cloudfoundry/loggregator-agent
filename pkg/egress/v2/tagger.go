package v2

import (
	"strconv"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

type Tagger struct {
	tags map[string]string
}

func NewTagger(ts map[string]string) Tagger {
	return Tagger{
		tags: ts,
	}
}

func (t Tagger) Process(env *loggregator_v2.Envelope) error {
	if env.Tags == nil {
		env.Tags = make(map[string]string)
	}

	// Move deprecated tags to tags.
	for k, v := range env.GetDeprecatedTags() {
		switch v.Data.(type) {
		case *loggregator_v2.Value_Text:
			env.Tags[k] = v.GetText()
		case *loggregator_v2.Value_Integer:
			env.Tags[k] = strconv.FormatInt(v.GetInteger(), 10)
		case *loggregator_v2.Value_Decimal:
			env.Tags[k] = strconv.FormatFloat(v.GetDecimal(), 'f', -1, 64)
		default:
			env.Tags[k] = v.String()
		}
	}

	for k, v := range t.tags {
		if _, ok := env.Tags[k]; !ok {
			env.Tags[k] = v
		}
	}

	return nil
}
