package app_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/cmd/forwarder-agent/app"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"github.com/gogo/protobuf/proto"
	"github.com/onsi/gomega/gexec"
	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main", func() {
	var (
		grpcPort   = 20000
		testLogger = log.New(GinkgoWriter, "", log.LstdFlags)

		forwarderAgent *app.ForwarderAgent
		mc             *testhelper.SpyMetricClient
		cfg            app.Config
		ingressClient  *loggregator.IngressClient

		emitEnvelopes = func(ctx context.Context, d time.Duration) {
			go func() {
				ticker := time.NewTicker(d)
				for {
					select {
					case <-ticker.C:
						ingressClient.Emit(sampleEnvelope)
					case <-ctx.Done():
						return
					}
				}
			}()
		}
	)

	BeforeEach(func() {
		mc = testhelper.NewMetricClient()
		cfg = app.Config{
			GRPC: app.GRPC{
				Port:     uint16(grpcPort),
				CAFile:   testhelper.Cert("loggregator-ca.crt"),
				CertFile: testhelper.Cert("metron.crt"),
				KeyFile:  testhelper.Cert("metron.key"),
			},
			DebugPort: 7392,
		}
		ingressClient = newIngressClient(grpcPort)
	})

	AfterEach(func() {
		gexec.CleanupBuildArtifacts()
		grpcPort++
	})

	It("has a health endpoint", func() {
		forwarderAgent = app.NewForwarderAgent(cfg, mc, testLogger)
		go forwarderAgent.Run()

		Eventually(func() bool {
			return mc.HasMetric("IngressDropped")
		}).Should(BeTrue())
	})

	It("forwards all envelopes downstream", func() {
		downstream1 := startSpyLoggregatorV2Ingress()
		downstream2 := startSpyLoggregatorV2Ingress()

		cfg.DownstreamIngressAddrs = []string{downstream1.addr, downstream2.addr}
		forwarderAgent = app.NewForwarderAgent(cfg, mc, testLogger)
		go forwarderAgent.Run()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		emitEnvelopes(ctx, 10*time.Millisecond)

		var e1, e2 *loggregator_v2.Envelope
		Eventually(downstream1.envelopes, 5).Should(Receive(&e1))
		Eventually(downstream2.envelopes, 5).Should(Receive(&e2))

		Expect(proto.Equal(e1, sampleEnvelope)).To(BeTrue())
		Expect(proto.Equal(e2, sampleEnvelope)).To(BeTrue())
	})

	It("continues writing to other consumers if one is slow", func() {
		downstreamNormal := startSpyLoggregatorV2Ingress()
		downstreamBlocking := startSpyLoggregatorV2BlockingIngress()

		cfg.DownstreamIngressAddrs = []string{downstreamBlocking.addr, downstreamNormal.addr}
		forwarderAgent = app.NewForwarderAgent(cfg, mc, testLogger)
		go forwarderAgent.Run()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		emitEnvelopes(ctx, 1*time.Millisecond)

		Eventually(downstreamNormal.envelopes, 5).Should(Receive())

		var prevSize int
		Consistently(func() bool {
			notEqual := len(downstreamNormal.envelopes) != prevSize
			prevSize = len(downstreamNormal.envelopes)
			return notEqual
		}, 5, 1).Should(BeTrue())
	})
})

var sampleEnvelope = &loggregator_v2.Envelope{
	Timestamp: time.Now().UnixNano(),
	SourceId:  "some-id",
	Message: &loggregator_v2.Envelope_Log{
		Log: &loggregator_v2.Log{
			Payload: []byte("hello"),
		},
	},
}

func newIngressClient(port int) *loggregator.IngressClient {
	tlsConfig, err := loggregator.NewIngressTLSConfig(
		testhelper.Cert("loggregator-ca.crt"),
		testhelper.Cert("metron.crt"),
		testhelper.Cert("metron.key"),
	)
	Expect(err).ToNot(HaveOccurred())
	ingressClient, err := loggregator.NewIngressClient(
		tlsConfig,
		loggregator.WithAddr(fmt.Sprintf("127.0.0.1:%d", port)),
		loggregator.WithLogger(log.New(GinkgoWriter, "[TEST INGRESS CLIENT] ", 0)),
		loggregator.WithBatchMaxSize(1),
	)
	Expect(err).ToNot(HaveOccurred())
	return ingressClient
}

func startForwarderAgent(envs ...string) *gexec.Session {
	path, err := gexec.Build("code.cloudfoundry.org/loggregator-agent/cmd/forwarder-agent")
	if err != nil {
		panic(err)
	}

	cmd := exec.Command(path)
	cmd.Env = envs
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		panic(err)
	}

	return session
}

func startSpyLoggregatorV2Ingress() *spyLoggregatorV2Ingress {
	s := &spyLoggregatorV2Ingress{
		envelopes: make(chan *loggregator_v2.Envelope, 10000),
	}

	serverCreds, err := plumbing.NewServerCredentials(
		testhelper.Cert("metron.crt"),
		testhelper.Cert("metron.key"),
		testhelper.Cert("loggregator-ca.crt"),
	)

	lis, err := net.Listen("tcp", ":0")
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	grpcServer := grpc.NewServer(grpc.Creds(serverCreds))
	loggregator_v2.RegisterIngressServer(grpcServer, s)

	s.close = func() {
		lis.Close()
	}
	s.addr = lis.Addr().String()

	go grpcServer.Serve(lis)

	return s
}

type spyLoggregatorV2Ingress struct {
	addr      string
	close     func()
	envelopes chan *loggregator_v2.Envelope
}

func (s *spyLoggregatorV2Ingress) Sender(loggregator_v2.Ingress_SenderServer) error {
	panic("not implemented")
}

func (s *spyLoggregatorV2Ingress) Send(context.Context, *loggregator_v2.EnvelopeBatch) (*loggregator_v2.SendResponse, error) {
	panic("not implemented")
}

func (s *spyLoggregatorV2Ingress) BatchSender(srv loggregator_v2.Ingress_BatchSenderServer) error {
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

func startSpyLoggregatorV2BlockingIngress() *spyLoggregatorV2BlockingIngress {
	s := &spyLoggregatorV2BlockingIngress{}

	serverCreds, err := plumbing.NewServerCredentials(
		testhelper.Cert("metron.crt"),
		testhelper.Cert("metron.key"),
		testhelper.Cert("loggregator-ca.crt"),
	)

	lis, err := net.Listen("tcp", ":0")
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	grpcServer := grpc.NewServer(grpc.Creds(serverCreds))
	loggregator_v2.RegisterIngressServer(grpcServer, s)

	s.close = func() {
		lis.Close()
	}
	s.addr = lis.Addr().String()

	go grpcServer.Serve(lis)

	return s
}

type spyLoggregatorV2BlockingIngress struct {
	addr  string
	close func()
}

func (s *spyLoggregatorV2BlockingIngress) Sender(loggregator_v2.Ingress_SenderServer) error {
	panic("not implemented")
}

func (s *spyLoggregatorV2BlockingIngress) Send(context.Context, *loggregator_v2.EnvelopeBatch) (*loggregator_v2.SendResponse, error) {
	panic("not implemented")
}

func (s *spyLoggregatorV2BlockingIngress) BatchSender(srv loggregator_v2.Ingress_BatchSenderServer) error {
	c := make(chan struct{})
	for {
		_, err := srv.Recv()
		if err != nil {
			return err
		}

		<-c
	}
}
