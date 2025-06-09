package inmemory

import (
	"context"
	"github.com/dns4acme/dns4acme/backend"
	"sync"
)

// New creates a new in-memory backend for the specified domains. This backend is not suitable for production use as
// it doesn't persist the domain serials over restarts.
func New(domains map[string]backend.ProviderResponse) backend.Provider {
	return &provider{
		lock:    &sync.RWMutex{},
		domains: domains,
	}
}

type provider struct {
	lock    *sync.RWMutex
	domains map[string]backend.ProviderResponse
}

func (p *provider) Get(_ context.Context, domain string) (backend.ProviderResponse, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	domainData, ok := p.domains[domain]
	if !ok {
		return backend.ProviderResponse{}, backend.ErrDomainNotInBackend
	}
	return domainData, nil
}

func (p *provider) Set(_ context.Context, domain string, acmeChallengeAnswers []string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, ok := p.domains[domain]; !ok {
		return backend.ErrDomainNotInBackend
	}
	response := p.domains[domain]
	response.Serial += 1
	response.ACMEChallengeAnswers = acmeChallengeAnswers
	p.domains[domain] = response
	return nil
}
