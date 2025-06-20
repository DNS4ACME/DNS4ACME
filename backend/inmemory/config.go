package inmemory

import (
	"context"
	"github.com/dns4acme/dns4acme/backend"
)

type config struct {
}

func (c config) Build(_ context.Context) (backend.Provider, error) {
	return &provider{}, nil
}
