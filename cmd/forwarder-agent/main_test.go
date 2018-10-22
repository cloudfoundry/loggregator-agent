package main_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"code.cloudfoundry.org/rfc5424"
	"github.com/onsi/gomega/gexec"
	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main", func() {
	var (
		syslogHTTPS  *syslogHTTPSServer
		syslogTLS    *syslogTCPServer
		cupsProvider *httptest.Server
		grpcPort     = 20000
	)

	BeforeEach(func() {
		syslogHTTPS = newSyslogHTTPSServer()
		syslogTLS = newSyslogTLSServer()

		cupsProvider = httptest.NewServer(
			&fakeCC{
				results: results{
					"v2-drain": appBindings{
						Hostname: "org.space.name",
						Drains: []string{
							syslogHTTPS.server.URL,
						},
					},
					"some-id": appBindings{
						Hostname: "org.space.name",
						Drains: []string{
							"v3-" + syslogHTTPS.server.URL,
						},
					},
					"some-id-tls": appBindings{
						Hostname: "org.space.name",
						Drains: []string{
							"v3-syslog-tls://" + syslogTLS.lis.Addr().String(),
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
			fmt.Sprintf("AGENT_CA_FILE_PATH=%s", testhelper.Cert("loggregator-ca.crt")),
			fmt.Sprintf("AGENT_CERT_FILE_PATH=%s", testhelper.Cert("metron.crt")),
			fmt.Sprintf("AGENT_KEY_FILE_PATH=%s", testhelper.Cert("metron.key")),
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

	It("forwards all envelopes downstream", func() {
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

		Eventually(downstream1.envelopes, 5).Should(Receive(Equal(e)))
		Eventually(downstream2.envelopes, 5).Should(Receive(Equal(e)))
	})

	It("forwards envelopes to syslog drains", func() {
		session := startForwarderAgent(
			fmt.Sprintf("API_CA_FILE_PATH=%s", testhelper.Cert("capi-ca.crt")),
			fmt.Sprintf("API_CERT_FILE_PATH=%s", testhelper.Cert("forwarder.crt")),
			fmt.Sprintf("API_COMMON_NAME=%s", "capiCA"),
			fmt.Sprintf("API_KEY_FILE_PATH=%s", testhelper.Cert("forwarder.key")),
			"API_POLLING_INTERVAL=10ms",
			"DRAIN_SKIP_CERT_VERIFY=true",
			fmt.Sprintf("API_URL=%s", cupsProvider.URL),
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

		eTLS := &loggregator_v2.Envelope{
			Timestamp: time.Now().UnixNano(),
			SourceId:  "some-id-tls",
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
					ingressClient.Emit(eTLS)
				case <-ctx.Done():
					return
				}
			}
		}()

		var msg *rfc5424.Message
		Eventually(syslogHTTPS.receivedMessages, 5).Should(Receive(&msg))
		Expect(msg.AppName).To(Equal("some-id"))

		Consistently(func() string {
			select {
			case msg := <-syslogHTTPS.receivedMessages:
				return msg.AppName
			default:
				return ""
			}
		}).ShouldNot(Equal("some-id-tls"))

		Eventually(syslogTLS.receivedMessages, 5).Should(Receive(&msg))
		Expect(msg.AppName).To(Equal("some-id-tls"))
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

type syslogHTTPSServer struct {
	receivedMessages chan *rfc5424.Message
	server           *httptest.Server
}

func newSyslogHTTPSServer() *syslogHTTPSServer {
	syslogServer := syslogHTTPSServer{
		receivedMessages: make(chan *rfc5424.Message, 100),
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		msg := &rfc5424.Message{}

		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}

		err = msg.UnmarshalBinary(data)
		if err != nil {
			panic(err)
		}

		// msg.AppName
		// msg.MessageID
		syslogServer.receivedMessages <- msg
	}))

	syslogServer.server = server
	return &syslogServer
}

type syslogTCPServer struct {
	lis              net.Listener
	mu               sync.Mutex
	receivedMessages chan *rfc5424.Message
}

func newSyslogTCPServer() *syslogTCPServer {
	lis, err := net.Listen("tcp", ":0")
	Expect(err).ToNot(HaveOccurred())
	m := &syslogTCPServer{
		receivedMessages: make(chan *rfc5424.Message, 100),
		lis:              lis,
	}
	go m.accept()
	return m
}

func newSyslogTLSServer() *syslogTCPServer {
	lis, err := net.Listen("tcp", ":0")
	Expect(err).ToNot(HaveOccurred())
	cert, err := tls.LoadX509KeyPair(
		testhelper.Cert("forwarder.crt"),
		testhelper.Cert("forwarder.key"),
	)
	Expect(err).ToNot(HaveOccurred())
	tlsLis := tls.NewListener(lis, &tls.Config{
		Certificates: []tls.Certificate{cert},
	})
	m := &syslogTCPServer{
		receivedMessages: make(chan *rfc5424.Message, 100),
		lis:              tlsLis,
	}
	go m.accept()
	return m
}

func (m *syslogTCPServer) accept() {
	for {
		conn, err := m.lis.Accept()
		if err != nil {
			return
		}
		go m.handleConn(conn)
	}
}

func (m *syslogTCPServer) handleConn(conn net.Conn) {
	for {
		var msg rfc5424.Message

		_, err := msg.ReadFrom(conn)
		if err != nil {
			return
		}

		m.receivedMessages <- &msg
	}
}

func (m *syslogTCPServer) addr() net.Addr {
	return m.lis.Addr()
}
