package inmemory

import (
	"github.com/dns4acme/dns4acme/backend"
)

type descriptor struct {
}

func (d descriptor) Name() string {
	return "In-Memory"
}

func (d descriptor) Description() string {
	return "Testing backend that stores domains in memory. The data is lost after a restart."
}

func (d descriptor) Config() backend.Config {
	return &config{}
}
