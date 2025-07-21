package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type zone struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec zoneSpec `json:"spec"`
}

func (d *zone) mutate(mutate func(object *zone) error) (*zone, []patch, error) {
	answers := make([]string, len(d.Spec.ACMEChallengeAnswers))
	copy(answers, d.Spec.ACMEChallengeAnswers)
	newZone := &zone{
		TypeMeta: d.TypeMeta,
		Metadata: *d.Metadata.DeepCopy(),
		Spec: zoneSpec{
			Serial:               d.Spec.Serial,
			ACMEChallengeAnswers: answers,
		},
	}
	if err := mutate(newZone); err != nil {
		return nil, nil, err
	}
	return newZone, newZone.createJSONPatch(d), nil
}

func (d *zone) name() string {
	return d.Metadata.Name
}

func (d *zone) createJSONPatch(previousVersion *zone) []patch {
	return []patch{
		{
			Op:    "test",
			Path:  "/spec/serial",
			Value: previousVersion.Spec.Serial,
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

func (d *zone) checkUpdate(newVersion *zone) bool {
	return newVersion.Spec.Serial >= d.Spec.Serial
}

var _ object[*zone] = &zone{}

type zoneSpec struct {
	Serial               uint32   `json:"serial"`
	ACMEChallengeAnswers []string `json:"acme_challenge_answers,omitempty"`
}

const zoneKind = "Zone"
const zoneResource = "zones"

var zoneGroupVersionResource = schema.GroupVersionResource{Group: groupVersion.Group, Version: groupVersion.Version, Resource: zoneResource}
