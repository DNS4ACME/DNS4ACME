package kubernetes

import (
	"context"
	"github.com/dns4acme/dns4acme/backend"
	"github.com/dns4acme/dns4acme/lang/E"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"log/slog"
	"sync"
)

// Provider implements backend.ExtendedProvider and adds Kubernetes-specific functions.
type Provider interface {
	backend.ExtendedProvider
}

type provider struct {
	config           Config
	zones            objectCRUD[*zone]
	keys             objectCRUD[*key]
	keyBindings      objectCRUD[*keyBinding]
	secrets          objectCRUD[*secret]
	logger           *slog.Logger
	dynamicClient    *dynamic.DynamicClient
	keyBindingsLock  *sync.RWMutex
	keyBindingsByKey map[string]map[string]keyBindingSpec
}

func (p provider) CreateKey(ctx context.Context, keyName string, secretData string) error {
	newSecret, err := p.secrets.create(ctx, &secret{
		TypeMeta: v1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		Metadata: v1.ObjectMeta{
			GenerateName: keyName,
		},
		Data: map[string]string{
			"key": secretData,
		},
	})
	if err != nil {
		p.logger.WarnContext(
			ctx,
			"Failed to create secret for update key",
			slog.String("updateKey", keyName),
			slog.String("error", err.Error()),
		)
		return err
	}
	newKey, err := p.keys.create(ctx, &key{
		TypeMeta: v1.TypeMeta{
			Kind:       keyKind,
			APIVersion: keyBindingGroupVersionResource.GroupVersion().String(),
		},
		Metadata: v1.ObjectMeta{
			Name:      keyName,
			Namespace: p.config.Namespace,
		},
		Spec: keySpec{
			SecretRef: secretRef{
				Name: newSecret.name(),
				Key:  "key",
			},
		},
	})
	if err != nil {
		if err := p.secrets.delete(ctx, newSecret.name()); err != nil {
			p.logger.WarnContext(
				ctx,
				"Cannot clean up secret after update key creation failed",
				slog.String("secret", newSecret.name()),
				slog.String("updateKey", keyName),
				slog.String("error", err.Error()),
			)
		}
		if E.Is(err, backend.ErrObjectBackendConflict) {
			return err
		}
		p.logger.WarnContext(
			ctx,
			"Failed to create update key",
			slog.String("updateKey", keyName),
			slog.String("error", err.Error()),
		)
		return err
	}
	if err := p.secrets.set(ctx, newSecret.name(), func(object *secret) error {
		object.Metadata.OwnerReferences = []v1.OwnerReference{
			{
				APIVersion: keyGroupVersionResource.GroupVersion().String(),
				Kind:       keyKind,
				Name:       keyName,
				UID:        newKey.Metadata.UID,
			},
		}
		return nil
	}); err != nil {
		// This is intentionally not a full error since it doesn't prevent DNS4ACME from working.
		p.logger.WarnContext(
			ctx,
			"Could not set up owner references for secret, secret will not automatically be cleaned up when the owning update key is deleted",
			slog.String("secret", newSecret.name()),
			slog.String("updateKey", newKey.name()),
			slog.String("error", err.Error()),
		)
	}
	return nil
}

func (p provider) GetKey(ctx context.Context, keyName string) (backend.ProviderKeyResponse, error) {
	keyData, err := p.keys.get(ctx, keyName)
	if err != nil {
		if E.Is(err, backend.ErrObjectNotInBackend) {
			return backend.ProviderKeyResponse{}, backend.ErrKeyNotFoundInBackend.Wrap(err)
		}
		p.logger.WarnContext(
			ctx,
			"Error getting key",
			slog.String("key", keyName),
			slog.String("error", err.Error()),
		)
		return backend.ProviderKeyResponse{}, err
	}

	secret, err := p.secrets.get(ctx, keyData.Spec.SecretRef.Name)
	if err != nil {
		if E.Is(err, backend.ErrObjectNotInBackend) {
			p.logger.DebugContext(
				ctx,
				"Referenced secret not found",
				slog.String("secret", keyData.Spec.SecretRef.Name),
				slog.String("referencedByKey", keyData.Metadata.Name),
			)
			return backend.ProviderKeyResponse{}, backend.ErrKeyNotFoundInBackend.Wrap(err)
		}
		p.logger.WarnContext(
			ctx,
			"Error getting secret",
			slog.String("secret", keyData.Spec.SecretRef.Name),
			slog.String("error", err.Error()),
		)
		return backend.ProviderKeyResponse{}, err
	}
	updateKey, ok := secret.Data[keyData.Spec.SecretRef.Key]
	if !ok {
		p.logger.DebugContext(
			ctx,
			"Referenced secret does not contain key in data section",
			slog.String("key", keyData.Spec.SecretRef.Key),
			slog.String("secret", keyData.Spec.SecretRef.Name),
			slog.String("referencedByKey", keyData.Metadata.Name),
		)
		return backend.ProviderKeyResponse{}, backend.ErrKeyNotFoundInBackend
	}

	result := backend.ProviderKeyResponse{
		Secret: updateKey,
		Zones:  nil,
	}

	p.keyBindingsLock.RLock()
	defer p.keyBindingsLock.RUnlock()
	keyBindings, ok := p.keyBindingsByKey[keyName]
	if !ok {
		return result, nil
	}
	zones := map[string]struct{}{}
	for _, keyBindingData := range keyBindings {
		if _, ok := zones[keyBindingData.Zone]; ok {
			continue
		}
		zones[keyBindingData.Zone] = struct{}{}
		result.Zones = append(result.Zones, keyBindingData.Zone)
	}
	return result, nil
}

func (p provider) DeleteKey(ctx context.Context, keyName string) error {
	p.logger.InfoContext(ctx, "Deleting update key", slog.String("updateKey", keyName))
	// Secrets are auto-deleted due to the owner reference if present.
	if err := p.keys.delete(ctx, keyName); err != nil {
		if E.Is(err, backend.ErrObjectNotInBackend) {
			return backend.ErrKeyNotFoundInBackend.Wrap(err)
		}
		p.logger.WarnContext(
			ctx,
			"Error deleting update key",
			slog.String("updateKey", keyName),
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}

func (p provider) SetKeySecret(ctx context.Context, keyName string, secretData string) error {
	p.logger.InfoContext(ctx, "Setting secret for update key", slog.String("updateKey", keyName))
	keyData, err := p.keys.get(ctx, keyName)
	if err != nil {
		if E.Is(err, backend.ErrObjectNotInBackend) {
			return backend.ErrKeyNotFoundInBackend.Wrap(err)
		}
		p.logger.WarnContext(
			ctx,
			"Error getting update key",
			slog.String("updateKey", keyName),
			slog.String("error", err.Error()),
		)
		return err
	}
	if err := p.secrets.set(ctx, keyData.Spec.SecretRef.Name, func(object *secret) error {
		object.Data[keyData.Spec.SecretRef.Key] = secretData
		return nil
	}); err != nil {
		if E.Is(err, backend.ErrObjectNotInBackend) {
			return backend.ErrKeyNotFoundInBackend.Wrap(err)
		}
		p.logger.WarnContext(
			ctx,
			"Error updating secret",
			slog.String("updateKey", keyName),
			slog.String("secret", keyData.Spec.SecretRef.Name),
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}

func (p provider) BindKey(ctx context.Context, keyName string, zoneName string) error {
	p.logger.InfoContext(
		ctx,
		"Binding update key to zone",
		slog.String("updateKey", keyName),
		slog.String("zone", zoneName),
	)
	theKey, err := p.keys.get(ctx, keyName)
	if err != nil {
		return err
	}
	theZone, err := p.zones.get(ctx, zoneName)
	if err != nil {
		return err
	}
	_, err = p.keyBindings.create(ctx, &keyBinding{
		TypeMeta: v1.TypeMeta{
			Kind:       keyBindingKind,
			APIVersion: keyBindingGroupVersionResource.GroupVersion().String(),
		},
		Metadata: v1.ObjectMeta{
			GenerateName: keyName + "-binding-" + zoneName + "-",
			Namespace:    p.config.Namespace,
			OwnerReferences: []v1.OwnerReference{
				{
					Kind:       keyKind,
					APIVersion: keyGroupVersionResource.GroupVersion().String(),
					Name:       keyName,
					UID:        theKey.Metadata.UID,
				},
				{
					Kind:       zoneKind,
					APIVersion: zoneGroupVersionResource.GroupVersion().String(),
					Name:       zoneName,
					UID:        theZone.Metadata.UID,
				},
			},
		},
		Spec: keyBindingSpec{
			Zone:      zoneName,
			UpdateKey: keyName,
		},
	})
	return err
}

func (p provider) UnbindKey(ctx context.Context, keyName string, zoneName string) error {
	p.keyBindingsLock.RLock()
	defer p.keyBindingsLock.RUnlock()
	bindings, ok := p.keyBindingsByKey[keyName]
	if !ok {
		return nil
	}
	for bindingName, binding := range bindings {
		if binding.Zone != zoneName {
			continue
		}
		if err := p.keyBindings.delete(ctx, bindingName); err != nil {
			if E.Is(err, backend.ErrObjectNotInBackend) {
				continue
			}
			p.logger.WarnContext(
				ctx,
				"Error unbinding update key from zone",
				slog.String("keyBinding", bindingName),
				slog.String("updateKey", keyName),
				slog.String("zone", zoneName),
				slog.String("error", err.Error()),
			)
			return err
		}
	}
	return nil
}

func (p provider) CreateZone(ctx context.Context, zoneName string) error {
	p.logger.InfoContext(ctx, "Creating zone", slog.String("zone", zoneName))
	_, err := p.zones.create(ctx, &zone{
		TypeMeta: v1.TypeMeta{
			Kind:       zoneKind,
			APIVersion: groupVersion.String(),
		},
		Metadata: v1.ObjectMeta{
			Name:      zoneName,
			Namespace: p.config.Namespace,
		},
		Spec: zoneSpec{
			Serial: 0,
		},
	})
	return err
}

func (p provider) GetZone(ctx context.Context, zoneName string) (backend.ProviderZoneResponse, error) {
	zoneData, err := p.zones.get(ctx, zoneName)
	if err != nil {
		if E.Is(err, backend.ErrObjectNotInBackend) {
			return backend.ProviderZoneResponse{}, backend.ErrZoneNotInBackend.Wrap(err)
		}
		p.logger.WarnContext(
			ctx,
			"Error getting zone",
			slog.String("zone", zoneName),
			slog.String("error", err.Error()),
		)
		return backend.ProviderZoneResponse{}, err
	}
	return backend.ProviderZoneResponse{
		Serial:               zoneData.Spec.Serial,
		ACMEChallengeAnswers: zoneData.Spec.ACMEChallengeAnswers,
	}, nil
}

func (p provider) SetZone(ctx context.Context, zoneName string, acmeChallengeAnswers []string) error {
	return p.zones.set(ctx, zoneName, func(object *zone) error {
		object.Spec.ACMEChallengeAnswers = acmeChallengeAnswers
		object.Spec.Serial++
		return nil
	})
}

func (p provider) DeleteZone(ctx context.Context, zoneName string) error {
	p.logger.InfoContext(ctx, "Deleting zone", slog.String("zone", zoneName))
	return p.zones.delete(ctx, zoneName)
}

func (p provider) HandleWarningHeaderWithContext(ctx context.Context, code int, agent string, text string) {
	p.logger.WarnContext(
		ctx,
		"Kubernetes API warning header",
		slog.Int("code", code),
		slog.String("agent", agent),
		slog.String("text", text),
	)
}

func (p provider) Close(ctx context.Context) error {
	return p.zones.close(ctx)
}

func (p provider) updateKeyBindingIndex(change changeType, binding *keyBinding, oldBinding *keyBinding) {
	p.keyBindingsLock.Lock()
	defer p.keyBindingsLock.Unlock()
	add := func(binding *keyBinding) {
		if _, ok := p.keyBindingsByKey[binding.Spec.UpdateKey]; !ok {
			p.keyBindingsByKey[binding.Spec.UpdateKey] = map[string]keyBindingSpec{}
		}
		p.keyBindingsByKey[binding.Spec.UpdateKey][binding.name()] = binding.Spec
	}
	remove := func(binding *keyBinding) {
		if _, ok := p.keyBindingsByKey[binding.Spec.UpdateKey]; !ok {
			delete(p.keyBindingsByKey, binding.Spec.UpdateKey)
		}
	}

	switch change {
	case changeTypeAdd:
		add(binding)
	case changeTypeUpdate:
		remove(oldBinding)
		add(binding)
	case changeTypeDelete:
		remove(oldBinding)
	}
}
