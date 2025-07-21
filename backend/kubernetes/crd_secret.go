package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type secret struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`

	Data map[string]string `json:"data"`
}

func (s *secret) name() string {
	return s.Metadata.Name
}

func (s *secret) mutate(mutate func(object *secret) error) (*secret, []patch, error) { //nolint:unused // This is used from the interface
	newSecret := &secret{
		TypeMeta: s.TypeMeta,
		Metadata: *s.Metadata.DeepCopy(),
		Data:     make(map[string]string, len(s.Data)),
	}
	for k, v := range s.Data {
		newSecret.Data[k] = v
	}
	if err := mutate(newSecret); err != nil {
		return nil, nil, err
	}
	return newSecret, newSecret.createJSONPatch(s), nil
}

func (s *secret) checkUpdate(newVersion *secret) bool { //nolint:unused // This is used from the interface
	if len(s.Data) != len(newVersion.Data) {
		return false
	}
	for key, value := range s.Data {
		other, ok := newVersion.Data[key]
		if !ok {
			return false
		}
		if value != other {
			return false
		}
	}
	return true
}

func (s *secret) createJSONPatch(oldSecret *secret) []patch { //nolint:unused // This is used from the interface
	return []patch{
		{
			Op:    "test",
			Path:  "/data",
			Value: oldSecret.Data,
		},
		{
			Op:    "replace",
			Path:  "/data",
			Value: s.Data,
		},
	}
}

const secretKind = "Secret"
const secretResource = "secrets"

var secretGroupVersionResource = schema.GroupVersionResource{Group: "", Version: "v1", Resource: secretResource}
