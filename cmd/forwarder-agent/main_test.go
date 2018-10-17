package main_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"github.com/onsi/gomega/gexec"
	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main", func() {
	var (
		cupsProvider *httptest.Server
		grpcPort     = 20000
	)

	BeforeEach(func() {
		cupsProvider = httptest.NewServer(
			&fakeCC{
				results: results{
					"9be15160-4845-4f05-b089-40e827ba61f1": appBindings{
						Hostname: "org.space.name",
						Drains: []string{
							"syslog://1.1.1.1",
						},
					},
					"9be15160-4845-4f05-b089-40e827ba61f2": appBindings{
						Hostname: "org.space.name",
						Drains: []string{
							"v3-syslog://1.1.1.1",
						},
					},
					"9be15160-4845-4f05-b089-40e827ba61f13": appBindings{
						Hostname: "org.space.name",
						Drains: []string{
							"v3-syslog://1.1.1.2",
						},
					},
				},
			},
		)
	})

	AfterEach(func() {
		gexec.CleanupBuildArtifacts()
		grpcPort++
	})

	It("has a health endpoint", func() {
		session := startForwarderAgent(
			fmt.Sprintf("API_CA_FILE_PATH=%s", testhelper.Cert("capi-ca.crt")),
			fmt.Sprintf("API_CERT_FILE_PATH=%s", testhelper.Cert("forwarder.crt")),
			fmt.Sprintf("API_COMMON_NAME=%s", "capiCA"),
			fmt.Sprintf("API_KEY_FILE_PATH=%s", testhelper.Cert("forwarder.key")),
			"API_POLLING_INTERVAL=10ms",
			"DEBUG_PORT=7392",
			fmt.Sprintf("API_URL=%s", cupsProvider.URL),
			fmt.Sprintf("AGENT_PORT=%d", grpcPort),
			fmt.Sprintf("AGENT_CA_FILE=%s", testhelper.Cert("loggregator-ca.crt")),
			fmt.Sprintf("AGENT_CERT_FILE=%s", testhelper.Cert("metron.crt")),
			fmt.Sprintf("AGENT_KEY_FILE=%s", testhelper.Cert("metron.key")),
		)
		defer session.Kill()

		Eventually(func() int {
			resp, err := http.Get("http://127.0.0.1:7392/debug/vars")
			if err != nil {
				return -1
			}

			return resp.StatusCode
		}).Should(Equal(http.StatusOK))

		f := func() string {
			resp, err := http.Get("http://127.0.0.1:7392/debug/vars")
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			return string(body)
		}
		Eventually(f, 3*time.Second, 500*time.Millisecond).Should(ContainSubstring(`"drainCount": 2`))
		var resp *http.Response
		Eventually(func() int {
			var err error
			resp, err = http.Get("http://127.0.0.1:7392/debug/vars")
			if err != nil {
				return -1
			}

			resp = resp

			return resp.StatusCode
		}).Should(Equal(http.StatusOK))

		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		Expect(string(body)).To(ContainSubstring("drainCount"))
	})

	FIt("forwards all envelopes downstream", func() {
		downstream1 := startSpyLoggregatorV2Ingress()
		downstream2 := startSpyLoggregatorV2Ingress()

		session := startForwarderAgent(
			fmt.Sprintf("API_CA_FILE_PATH=%s", testhelper.Cert("capi-ca.crt")),
			fmt.Sprintf("API_CERT_FILE_PATH=%s", testhelper.Cert("forwarder.crt")),
			fmt.Sprintf("API_COMMON_NAME=%s", "capiCA"),
			fmt.Sprintf("API_KEY_FILE_PATH=%s", testhelper.Cert("forwarder.key")),
			"API_POLLING_INTERVAL=10ms",
			fmt.Sprintf("API_URL=%s", cupsProvider.URL),
			"DOWNSTREAM_INGRESS_ADDRS="+strings.Join([]string{downstream1.addr, downstream2.addr}, ","),
			fmt.Sprintf("AGENT_PORT=%d", grpcPort),
			fmt.Sprintf("AGENT_CA_FILE=%s", testhelper.Cert("loggregator-ca.crt")),
			fmt.Sprintf("AGENT_CERT_FILE=%s", testhelper.Cert("metron.crt")),
			fmt.Sprintf("AGENT_KEY_FILE=%s", testhelper.Cert("metron.key")),
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

		Eventually(downstream1.envelopes, 5).Should(Receive(Equal(e)))
		Eventually(downstream2.envelopes, 5).Should(Receive(Equal(e)))
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

type results map[string]appBindings

type appBindings struct {
	Drains   []string `json:"drains"`
	Hostname string   `json:"hostname"`
}

type fakeCC struct {
	count           int
	called          bool
	withEmptyResult bool
	results         results
}

func (f *fakeCC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/internal/v4/syslog_drain_urls" {
		w.WriteHeader(500)
		return
	}

	if r.URL.Query().Get("batch_size") != "1000" {
		w.WriteHeader(500)
		return
	}

	f.serveWithResults(w, r)
}

func (f *fakeCC) serveWithResults(w http.ResponseWriter, r *http.Request) {
	resultData, err := json.Marshal(struct {
		Results results `json:"results"`
	}{
		Results: f.results,
	})
	if err != nil {
		w.WriteHeader(500)
		return
	}

	if f.count > 0 {
		resultData = []byte(`{"results": {}}`)
	}

	w.Write(resultData)
	if f.withEmptyResult {
		f.count++
	}
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
