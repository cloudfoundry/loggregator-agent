package v2_test

import (
	"fmt"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	egress "code.cloudfoundry.org/loggregator-agent/pkg/egress/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CounterAggregator", func() {
	It("calculates totals for same counter envelopes", func() {
		aggregator := egress.NewCounterAggregator()
		env1 := buildCounterEnvelope(10, "name-1", "origin-1")
		env2 := buildCounterEnvelope(15, "name-1", "origin-1")

		Expect(aggregator.Process(env1)).ToNot(HaveOccurred())
		Expect(aggregator.Process(env2)).ToNot(HaveOccurred())

		Expect(env1.GetCounter().GetTotal()).To(Equal(uint64(10)))
		Expect(env2.GetCounter().GetTotal()).To(Equal(uint64(25)))
	})

	It("calculates totals separately for counter envelopes with unique names", func() {
		aggregator := egress.NewCounterAggregator()
		env1 := buildCounterEnvelope(10, "name-1", "origin-1")
		env2 := buildCounterEnvelope(15, "name-2", "origin-1")
		env3 := buildCounterEnvelope(20, "name-3", "origin-1")

		Expect(aggregator.Process(env1)).ToNot(HaveOccurred())
		Expect(aggregator.Process(env2)).ToNot(HaveOccurred())
		Expect(aggregator.Process(env3)).ToNot(HaveOccurred())

		Expect(env1.GetCounter().GetTotal()).To(Equal(uint64(10)))
		Expect(env2.GetCounter().GetTotal()).To(Equal(uint64(15)))
		Expect(env3.GetCounter().GetTotal()).To(Equal(uint64(20)))
	})

	It("calculates totals separately for counter envelopes with same name but unique tags", func() {
		aggregator := egress.NewCounterAggregator()
		env1 := buildCounterEnvelope(10, "name-1", "origin-1")
		env2 := buildCounterEnvelope(15, "name-1", "origin-1")
		env3 := buildCounterEnvelope(20, "name-1", "origin-2")

		Expect(aggregator.Process(env1)).ToNot(HaveOccurred())
		Expect(aggregator.Process(env2)).ToNot(HaveOccurred())
		Expect(aggregator.Process(env3)).ToNot(HaveOccurred())

		Expect(env1.GetCounter().GetTotal()).To(Equal(uint64(10)))
		Expect(env2.GetCounter().GetTotal()).To(Equal(uint64(25)))
		Expect(env3.GetCounter().GetTotal()).To(Equal(uint64(20)))
	})

	It("overwrites aggregated total when total is set", func() {
		aggregator := egress.NewCounterAggregator()
		env1 := buildCounterEnvelope(10, "name-1", "origin-1")
		env2 := buildCounterEnvelopeWithTotal(5000, "name-1", "origin-1")
		env3 := buildCounterEnvelope(10, "name-1", "origin-1")

		Expect(aggregator.Process(env1)).ToNot(HaveOccurred())
		Expect(aggregator.Process(env2)).ToNot(HaveOccurred())
		Expect(aggregator.Process(env3)).ToNot(HaveOccurred())

		Expect(env1.GetCounter().GetTotal()).To(Equal(uint64(10)))
		Expect(env2.GetCounter().GetTotal()).To(Equal(uint64(5000)))
		Expect(env3.GetCounter().GetTotal()).To(Equal(uint64(5010)))
	})

	It("prunes the cache of totals when there are too many unique counters", func() {
		aggregator := egress.NewCounterAggregator()

		env1 := buildCounterEnvelope(500, "unique-name", "origin-1")

		Expect(aggregator.Process(env1)).ToNot(HaveOccurred())
		Expect(env1.GetCounter().GetTotal()).To(Equal(uint64(500)))

		for i := 0; i < 10000; i++ {
			aggregator.Process(buildCounterEnvelope(10, fmt.Sprint("name-", i), "origin-1"))
		}

		env2 := buildCounterEnvelope(10, "unique-name", "origin-1")
		aggregator.Process(env2)

		Expect(env2.GetCounter().GetTotal()).To(Equal(uint64(10)))
	})

	It("keeps the delta as part of the message", func() {
		aggregator := egress.NewCounterAggregator()
		env1 := buildCounterEnvelope(10, "name-1", "origin-1")

		Expect(aggregator.Process(env1)).ToNot(HaveOccurred())
		Expect(env1.GetCounter().GetDelta()).To(Equal(uint64(10)))
	})
})

func buildCounterEnvelope(delta uint64, name, origin string) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Message: &loggregator_v2.Envelope_Counter{
			Counter: &loggregator_v2.Counter{
				Name:  name,
				Delta: delta,
			},
		},
		Tags: map[string]string{
			"origin": origin,
		},
	}
}

func buildCounterEnvelopeWithTotal(total uint64, name, origin string) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Message: &loggregator_v2.Envelope_Counter{
			Counter: &loggregator_v2.Counter{
				Name:  name,
				Total: total,
			},
		},
		Tags: map[string]string{
			"origin": origin,
		},
	}
}
