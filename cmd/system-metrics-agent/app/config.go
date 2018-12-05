package app

import (
	"crypto/tls"
	"log"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	loggregator "code.cloudfoundry.org/go-loggregator"
)

// Config holds the configuration for the system metrics agent.
type Config struct {
	LoggregatorAddr string        `env:"LOGGREGATOR_ADDR, required, report"`
	SampleInterval  time.Duration `env:"SAMPLE_INTERVAL,            report"`

	DebugPort uint16 `env:"DEBUG_PORT, report"`

	TLS TLS
}

func LoadConfig() Config {
	cfg := Config{
		SampleInterval: time.Minute,
	}

	if err := envstruct.Load(&cfg); err != nil {
		log.Panicf("failed to load config from environment: %s", err)
	}

	_ = envstruct.WriteReport(&cfg)
	_ = cfg.TLS.Config() // This is to ensure that the TLS creds are valid

	return cfg
}

// TLS holds the configuration for the certificates for mTLS to the
// loggregator agent.
type TLS struct {
	CAPath   string `env:"CA_PATH,   required, report"`
	CertPath string `env:"CERT_PATH, required, report"`
	KeyPath  string `env:"KEY_PATH,  required, report"`
}

func (tls TLS) Config() *tls.Config {
	creds, err := loggregator.NewIngressTLSConfig(
		tls.CAPath,
		tls.CertPath,
		tls.KeyPath,
	)

	if err != nil {
		log.Panicf("failed to create tls config: %s", err)
	}

	return creds
}
