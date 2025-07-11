package inmemory

import (
	"context"
	"github.com/dns4acme/dns4acme/backend"
	"sync"
)

type Config struct {
	Keys  map[string]*backend.ProviderKeyResponse  `json:"keys"`
	Zones map[string]*backend.ProviderZoneResponse `json:"zones"`
}

func (c Config) Build(_ context.Context) (backend.Provider, error) {
	return &provider{
		lock:  &sync.RWMutex{},
		keys:  c.Keys,
		zones: c.Zones,
	}, nil
}
