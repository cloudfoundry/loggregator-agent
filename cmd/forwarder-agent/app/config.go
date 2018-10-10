package app

import (
	"fmt"

	envstruct "code.cloudfoundry.org/go-envstruct"
)

// Config holds the configuration for the forwarder agent
type Config struct {
	DebugPort uint32 `env:"DEBUG_PORT"`
}

// LoadConfig will load the configuration for the forwarder agent from the
// environment. If loading the config fails for any reason this function will
// panic.
func LoadConfig() Config {
	var cfg Config
	if err := envstruct.Load(&cfg); err != nil {
		panic(fmt.Sprintf("Failed to load config from environment: %s", err))
	}

	return cfg
}
