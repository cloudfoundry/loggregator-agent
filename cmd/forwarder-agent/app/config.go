package app

import (
	"fmt"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
)

// Config holds the configuration for the forwarder agent
type Config struct {
	APIURL             string        `env:"API_URL,              required, report"`
	APICAFile          string        `env:"API_CA_FILE_PATH,     required, report"`
	APICertFile        string        `env:"API_CERT_FILE_PATH,   required, report"`
	APIKeyFile         string        `env:"API_KEY_FILE_PATH,    required, report"`
	APICommonName      string        `env:"API_COMMON_NAME,      required, report"`
	APISkipCertVerify  bool          `env:"API_SKIP_CERT_VERIFY, report"`
	APIPollingInterval time.Duration `env:"API_POLLING_INTERVAL, report"`
	APIBatchSize       int           `env:"API_BATCH_SIZE, report"`

	DebugPort uint16 `env:"DEBUG_PORT, report"`
}

// LoadConfig will load the configuration for the forwarder agent from the
// environment. If loading the config fails for any reason this function will
// panic.
func LoadConfig() Config {
	cfg := Config{
		APIPollingInterval: 15 * time.Second,
	}
	if err := envstruct.Load(&cfg); err != nil {
		panic(fmt.Sprintf("Failed to load config from environment: %s", err))
	}

	return cfg
}
