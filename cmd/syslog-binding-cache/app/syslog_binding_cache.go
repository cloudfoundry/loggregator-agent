package app

import (
	"log"
	"net"
	"net/http"
	"time"

	"code.cloudfoundry.org/loggregator-agent/pkg/binding"
	"code.cloudfoundry.org/loggregator-agent/pkg/cache"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/api"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"github.com/gorilla/mux"
)

type SyslogBindingCache struct {
	addr   string
	config Config
	log    *log.Logger
}

func NewSyslogBindingCache(config Config, log *log.Logger) *SyslogBindingCache {
	return &SyslogBindingCache{
		config: config,
		log:    log,
	}
}

func (sbc *SyslogBindingCache) Run(blocking bool) {
	store := binding.NewStore()
	poller := binding.NewPoller(sbc.apiClient(), sbc.config.APIPollingInterval, store)

	lis, err := net.Listen("tcp", sbc.config.HTTPAddr)
	if err != nil {
		sbc.log.Panicf("error creating listener: %s", err)
	}
	sbc.addr = lis.Addr().String()

	go poller.Poll()

	router := mux.NewRouter()
	router.HandleFunc("/bindings", cache.Handler(store)).Methods(http.MethodGet)

	var opts []plumbing.ConfigOption
	if len(sbc.config.CipherSuites) > 0 {
		opts = append(opts, plumbing.WithCipherSuites(sbc.config.CipherSuites))
	}

	tlsConfig, err := plumbing.NewServerMutualTLSConfig(
		sbc.config.APICertFile,
		sbc.config.APIKeyFile,
		sbc.config.APICAFile,
		opts...,
	)
	if err != nil {
		sbc.log.Panicf("failed to load server TLS config: %s", err)
	}

	server := &http.Server{
		Handler:   router,
		TLSConfig: tlsConfig,
	}

	if blocking {
		server.ServeTLS(lis, "", "")
		return
	}

	go server.ServeTLS(lis, "", "")
}

func (sbc *SyslogBindingCache) Addr() string {
	return sbc.addr
}

func (sbc *SyslogBindingCache) apiClient() api.Client {
	tlsConfig, err := plumbing.NewClientMutualTLSConfig(
		sbc.config.APICertFile,
		sbc.config.APIKeyFile,
		sbc.config.APICAFile,
		sbc.config.APICommonName,
	)
	if err != nil {
		log.Panicf("failed to load API client certificates: %s", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       tlsConfig,
	}
	return api.Client{
		Addr: sbc.config.APIURL,
		Client: &http.Client{
			Transport: transport,
		},
		BatchSize: sbc.config.APIBatchSize,
	}
}
