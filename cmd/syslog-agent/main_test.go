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
	"code.cloudfoundry.org/loggregator-agent/pkg/binding"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"code.cloudfoundry.org/rfc5424"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main", func() {
	var (
		syslogHTTPS  *syslogHTTPSServer
		syslogTLS    *syslogTCPServer
		cupsProvider *fakeBindingCache
		grpcPort     = 30000
	)

	BeforeEach(func() {
		syslogHTTPS = newSyslogHTTPSServer()
		syslogTLS = newSyslogTLSServer()

		cupsProvider = &fakeBindingCache{
			results: []binding.Binding{
				{
					AppID:    "v2-drain",
					Hostname: "org.space.name",
					Drains: []string{
						syslogHTTPS.server.URL,
					},
				},
				{
					AppID:    "some-id",
					Hostname: "org.space.name",
					Drains: []string{
						strings.Replace(syslogHTTPS.server.URL, "https", "https-v3", 1),
					},
				},
				{
					AppID:    "some-id-tls",
					Hostname: "org.space.name",
					Drains: []string{
						"syslog-tls-v3://" + syslogTLS.lis.Addr().String(),
					},
				},
			},
		}
		cupsProvider.startTLS()
	})

	AfterEach(func() {
		gexec.CleanupBuildArtifacts()
		grpcPort++
	})

	It("has a health endpoint", func() {
		session := startSyslogAgent(
			fmt.Sprintf("CACHE_URL=%s", cupsProvider.URL),
			fmt.Sprintf("CACHE_CA_FILE_PATH=%s", testhelper.Cert("binding-cache-ca.crt")),
			fmt.Sprintf("CACHE_CERT_FILE_PATH=%s", testhelper.Cert("binding-cache-ca.crt")),
			fmt.Sprintf("CACHE_KEY_FILE_PATH=%s", testhelper.Cert("binding-cache-ca.key")),
			fmt.Sprintf("CACHE_COMMON_NAME=%s", "bindingCacheCA"),

			"CACHE_POLLING_INTERVAL=10ms",
			"DEBUG_PORT=7392",

			fmt.Sprintf("AGENT_PORT=%d", grpcPort),
			fmt.Sprintf("AGENT_CA_FILE_PATH=%s", testhelper.Cert("loggregator-ca.crt")),
			fmt.Sprintf("AGENT_CERT_FILE_PATH=%s", testhelper.Cert("metron.crt")),
			fmt.Sprintf("AGENT_KEY_FILE_PATH=%s", testhelper.Cert("metron.key")),
		)
		defer session.Kill()

		f := func() string {
			resp, err := http.Get("http://127.0.0.1:7392/debug/vars")
			if err != nil {
				return ""
			}

			if resp.StatusCode == http.StatusOK {
				b, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					return ""
				}
				return string(b)
			}

			return ""
		}

		Eventually(f).Should(ContainSubstring(`"IngressDropped"`))
		Eventually(f).Should(ContainSubstring(`"IngressV2"`))
		Eventually(f).Should(ContainSubstring(`"DrainCount"`))
		Eventually(f).Should(ContainSubstring(`"BindingRefreshCount"`))
		Eventually(f).Should(ContainSubstring(`"LatencyForLastBindingRefreshMS"`))
		Eventually(f).Should(ContainSubstring(`"DrainIngress"`))
		Eventually(f).Should(ContainSubstring(`"EgressDropped"`))
		Eventually(f).Should(ContainSubstring(`"Egress"`))
	})

	It("forwards envelopes to syslog drains", func() {
		session := startSyslogAgent(
			fmt.Sprintf("CACHE_URL=%s", cupsProvider.URL),
			fmt.Sprintf("CACHE_CA_FILE_PATH=%s", testhelper.Cert("binding-cache-ca.crt")),
			fmt.Sprintf("CACHE_CERT_FILE_PATH=%s", testhelper.Cert("binding-cache-ca.crt")),
			fmt.Sprintf("CACHE_KEY_FILE_PATH=%s", testhelper.Cert("binding-cache-ca.key")),
			fmt.Sprintf("CACHE_COMMON_NAME=%s", "bindingCacheCA"),

			"CACHE_POLLING_INTERVAL=10ms",
			"DRAIN_SKIP_CERT_VERIFY=true",

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

func startSyslogAgent(envs ...string) *gexec.Session {
	path, err := gexec.Build("code.cloudfoundry.org/loggregator-agent/cmd/syslog-agent")
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

type fakeBindingCache struct {
	*httptest.Server
	called  bool
	results []binding.Binding
}

func (f *fakeBindingCache) startTLS() {
	tlsConfig, err := plumbing.NewServerMutualTLSConfig(
		testhelper.Cert("binding-cache-ca.crt"),
		testhelper.Cert("binding-cache-ca.key"),
		testhelper.Cert("binding-cache-ca.crt"),
	)
	Expect(err).ToNot(HaveOccurred())

	f.Server = httptest.NewUnstartedServer(f)
	f.Server.TLS = tlsConfig
	f.Server.StartTLS()
}

func (f *fakeBindingCache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/bindings" {
		w.WriteHeader(500)
		return
	}

	f.serveWithResults(w, r)
}

func (f *fakeBindingCache) serveWithResults(w http.ResponseWriter, r *http.Request) {
	resultData, err := json.Marshal(&f.results)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	w.Write(resultData)
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
