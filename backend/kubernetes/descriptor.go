package kubernetes

import (
	"github.com/dns4acme/dns4acme/backend"
)

type descriptor struct {
}

func (d descriptor) Name() string {
	return "Kubernetes"
}

func (d descriptor) Description() string {
	return "Backend that stores zones and their records in a Kubernetes CRD."
}

func (d descriptor) Config() backend.Config {
	return &Config{}
}
