package app_test

import (
	"context"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/cmd/system-metrics-agent/app"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("SystemMetricsAgent", func() {
	var (
		loggr *spyLoggregator
		agent *app.SystemMetricsAgent
	)

	BeforeEach(func() {
		loggr = newSpyLoggregator()
		agent = app.NewSystemMetricsAgent(
			app.Config{
				LoggregatorAddr: loggr.addr,
				SampleInterval:  time.Millisecond,
				TLS: app.TLS{
					CAPath:   testhelper.Cert("loggregator-ca.crt"),
					CertPath: testhelper.Cert("metron.crt"),
					KeyPath:  testhelper.Cert("metron.key"),
				},
				Deployment: "some-deployment",
				Job:        "some-job",
				Index:      "some-index",
				IP:         "some-ip",
			},
			log.New(GinkgoWriter, "", log.LstdFlags),
		)
	})

	It("has an http listener for PProf", func() {
		go agent.Run()
		defer agent.Shutdown(context.Background())

		var addr string
		Eventually(func() int {
			addr = agent.DebugAddr()
			return len(addr)
		}).ShouldNot(Equal(0))

		resp, err := http.Get("http://" + addr + "/debug/pprof/")
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("has a prom exposition endpoint", func() {
		go agent.Run()
		defer agent.Shutdown(context.Background())

		var addr string
		Eventually(func() int {
			addr = agent.MetricsAddr()
			return len(addr)
		}).ShouldNot(Equal(0))

		resp, err := http.Get("http://" + addr + "/metrics")
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	DescribeTable("default prom labels", func(label string) {
		go agent.Run()
		defer agent.Shutdown(context.Background())

		var addr string
		Eventually(func() int {
			addr = agent.MetricsAddr()
			return len(addr)
		}).ShouldNot(Equal(0))

		Eventually(hasLabel(addr, label)).Should(BeTrue())
	},
		Entry("origin", `origin="system_metrics_agent"`),
		Entry("source_id", `source_id="system_metrics_agent"`),
		Entry("deployment", `deployment="some-deployment"`),
		Entry("job", `job="some-job"`),
		Entry("index", `index="some-index"`),
		Entry("ip", `ip="some-ip"`),
	)
})

func hasLabel(addr, label string) func() bool {
	return func() bool {
		resp, err := http.Get("http://" + addr + "/metrics")
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		return strings.Contains(string(body), label)
	}
}

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
