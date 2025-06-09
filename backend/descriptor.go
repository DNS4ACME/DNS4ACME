package backend

type Descriptor interface {
	Name() string
	Description() string
	Config() Config
}
