package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type key struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec keySpec `json:"spec"`
}

func (k *key) name() string {
	return k.Metadata.Name
}

func (k *key) mutate(mutate func(object *key) error) (*key, []patch, error) { //nolint:unused // This is used from the interface
	newKey := &key{
		TypeMeta: k.TypeMeta,
		Metadata: *k.Metadata.DeepCopy(),
		Spec: keySpec{
			SecretRef: k.Spec.SecretRef,
		},
	}
	if err := mutate(newKey); err != nil {
		return nil, nil, err
	}
	return newKey, newKey.createJSONPatch(k), nil
}

func (k *key) checkUpdate(newVersion *key) bool { //nolint:unused // This is used from the interface
	return k.Spec.SecretRef.Key == newVersion.Spec.SecretRef.Key && k.Spec.SecretRef.Name == newVersion.Spec.SecretRef.Name
}

func (k *key) createJSONPatch(previousKey *key) []patch { //nolint:unused // This is used from the interface
	return []patch{
		{
			Op:    "test",
			Path:  "/spec/secretRef/key",
			Value: previousKey.Spec.SecretRef.Key,
		},
		{
			Op:    "test",
			Path:  "/spec/secretRef/name",
			Value: previousKey.Spec.SecretRef.Name,
		},
		{
			Op:    "replace",
			Path:  "/spec/secretRef/key",
			Value: k.Spec.SecretRef.Key,
		},
		{
			Op:    "replace",
			Path:  "/spec/secretRef/name",
			Value: k.Spec.SecretRef.Name,
		},
		{
			Op:   "replace",
			Path: "/metadata/ownerReferences",
			Value: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       secretKind,
					Name:       k.Spec.SecretRef.Name,
				},
			},
		},
	}
}

type keySpec struct {
	SecretRef secretRef `json:"secretRef"`
}

type secretRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

const keyKind = "UpdateKey"
const keyResource = "updatekeys"

var keyGroupVersionResource = schema.GroupVersionResource{Group: groupVersion.Group, Version: groupVersion.Version, Resource: keyResource}
