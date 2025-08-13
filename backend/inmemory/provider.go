package inmemory

import (
	"context"
	"github.com/dns4acme/dns4acme/backend"
	"sync"
)

// New creates a new in-memory backend for the specified domains. This backend is not suitable for production use as
// it doesn't persist the domain serials over restarts.
func New(zones map[string]*backend.ProviderZoneResponse, keys map[string]*backend.ProviderKeyResponse) backend.ExtendedProvider {
	return &provider{
		lock:  &sync.RWMutex{},
		keys:  keys,
		zones: zones,
	}
}

type provider struct {
	lock  *sync.RWMutex
	keys  map[string]*backend.ProviderKeyResponse
	zones map[string]*backend.ProviderZoneResponse
}

func (p *provider) CreateKey(_ context.Context, keyName string, secret string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, ok := p.keys[keyName]; ok {
		return backend.ErrObjectBackendConflict
	}
	p.keys[keyName] = &backend.ProviderKeyResponse{
		Secret: secret,
		Zones:  nil,
	}
	return nil
}

func (p *provider) DeleteKey(_ context.Context, keyName string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, ok := p.keys[keyName]; !ok {
		return backend.ErrObjectNotInBackend
	}
	delete(p.keys, keyName)
	return nil
}

func (p *provider) SetKeySecret(_ context.Context, keyName string, secret string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	keyData, ok := p.keys[keyName]
	if !ok {
		return backend.ErrObjectNotInBackend
	}
	keyData.Secret = secret
	return nil
}

func (p *provider) BindKey(_ context.Context, keyName string, zoneName string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	keyData, ok := p.keys[keyName]
	if !ok {
		return backend.ErrObjectNotInBackend
	}
	for _, zone := range keyData.Zones {
		if zone == zoneName {
			return nil
		}
	}
	keyData.Zones = append(keyData.Zones, zoneName)
	return nil
}

func (p *provider) UnbindKey(_ context.Context, keyName string, zoneName string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.unbindKeyLocked(keyName, zoneName)
}

func (p *provider) unbindKeyLocked(keyName string, zoneName string) error {
	keyData, ok := p.keys[keyName]
	if !ok {
		return backend.ErrObjectNotInBackend
	}
	var newZones []string //nolint:prealloc // This is a filtered list
	found := false
	for _, zone := range keyData.Zones {
		if zone == zoneName {
			found = true
			continue
		}
		newZones = append(newZones, zone)
	}
	keyData.Zones = newZones
	if !found {
		return backend.ErrObjectNotInBackend
	}
	return nil
}

func (p *provider) CreateZone(_ context.Context, zoneName string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, ok := p.zones[zoneName]; ok {
		return backend.ErrObjectBackendConflict
	}
	p.zones[zoneName] = &backend.ProviderZoneResponse{}
	return nil
}

func (p *provider) DeleteZone(_ context.Context, zoneName string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, ok := p.zones[zoneName]; !ok {
		return backend.ErrZoneNotInBackend
	}
	for keyName := range p.keys {
		_ = p.unbindKeyLocked(keyName, zoneName)
	}
	delete(p.zones, zoneName)
	return nil
}

func (p *provider) GetKey(_ context.Context, keyName string) (backend.ProviderKeyResponse, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	key, ok := p.keys[keyName]
	if !ok {
		return backend.ProviderKeyResponse{}, backend.ErrKeyNotFoundInBackend
	}
	return *key, nil
}

func (p *provider) GetZone(_ context.Context, zoneName string) (backend.ProviderZoneResponse, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	zone, ok := p.zones[zoneName]
	if !ok {
		return backend.ProviderZoneResponse{}, backend.ErrZoneNotInBackend
	}
	return *zone, nil
}

func (p *provider) SetZone(_ context.Context, zoneName string, acmeChallengeAnswers []string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	zone, ok := p.zones[zoneName]
	if !ok {
		return backend.ErrZoneNotInBackend
	}
	zone.Serial++
	zone.ACMEChallengeAnswers = acmeChallengeAnswers
	return nil
}

func (p *provider) SetZoneDebug(_ context.Context, zoneName string, debug bool) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	zone, ok := p.zones[zoneName]
	if !ok {
		return backend.ErrZoneNotInBackend
	}
	zone.Debug = debug
	return nil
}

func (p *provider) Close(_ context.Context) error {
	return nil
}
