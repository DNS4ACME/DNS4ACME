package backend

import "context"

// ExtendedProvider defines the functions on top of Provider that a backend wishing to provide full management
// capabilities should implement.
type ExtendedProvider interface {
	Provider

	// CreateKey create a new update key without any zone bindings.
	CreateKey(ctx context.Context, keyName string, secret string) error
	// DeleteKey deletes the specified update key.
	DeleteKey(ctx context.Context, keyName string) error
	// SetKeySecret updates the secret of a specified update key to the specified secret.
	SetKeySecret(ctx context.Context, keyName string, secret string) error
	// BindKey binds the specified key to the specified zone, allowing the update key to be used for that zone.
	BindKey(ctx context.Context, keyName string, zoneName string) error
	// UnbindKey removes all key bindings between a specific zone and an update key.
	UnbindKey(ctx context.Context, keyName string, zoneName string) error

	// CreateZone registers a new zone with the specified name.
	CreateZone(ctx context.Context, zoneName string) error
	// DeleteZone deletes a zone with the specified name.
	DeleteZone(ctx context.Context, zoneName string) error
}

// Provider defines the baseline functionality a backend must implement.
type Provider interface {
	// GetKey returns the key with the specified name if any.
	GetKey(ctx context.Context, keyName string) (ProviderKeyResponse, error)

	// GetZone retrieves the information related to a zone.
	GetZone(ctx context.Context, zoneName string) (ProviderZoneResponse, error)
	// SetZone updates the zone with the specified ACME challenge answers, also implicitly updating the serial.
	SetZone(ctx context.Context, zoneName string, acmeChallengeAnswers []string) error

	// Close shuts down the provider.
	Close(ctx context.Context) error
}

// ProviderKeyResponse defines the fields a Provider needs to fill when returning an update key.
type ProviderKeyResponse struct {
	Secret string
	Zones  []string
}

// ProviderZoneResponse defines the fields a Provider needs to fill when returning a zone.
type ProviderZoneResponse struct {
	Serial               uint32
	ACMEChallengeAnswers []string
}
