package main_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
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
		grpcPort = 20000
	)

	AfterEach(func() {
		gexec.CleanupBuildArtifacts()
		grpcPort++
	})

	It("has a health endpoint", func() {
		session := startForwarderAgent(
			"DEBUG_PORT=7392",
			fmt.Sprintf("AGENT_PORT=%d", grpcPort),
			fmt.Sprintf("AGENT_CA_FILE_PATH=%s", testhelper.Cert("loggregator-ca.crt")),
			fmt.Sprintf("AGENT_CERT_FILE_PATH=%s", testhelper.Cert("metron.crt")),
			fmt.Sprintf("AGENT_KEY_FILE_PATH=%s", testhelper.Cert("metron.key")),
		)
		defer session.Kill()

		var body string
		Eventually(func() int {
			resp, err := http.Get("http://127.0.0.1:7392/debug/vars")
			if err != nil {
				return -1
			}

			if resp.StatusCode == http.StatusOK {
				b, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					return -1
				}
				body = string(b)
			}

			return resp.StatusCode
		}).Should(Equal(http.StatusOK))

		Expect(body).To(ContainSubstring(`"IngressDropped"`))
	})

	It("forwards all envelopes downstream", func() {
		downstream1 := startSpyLoggregatorV2Ingress()
		downstream2 := startSpyLoggregatorV2Ingress()

		session := startForwarderAgent(
			"DOWNSTREAM_INGRESS_ADDRS="+strings.Join([]string{downstream1.addr, downstream2.addr}, ","),
			fmt.Sprintf("AGENT_PORT=%d", grpcPort),
			fmt.Sprintf("AGENT_CA_FILE_PATH=%s", testhelper.Cert("loggregator-ca.crt")),
			fmt.Sprintf("AGENT_CERT_FILE_PATH=%s", testhelper.Cert("metron.crt")),
			fmt.Sprintf("AGENT_KEY_FILE_PATH=%s", testhelper.Cert("metron.key")),
		)
		defer session.Kill()

		tlsConfig, err := loggregator.NewIngressTLSConfig(
			testhelper.Cert("loggregator-ca.crt"),
			testhelper.Cert("metron.crt"),
			testhelper.Cert("metron.key"),
		)
		Expect(err).ToNot(HaveOccurred())
		ingressClient, err := loggregator.NewIngressClient(
			tlsConfig,
			loggregator.WithAddr(fmt.Sprintf("127.0.0.1:%d", grpcPort)),
			loggregator.WithLogger(log.New(GinkgoWriter, "[TEST INGRESS CLIENT] ", 0)),
			loggregator.WithBatchMaxSize(1),
		)
		Expect(err).ToNot(HaveOccurred())

		e := &loggregator_v2.Envelope{
			Timestamp: time.Now().UnixNano(),
			SourceId:  "some-id",
			Message: &loggregator_v2.Envelope_Log{
				Log: &loggregator_v2.Log{
					Payload: []byte("hello"),
				},
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			for {
				select {
				case <-ticker.C:
					ingressClient.Emit(e)
				case <-ctx.Done():
					return
				}
			}
		}()

		var e1, e2 *loggregator_v2.Envelope
		Eventually(downstream1.envelopes, 5).Should(Receive(&e1))
		Eventually(downstream2.envelopes, 5).Should(Receive(&e2))

		Expect(proto.Equal(e1, e)).To(BeTrue())
		Expect(proto.Equal(e2, e)).To(BeTrue())
	})
})

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

func startSpyLoggregatorV2Ingress() *spyLoggregatorV2Ingress {
	s := &spyLoggregatorV2Ingress{
		envelopes: make(chan *loggregator_v2.Envelope, 100),
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
