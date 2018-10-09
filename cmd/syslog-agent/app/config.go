package app

import (
	"fmt"

	envstruct "code.cloudfoundry.org/go-envstruct"
)

type Config struct {
	DebugPort uint32 `env:"DEBUG_PORT"`
}

func LoadConfig() Config {
	var cfg Config
	if err := envstruct.Load(&cfg); err != nil {
		panic(fmt.Sprintf("Failed to load config from environment: %s", err))
	}

	return cfg
}
