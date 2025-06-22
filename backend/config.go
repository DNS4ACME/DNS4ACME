package backend

import "context"

type ExtendedConfig interface {
	Config
	BuildExtended(ctx context.Context) (ExtendedProvider, error)
}

type Config interface {
	Build(ctx context.Context) (Provider, error)
}
