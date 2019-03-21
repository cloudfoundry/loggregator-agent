package app_test

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo/extensions/table"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/cmd/syslog-agent/app"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/binding"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"code.cloudfoundry.org/rfc5424"
)

var _ = Describe("SyslogAgent", func() {
	var (
		syslogHTTPS  *syslogHTTPSServer
		syslogTLS    *syslogTCPServer
		cupsProvider *fakeBindingCache

		grpcPort   = 30000
		testLogger = log.New(GinkgoWriter, "", log.LstdFlags)
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
						fmt.Sprintf("syslog-tls-v3://localhost:%s", syslogTLS.port()),
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

	DescribeTable("has metric and expected tags", func(name string, tags map[string]string) {
		mc := testhelper.NewMetricClient()
		cfg := app.Config{
			BindingsPerAppLimit: 5,
			DebugPort:           7392,
			IdleDrainTimeout:    10 * time.Minute,
			Cache: app.Cache{
				URL:             cupsProvider.URL,
				CAFile:          testhelper.Cert("binding-cache-ca.crt"),
				CertFile:        testhelper.Cert("binding-cache-ca.crt"),
				KeyFile:         testhelper.Cert("binding-cache-ca.key"),
				CommonName:      "bindingCacheCA",
				PollingInterval: 10 * time.Millisecond,
			},
			GRPC: app.GRPC{
				Port:     grpcPort,
				CAFile:   testhelper.Cert("loggregator-ca.crt"),
				CertFile: testhelper.Cert("metron.crt"),
				KeyFile:  testhelper.Cert("metron.key"),
			},
		}
		go app.NewSyslogAgent(cfg, mc, testLogger).Run()

		Eventually(func() bool {
			return mc.HasMetric(name, tags)
		}).Should(BeTrue())

		m := mc.GetMetric(name, tags)
		for k, v := range tags {
			Expect(m.Opts.ConstLabels).To(HaveKeyWithValue(k, v))
		}
	},
		//Need to find a place to test that the ingress metric receives the scope tag
		Entry("IngressV2", "ingress", map[string]string{}),
		Entry("OriginMappingsV2", "origin_mappings", map[string]string{}),
		Entry("IngressDropped", "dropped", map[string]string{"direction": "ingress"}),
		Entry("BindingRefreshCount", "binding_refresh_count", map[string]string{}),
		Entry("DrainCount", "drains", map[string]string{"unit": "count"}),
		Entry("ActiveDrainCount", "active_drains", map[string]string{"unit": "count"}),
		Entry("LatencyForLastBindingRefreshMS", "latency_for_last_binding_refresh", map[string]string{"unit": "ms"}),
		Entry("InvalidDrains", "invalid_drains", map[string]string{"unit": "total"}),
		Entry("BlacklistedDrains", "blacklisted_drains", map[string]string{"unit": "total"}),
		Entry("DrainIngress", "ingress", map[string]string{"scope": "all_drains"}),

		Entry("Egress", "egress", map[string]string{}),
		Entry("EgressDropped", "dropped", map[string]string{"direction": "egress"}),
	)

	It("forwards envelopes to syslog drains", func() {
		mc := testhelper.NewMetricClient()
		cfg := app.Config{
			BindingsPerAppLimit: 5,
			DebugPort:           7392,
			IdleDrainTimeout:    10 * time.Minute,
			DrainSkipCertVerify: true,
			Cache: app.Cache{
				URL:             cupsProvider.URL,
				CAFile:          testhelper.Cert("binding-cache-ca.crt"),
				CertFile:        testhelper.Cert("binding-cache-ca.crt"),
				KeyFile:         testhelper.Cert("binding-cache-ca.key"),
				CommonName:      "bindingCacheCA",
				PollingInterval: 10 * time.Millisecond,
				Blacklist:       cups.BlacklistRanges{},
			},
			GRPC: app.GRPC{
				Port:     grpcPort,
				CAFile:   testhelper.Cert("loggregator-ca.crt"),
				CertFile: testhelper.Cert("metron.crt"),
				KeyFile:  testhelper.Cert("metron.key"),
			},
		}

		go app.NewSyslogAgent(cfg, mc, testLogger).Run()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		emitLogs(ctx, grpcPort)

		var msg *rfc5424.Message
		Eventually(syslogHTTPS.receivedMessages, 5).Should(Receive(&msg))
		Expect(msg.AppName).To(Equal("some-id"))

		//Tests Crosstalk between the drains
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

	It("should not send logs to blacklisted IPs", func() {
		mc := testhelper.NewMetricClient()
		cfg := app.Config{
			BindingsPerAppLimit: 5,
			DebugPort:           7392,
			IdleDrainTimeout:    10 * time.Minute,
			DrainSkipCertVerify: true,
			Cache: app.Cache{
				URL:             cupsProvider.URL,
				CAFile:          testhelper.Cert("binding-cache-ca.crt"),
				CertFile:        testhelper.Cert("binding-cache-ca.crt"),
				KeyFile:         testhelper.Cert("binding-cache-ca.key"),
				CommonName:      "bindingCacheCA",
				PollingInterval: 10 * time.Millisecond,
				Blacklist: cups.BlacklistRanges{
					Ranges: []cups.BlacklistRange{
						{
							Start: "127.0.0.1",
							End:   "127.0.0.1",
						},
					},
				},
			},
			GRPC: app.GRPC{
				Port:     grpcPort,
				CAFile:   testhelper.Cert("loggregator-ca.crt"),
				CertFile: testhelper.Cert("metron.crt"),
				KeyFile:  testhelper.Cert("metron.key"),
			},
		}
		go app.NewSyslogAgent(cfg, mc, testLogger).Run()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		emitLogs(ctx, grpcPort)

		var msg *rfc5424.Message
		Consistently(syslogHTTPS.receivedMessages, 5).ShouldNot(Receive(&msg))
	})
})

func emitLogs(ctx context.Context, grpcPort int) {
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

func (m *syslogTCPServer) port() string {
	tokens := strings.Split(m.lis.Addr().String(), ":")
	return tokens[len(tokens)-1]
}
