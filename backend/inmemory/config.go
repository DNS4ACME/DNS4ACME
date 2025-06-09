package inmemory

import (
	"github.com/dns4acme/dns4acme/backend"
)

type config struct {
}

func (c config) Build() (backend.Provider, error) {
	return &provider{}, nil
}
