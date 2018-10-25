package app

import (
	"fmt"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
)

// GRPC stores the configuration for the router as a server using a PORT
// with mTLS certs and as a client.
type GRPC struct {
	Port         uint16   `env:"AGENT_PORT, report"`
	CAFile       string   `env:"AGENT_CA_FILE_PATH, required, report"`
	CertFile     string   `env:"AGENT_CERT_FILE_PATH, required, report"`
	KeyFile      string   `env:"AGENT_KEY_FILE_PATH, required, report"`
	CipherSuites []string `env:"AGENT_CIPHER_SUITES, report"`
}

// Config holds the configuration for the forwarder agent
type Config struct {
	APIURL             string        `env:"API_URL,              required, report"`
	APICAFile          string        `env:"API_CA_FILE_PATH,     required, report"`
	APICertFile        string        `env:"API_CERT_FILE_PATH,   required, report"`
	APIKeyFile         string        `env:"API_KEY_FILE_PATH,    required, report"`
	APICommonName      string        `env:"API_COMMON_NAME,      required, report"`
	APIPollingInterval time.Duration `env:"API_POLLING_INTERVAL, report"`
	APIBatchSize       int           `env:"API_BATCH_SIZE, report"`
	BindingPerAppLimit int           `env:"BINDING_PER_APP_LIMIT, report"`

	// DownstreamIngressAddrs will receive each envelope. It is assumed to
	// adhere to the Loggregator Ingress Service and use the provided TLS
	// configuration.
	DownstreamIngressAddrs []string `env:"DOWNSTREAM_INGRESS_ADDRS, report"`

	DebugPort uint16 `env:"DEBUG_PORT, report"`

	// DrainSkipCertVerify will skip SSL hostname validation on external
	// syslog drains.
	DrainSkipCertVerify bool `env:"DRAIN_SKIP_CERT_VERIFY, report"`

	GRPC GRPC
}

// LoadConfig will load the configuration for the forwarder agent from the
// environment. If loading the config fails for any reason this function will
// panic.
func LoadConfig() Config {
	cfg := Config{
		APIPollingInterval: 15 * time.Second,
		GRPC: GRPC{
			Port: 3458,
		},
		BindingPerAppLimit: 5,
	}
	if err := envstruct.Load(&cfg); err != nil {
		panic(fmt.Sprintf("Failed to load config from environment: %s", err))
	}

	envstruct.WriteReport(&cfg)

	return cfg
}
