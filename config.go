package dns4acme

import (
	"fmt"
	"github.com/dns4acme/dns4acme/backend"
	"github.com/dns4acme/dns4acme/backend/registry"
	"github.com/dns4acme/dns4acme/core"
	"log/slog"
	"reflect"
)

func NewConfig() *Config {
	backendConfigurations := map[string]backend.Config{}
	for backendID, provider := range registry.Backends {
		backendConfigurations[backendID] = provider.Config()
		reflected := reflect.ValueOf(backendConfigurations[backendID])
		if reflected.Type().Kind() != reflect.Ptr && reflected.Type().Kind() != reflect.Struct {
			panic(fmt.Errorf("invalid backend %s, expected a struct or a pointer to a struct as configuration", backendID))
		}
	}

	return &Config{
		BackendConfigs: backendConfigurations,
	}
}

type BackendConfigs map[string]backend.Config

type Config struct {
	core.Config
	BackendConfigs

	Log     LogConfig `config:"log"`
	Backend string    `config:"backend" description:"Select the backend to use"`
}

type LogConfig struct {
	Level slog.Level `config:"level" default:"info" description:"Log level"`
}
