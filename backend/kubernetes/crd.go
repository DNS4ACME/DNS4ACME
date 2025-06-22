package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type domain struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec domainSpec `json:"spec"`
}

func (d *domain) mutate(mutate func(object *domain) error) (*domain, []patch, error) {
	answers := make([]string, len(d.Spec.ACMEChallengeAnswers))
	copy(answers, d.Spec.ACMEChallengeAnswers)
	newDomain := &domain{
		TypeMeta: d.TypeMeta,
		Metadata: d.Metadata,
		Spec: domainSpec{
			UpdateKey:            d.Spec.UpdateKey,
			Serial:               d.Spec.Serial,
			ACMEChallengeAnswers: answers,
		},
	}
	if err := mutate(newDomain); err != nil {
		return nil, nil, err
	}
	return newDomain, newDomain.createJSONPatch(d), nil
}

func (d *domain) name() string {
	return d.Metadata.Name
}

func (d *domain) createJSONPatch(previousVersion *domain) []patch {
	return []patch{
		{
			Op:    "test",
			Path:  "/spec/serial",
			Value: previousVersion.Spec.Serial,
		},
		{
			Op:    "replace",
			Path:  "/spec/update_key",
			Value: d.Spec.UpdateKey,
		},
		{
			Op:    "replace",
			Path:  "/spec/serial",
			Value: d.Spec.Serial,
		},
		{
			Op:    "replace",
			Path:  "/spec/acme_challenge_answers",
			Value: d.Spec.ACMEChallengeAnswers,
		},
	}
}

func (d *domain) checkUpdate(newVersion *domain) bool {
	return newVersion.Spec.Serial >= d.Spec.Serial
}

var _ object[*domain] = &domain{}

type domainSpec struct {
	UpdateKey            string   `json:"update_key"`
	Serial               uint32   `json:"serial"`
	ACMEChallengeAnswers []string `json:"acme_challenge_answers,omitempty"`
}

const kind = "Domain"
const resource = "domains"

var (
	groupVersion         = schema.GroupVersion{Group: "dns4acme.github.io", Version: "v1"}
	groupVersionResource = schema.GroupVersionResource{Group: groupVersion.Group, Version: groupVersion.Version, Resource: resource}
)
