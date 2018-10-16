package main

import (
	"encoding/json"
	"log"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"google.golang.org/grpc/credentials"
)

var (
	buildVersion string
)

// Config is the configuration for a LogCache.
type Config struct {
	LoggregatorAddr string              `env:"LOGGREGATOR_ADDR,  required, report"`
	DefaultSourceId string              `env:"DEFAULT_SOURCE_ID, required, report"`
	Interval        time.Duration       `env:"INTERVAL,                    report"`
	Counters        CounterDescriptions `env:"COUNTERS_JSON,               report"`
	Gauges          GaugeDescriptions   `env:"GAUGES_JSON,                 report"`

	TLS TLS
}

type CounterDescription struct {
	Addr     string            `json:"addr"`
	Name     string            `json:"name"`
	SourceId string            `json:"source_id, optional"`
	Template string            `json:"template"`
	Tags     map[string]string `json:"tags"`
}

type GaugeDescription struct {
	Addr     string            `json:"addr"`
	Name     string            `json:"name"`
	Unit     string            `json:"unit"`
	SourceId string            `json:"source_id, optional"`
	Template string            `json:"template"`
	Tags     map[string]string `json:"tags"`
}

type TLS struct {
	CAPath   string `env:"CA_PATH,   required, report"`
	CertPath string `env:"CERT_PATH, required, report"`
	KeyPath  string `env:"KEY_PATH,  required, report"`
}

func (tls TLS) Credentials() credentials.TransportCredentials {
	creds, err := plumbing.NewClientCredentials(tls.CertPath, tls.KeyPath, tls.CAPath, "metron")
	if err != nil {
		log.Fatalf("failed to create tls credentials: %s", err)
	}
	return creds
}

// LoadConfig creates Config object from environment variables
func LoadConfig() (*Config, error) {
	c := Config{
		Interval: time.Minute,
	}

	if err := envstruct.Load(&c); err != nil {
		return nil, err
	}

	envstruct.WriteReport(&c)
	return &c, nil
}

type CounterDescriptions struct {
	Descriptions []CounterDescription
}

func (d *CounterDescriptions) UnmarshalEnv(v string) error {
	return json.Unmarshal([]byte(v), &d.Descriptions)
}

type GaugeDescriptions struct {
	Descriptions []GaugeDescription
}

func (d *GaugeDescriptions) UnmarshalEnv(v string) error {
	return json.Unmarshal([]byte(v), &d.Descriptions)
}
