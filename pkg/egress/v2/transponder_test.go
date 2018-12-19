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

		tx := egress.NewTransponder(nexter, writer, 1, time.Nanosecond, testhelper.NewMetricClient())
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

			tx := egress.NewTransponder(nexter, writer, 5, time.Minute, testhelper.NewMetricClient())
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

			tx := egress.NewTransponder(nexter, writer, 5, time.Millisecond, testhelper.NewMetricClient())
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

			tx := egress.NewTransponder(nexter, writer, 5, time.Minute, testhelper.NewMetricClient())
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
			tx := egress.NewTransponder(nexter, writer, 5, time.Minute, spy)
			go tx.Start()

			f := func() uint64 {
				return spy.GetMetric("EgressV2").Delta()
			}

			Eventually(f).Should(Equal(uint64(5)))
		})
	})
})
