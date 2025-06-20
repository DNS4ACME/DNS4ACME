package kubernetes

import (
	"context"
	"github.com/dns4acme/dns4acme/backend"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"log/slog"
)

// Provider implements backend.ExtendedProvider and adds Kubernetes-specific functions.
type Provider interface {
	backend.ExtendedProvider
}

type provider struct {
	config        Config
	domains       objectCRUD[*domain]
	logger        *slog.Logger
	dynamicClient *dynamic.DynamicClient
}

func (p provider) HandleWarningHeaderWithContext(ctx context.Context, code int, agent string, text string) {
	p.logger.WarnContext(ctx, "Kubernetes API warning header", slog.Int("code", code), slog.String("agent", agent), slog.String("text", text))
}

func (p provider) Create(ctx context.Context, domainName string, updateKey string) error {
	p.logger.InfoContext(ctx, "Creating domain", slog.String("domain", domainName))
	return p.domains.create(ctx, &domain{
		TypeMeta: v1.TypeMeta{
			Kind:       kind,
			APIVersion: groupVersion.String(),
		},
		Metadata: v1.ObjectMeta{
			Name: domainName,
		},
		Spec: domainSpec{
			UpdateKey: updateKey,
			Serial:    0,
		},
	})
}

func (p provider) Delete(ctx context.Context, domainName string) error {
	p.logger.InfoContext(ctx, "Deleting domain", slog.String("domain", domainName))
	return p.domains.delete(ctx, domainName)
}

func (p provider) Get(ctx context.Context, domainName string) (backend.ProviderResponse, error) {
	res, err := p.domains.get(ctx, domainName)
	if err != nil {
		return backend.ProviderResponse{}, err
	}
	return backend.ProviderResponse{
		UpdateKey:            res.Spec.UpdateKey,
		Serial:               res.Spec.Serial,
		ACMEChallengeAnswers: res.Spec.ACMEChallengeAnswers,
	}, nil
}

func (p provider) Set(ctx context.Context, domainName string, acmeChallengeAnswers []string) error {
	p.logger.InfoContext(ctx, "Updating domain", slog.String("domain", domainName))
	return p.domains.set(ctx, domainName, func(object *domain) error {
		object.Spec.ACMEChallengeAnswers = acmeChallengeAnswers
		object.Spec.Serial++
		return nil
	})
}

func (p provider) Close(ctx context.Context) error {
	return p.domains.close(ctx)
}
