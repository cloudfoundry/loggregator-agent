package app_test

import (
	"context"
	"log"
	"net"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/cmd/system-metrics-agent/app"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SystemMetricsAgent", func() {
	It("sends system metrics on an interval", func() {
		loggr := newSpyLoggregator()
		a := app.NewSystemMetricsAgent(
			app.Config{
				LoggregatorAddr: loggr.addr,
				MetricInterval:  100 * time.Millisecond,
				TLS: app.TLS{
					CAPath:   testhelper.Cert("loggregator-ca.crt"),
					CertPath: testhelper.Cert("metron.crt"),
					KeyPath:  testhelper.Cert("metron.key"),
				},
			},
			log.New(GinkgoWriter, "", log.LstdFlags),
		)
		a.Run(false)
		defer a.Stop()

		var env *loggregator_v2.Envelope
		Eventually(loggr.envelopes, 5).Should(Receive(&env))
	})
})

type spyLoggregator struct {
	addr      string
	close     func()
	envelopes chan *loggregator_v2.Envelope
}

func newSpyLoggregator() *spyLoggregator {
	s := &spyLoggregator{
		envelopes: make(chan *loggregator_v2.Envelope, 100),
	}

	serverCreds, err := plumbing.NewServerCredentials(
		testhelper.Cert("metron.crt"),
		testhelper.Cert("metron.key"),
		testhelper.Cert("loggregator-ca.crt"),
	)

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	grpcServer := grpc.NewServer(grpc.Creds(serverCreds))
	loggregator_v2.RegisterIngressServer(grpcServer, s)

	s.close = func() {
		lis.Close()
	}
	s.addr = lis.Addr().String()

	go grpcServer.Serve(lis)

	return s
}

func (s *spyLoggregator) Sender(loggregator_v2.Ingress_SenderServer) error {
	panic("not implemented")
}

func (s *spyLoggregator) Send(context.Context, *loggregator_v2.EnvelopeBatch) (*loggregator_v2.SendResponse, error) {
	panic("not implemented")
}

func (s *spyLoggregator) BatchSender(srv loggregator_v2.Ingress_BatchSenderServer) error {
	for {
		batch, err := srv.Recv()
		if err != nil {
			return err
		}

		for _, e := range batch.Batch {
			s.envelopes <- e
		}
	}
}
