package kubernetes

import "github.com/dns4acme/dns4acme/backend/registry"

func init() {
	registry.Backends[ID] = &descriptor{}
}
