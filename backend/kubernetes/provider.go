package kubernetes

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"github.com/dns4acme/dns4acme/backend"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"log/slog"
)

// Provider implements backend.ExtendedProvider and adds Kubernetes-specific functions.
type Provider interface {
	backend.ExtendedProvider
}

func processKubernetesError(err error, domainName string) error {
	var kubeError *kubeerrors.StatusError
	if !errors.As(err, &kubeError) {
		return err
	}
	return backend.ErrBackendRequestFailed.Wrap(err).
		WithAttr(slog.String("domain", domainName)).
		WithAttr(slog.String("reason", string(kubeError.Status().Reason))).
		WithAttr(slog.String("status", kubeError.Status().Status))
}

type provider struct {
	config        Config
	absPathPrefix []string
	cli           *kubernetes.Clientset
	restClient    *restclient.RESTClient
}

func (p provider) Create(ctx context.Context, domainName string, updateKey string) error {
	domainObj := Domain{
		TypeMeta: metav1.TypeMeta{
			Kind:       kind,
			APIVersion: groupName + "/" + groupVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: domainName,
		},
		Spec: backend.ProviderResponse{
			UpdateKey: updateKey,
		},
	}
	domainData, err := json.Marshal(domainObj)
	if err != nil {
		return backend.ErrBackendRequestFailed.Wrap(err)
	}
	if err := p.restClient.Post().
		AbsPath(p.getDomainsAbsPath(ctx)...).
		Body(domainData).
		Do(ctx).
		Error(); err != nil {
		// TODO handle non-existent case
		return processKubernetesError(err, domainName)
	}
	return nil
}

func (p provider) Delete(ctx context.Context, domainName string) error {
	if err := p.restClient.Delete().
		AbsPath(p.getDomainAbsPath(ctx, domainName)...).
		Do(ctx).
		Error(); err != nil {
		// TODO handle non-existent case
		return processKubernetesError(err, domainName)
	}
	return nil
}

func (p provider) Get(ctx context.Context, domainName string) (backend.ProviderResponse, error) {
	result := &Domain{}
	err := p.restClient.Get().
		AbsPath(p.getDomainAbsPath(ctx, domainName)...).
		Do(ctx).
		Into(result)
	if err != nil {
		return backend.ProviderResponse{}, processKubernetesError(err, domainName)
	}
	return result.Spec, nil
}

func (p provider) Set(ctx context.Context, domainName string, acmeChallengeAnswers []string) error {
	original, err := p.Get(ctx, domainName)
	if err != nil {
		return err
	}

	result := &Domain{}
	type Patch struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value"`
	}
	changes := []Patch{
		{
			Op:   "replace",
			Path: "/spec/serial",
			// TODO there is a possible race condition here where the serial doesn't change. Is there a better way
			//      to update this value?
			Value: original.Serial + 1,
		},
		{
			Op:    "replace",
			Path:  "/spec/acme_challenge_answers",
			Value: acmeChallengeAnswers,
		},
	}
	encodedChanges, err := json.Marshal(changes)
	if err != nil {
		return backend.ErrBackendRequestFailed.Wrap(err).WithAttr(slog.String("domain", domainName))
	}

	if err := p.restClient.Patch(types.JSONPatchType).
		AbsPath(p.getDomainAbsPath(ctx, domainName)...).
		Body(encodedChanges).
		Do(ctx).
		Into(result); err != nil {
		return processKubernetesError(err, domainName)
	}
	return nil
}

func (p provider) getDomainAbsPath(_ context.Context, domainName string) []string {
	l := len(p.absPathPrefix)
	path := make([]string, l+6)
	copy(path, p.absPathPrefix)
	path[l] = groupName
	path[l+1] = groupVersion
	path[l+2] = "namespaces"
	path[l+3] = p.config.Namespace
	path[l+4] = "domains"
	path[l+5] = domainName
	return path
}

func (p provider) getDomainsAbsPath(_ context.Context) []string {
	l := len(p.absPathPrefix)
	path := make([]string, l+5)
	copy(path, p.absPathPrefix)
	path[l] = groupName
	path[l+1] = groupVersion
	path[l+2] = "namespaces"
	path[l+3] = p.config.Namespace
	path[l+4] = "domains"
	return path
}
