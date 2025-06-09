package backend

import "context"

type ExtendedProvider interface {
	Provider

	Create(ctx context.Context, domain string, updateKey string) error
	Delete(ctx context.Context, domain string) error
}

type Provider interface {
	Get(ctx context.Context, domain string) (ProviderResponse, error)
	Set(ctx context.Context, domain string, acmeChallengeAnswers []string) error
}

type ProviderResponse struct {
	UpdateKey            string   `json:"update_key"`
	Serial               uint32   `json:"serial,omitempty"`
	ACMEChallengeAnswers []string `json:"acme_challenge_answers,omitempty"`
}
