package app

import (
	"log"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
)

// Config holds the configuration for the syslog binding cache
type Config struct {
	APIURL             string        `env:"API_URL,              required, report"`
	APICAFile          string        `env:"API_CA_FILE_PATH,     required, report"`
	APICertFile        string        `env:"API_CERT_FILE_PATH,   required, report"`
	APIKeyFile         string        `env:"API_KEY_FILE_PATH,    required, report"`
	APICommonName      string        `env:"API_COMMON_NAME,      required, report"`
	APIPollingInterval time.Duration `env:"API_POLLING_INTERVAL, report"`
	APIBatchSize       int           `env:"API_BATCH_SIZE, report"`
	CipherSuites       []string      `env:"CIPHER_SUITES, report"`

	DebugPort uint16 `env:"DEBUG_PORT, report"`
	HTTPAddr  string `env:"HTTP_ADDR,  required, report"`
}

// LoadConfig will load the configuration for the syslog binding cache from the
// environment. If loading the config fails for any reason this function will
// panic.
func LoadConfig() Config {
	cfg := Config{
		APIPollingInterval: 15 * time.Second,
	}
	if err := envstruct.Load(&cfg); err != nil {
		log.Panicf("Failed to load config from environment: %s", err)
	}

	envstruct.WriteReport(&cfg)

	return cfg
}
