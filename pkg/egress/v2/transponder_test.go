package v2_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	egress "code.cloudfoundry.org/loggregator-agent/pkg/egress/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transponder", func() {
	It("reads from the buffer to the writer", func() {
		envelope := &loggregator_v2.Envelope{SourceId: "uuid"}
		nexter := newMockNexter()
		nexter.TryNextOutput.Ret0 <- envelope
		nexter.TryNextOutput.Ret1 <- true
		writer := newMockWriter()
		close(writer.WriteOutput.Ret0)

		tx := egress.NewTransponder(nexter, writer, nil, 1, time.Nanosecond, testhelper.NewMetricClient())
		go tx.Start()

		Eventually(nexter.TryNextCalled).Should(Receive())
		Eventually(writer.WriteInput.Msg).Should(Receive(Equal([]*loggregator_v2.Envelope{envelope})))
	})

	Describe("batching", func() {
		It("emits once the batch count has been reached", func() {
			envelope := &loggregator_v2.Envelope{SourceId: "uuid"}
			nexter := newMockNexter()
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			for i := 0; i < 6; i++ {
				nexter.TryNextOutput.Ret0 <- envelope
				nexter.TryNextOutput.Ret1 <- true
			}

			tx := egress.NewTransponder(nexter, writer, nil, 5, time.Minute, testhelper.NewMetricClient())
			go tx.Start()

			var batch []*loggregator_v2.Envelope
			Eventually(writer.WriteInput.Msg).Should(Receive(&batch))
			Expect(batch).To(HaveLen(5))
		})

		It("emits once the batch interval has been reached", func() {
			envelope := &loggregator_v2.Envelope{SourceId: "uuid"}
			nexter := newMockNexter()
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			nexter.TryNextOutput.Ret0 <- envelope
			nexter.TryNextOutput.Ret1 <- true
			close(nexter.TryNextOutput.Ret0)
			close(nexter.TryNextOutput.Ret1)

			tx := egress.NewTransponder(nexter, writer, nil, 5, time.Millisecond, testhelper.NewMetricClient())
			go tx.Start()

			var batch []*loggregator_v2.Envelope
			Eventually(writer.WriteInput.Msg).Should(Receive(&batch))
			Expect(batch).To(HaveLen(1))
		})

		It("clears batch upon egress failure", func() {
			envelope := &loggregator_v2.Envelope{SourceId: "uuid"}
			nexter := newMockNexter()
			writer := newMockWriter()

			go func() {
				for {
					writer.WriteOutput.Ret0 <- errors.New("some-error")
				}
			}()

			for i := 0; i < 6; i++ {
				nexter.TryNextOutput.Ret0 <- envelope
				nexter.TryNextOutput.Ret1 <- true
			}

			tx := egress.NewTransponder(nexter, writer, nil, 5, time.Minute, testhelper.NewMetricClient())
			go tx.Start()

			Eventually(writer.WriteCalled).Should(HaveLen(1))
			Consistently(writer.WriteCalled).Should(HaveLen(1))
		})

		It("emits egress metric", func() {
			envelope := &loggregator_v2.Envelope{SourceId: "uuid"}
			nexter := newMockNexter()
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			for i := 0; i < 6; i++ {
				nexter.TryNextOutput.Ret0 <- envelope
				nexter.TryNextOutput.Ret1 <- true
			}

			spy := testhelper.NewMetricClient()
			tx := egress.NewTransponder(nexter, writer, nil, 5, time.Minute, spy)
			go tx.Start()

			f := func() uint64 {
				return spy.GetMetric("EgressV2").Delta()
			}

			Eventually(f).Should(Equal(uint64(5)))
		})
	})

	Describe("tagging", func() {
		It("adds the given tags to all envelopes", func() {
			tags := map[string]string{
				"tag-one": "value-one",
				"tag-two": "value-two",
			}
			input := &loggregator_v2.Envelope{SourceId: "uuid"}
			nexter := newMockNexter()
			nexter.TryNextOutput.Ret0 <- input
			nexter.TryNextOutput.Ret1 <- true
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			tx := egress.NewTransponder(nexter, writer, tags, 1, time.Nanosecond, testhelper.NewMetricClient())

			go tx.Start()

			Eventually(nexter.TryNextCalled).Should(Receive())

			var output []*loggregator_v2.Envelope
			Eventually(writer.WriteInput.Msg).Should(Receive(&output))

			Expect(output).To(HaveLen(1))
			Expect(output[0].Tags["tag-one"]).To(Equal("value-one"))
			Expect(output[0].Tags["tag-two"]).To(Equal("value-two"))
		})

		It("does not write over tags if they already exist", func() {
			tags := map[string]string{
				"existing-tag": "some-new-value",
			}
			input := &loggregator_v2.Envelope{
				SourceId: "uuid",
				DeprecatedTags: map[string]*loggregator_v2.Value{
					"existing-tag": {
						Data: &loggregator_v2.Value_Text{
							Text: "existing-value",
						},
					},
				},
			}
			nexter := newMockNexter()
			nexter.TryNextOutput.Ret0 <- input
			nexter.TryNextOutput.Ret1 <- true
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			tx := egress.NewTransponder(nexter, writer, tags, 1, time.Nanosecond, testhelper.NewMetricClient())

			go tx.Start()

			Eventually(nexter.TryNextCalled).Should(Receive())

			var output []*loggregator_v2.Envelope
			Eventually(writer.WriteInput.Msg).Should(Receive(&output))
			Expect(output).To(HaveLen(1))

			Expect(output[0].Tags["existing-tag"]).To(Equal("existing-value"))
		})

		It("moves DesprecatedTags to Tags", func() {
			input := &loggregator_v2.Envelope{
				SourceId: "uuid",
				DeprecatedTags: map[string]*loggregator_v2.Value{
					"text-tag":    {Data: &loggregator_v2.Value_Text{Text: "text-value"}},
					"integer-tag": {Data: &loggregator_v2.Value_Integer{Integer: 502}},
					"decimal-tag": {Data: &loggregator_v2.Value_Decimal{Decimal: 0.23}},
				},
			}
			nexter := newMockNexter()
			nexter.TryNextOutput.Ret0 <- input
			nexter.TryNextOutput.Ret1 <- true
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			tx := egress.NewTransponder(nexter, writer, nil, 1, time.Nanosecond, testhelper.NewMetricClient())

			go tx.Start()

			Eventually(nexter.TryNextCalled).Should(Receive())

			var output []*loggregator_v2.Envelope
			Eventually(writer.WriteInput.Msg).Should(Receive(&output))
			Expect(output).To(HaveLen(1))

			Expect(output[0].Tags["text-tag"]).To(Equal("text-value"))
			Expect(output[0].Tags["integer-tag"]).To(Equal("502"))
			Expect(output[0].Tags["decimal-tag"]).To(Equal("0.23"))
		})
	})
})
