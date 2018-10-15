package v2_test

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/v2"
)

var _ = Describe("EnvelopeForwarder", func() {
	It("emits once the batch count has been reached", func() {
		spyAgent := newSpyLoggregatorAgent()
		buffer := &stubBuffer{
			envelopeCount: 10,
		}

		ef := v2.NewEnvelopeForwarder(
			spyAgent.Addr(),
			buffer,
			v2.WithEnvelopeForwarderBatchSize(5),
			v2.WithEnvelopeForwarderDialOptions(
				grpc.WithInsecure(),
			),
		)
		ef.Run(false)

		Eventually(spyAgent.batches, 2).ShouldNot(BeEmpty())

		batch := spyAgent.batches()[0]
		Expect(batch.GetBatch()).To(HaveLen(5))
	})

	It("emits once the batch interval has been reached", func() {

	})

	It("clears batch upon egress failure", func() {

	})

	XIt("increments an egress metric", func() {

	})
})

type stubBuffer struct {
	envelopeCount int
	doneChan      chan struct{}
}

func newStubBuffer(count int) *stubBuffer {
	return &stubBuffer{
		envelopeCount: count,
		doneChan:      make(chan struct{}),
	}
}

func (s *stubBuffer) Next() *loggregator_v2.Envelope {
	for i := 0; i < s.envelopeCount; i++ {
		time.Sleep(20 * time.Millisecond)
		return &loggregator_v2.Envelope{}
	}

	<-s.doneChan

	return nil
}

type spyLoggregatorAgent struct {
	listener net.Listener

	mu       sync.Mutex
	_batches []*loggregator_v2.EnvelopeBatch
}

func newSpyLoggregatorAgent() *spyLoggregatorAgent {
	spyAgent := &spyLoggregatorAgent{}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	spyAgent.listener = lis

	s := grpc.NewServer(
		// grpc.Creds(transportCreds),
		grpc.KeepaliveEnforcementPolicy(
			keepalive.EnforcementPolicy{
				MinTime:             10 * time.Second,
				PermitWithoutStream: true,
			},
		),
	)
	loggregator_v2.RegisterIngressServer(s, spyAgent)

	go func() {
		log.Println(s.Serve(lis))
	}()

	return spyAgent
}

func (a *spyLoggregatorAgent) Addr() string {
	return a.listener.Addr().String()
}

func (a *spyLoggregatorAgent) Sender(loggregator_v2.Ingress_SenderServer) error {
	panic("Not implemented")
}

func (a *spyLoggregatorAgent) BatchSender(senderServer loggregator_v2.Ingress_BatchSenderServer) error {
	for {
		batch, err := senderServer.Recv()
		if err != nil {
			return err
		}

		a.mu.Lock()
		a._batches = append(a._batches, batch)
		a.mu.Unlock()
	}
}

func (a *spyLoggregatorAgent) Send(context.Context, *loggregator_v2.EnvelopeBatch) (*loggregator_v2.SendResponse, error) {
	panic("Not implemented")
}

func (a *spyLoggregatorAgent) batches() []*loggregator_v2.EnvelopeBatch {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a._batches
}
