package backend

type ExtendedConfig interface {
	Config
	BuildExtended() (ExtendedProvider, error)
}

type Config interface {
	Build() (Provider, error)
}
