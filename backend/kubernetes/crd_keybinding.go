package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type keyBinding struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec keyBindingSpec `json:"spec"`
}

func (k *keyBinding) name() string {
	return k.Metadata.Name
}

func (k *keyBinding) mutate(mutate func(object *keyBinding) error) (*keyBinding, []patch, error) { //nolint:unused // This is used from the interface
	newKeyBinding := &keyBinding{
		TypeMeta: k.TypeMeta,
		Metadata: *k.Metadata.DeepCopy(),
		Spec:     k.Spec,
	}
	if err := mutate(newKeyBinding); err != nil {
		return nil, nil, err
	}
	return newKeyBinding, newKeyBinding.createJSONPatch(k), nil
}

func (k *keyBinding) checkUpdate(newVersion *keyBinding) bool { //nolint:unused // This is used from the interface
	return k.Spec.Zone == newVersion.Spec.Zone && k.Spec.UpdateKey == newVersion.Spec.UpdateKey
}

func (k *keyBinding) createJSONPatch(oldKeyBinding *keyBinding) []patch { //nolint:unused // This is used from the interface
	return []patch{
		{
			Op:    "test",
			Path:  "/spec/zone",
			Value: oldKeyBinding.Spec.Zone,
		},
		{
			Op:    "test",
			Path:  "/spec/updateKey",
			Value: oldKeyBinding.Spec.UpdateKey,
		},
		{
			Op:    "replace",
			Path:  "/spec/updateKey",
			Value: k.Spec.UpdateKey,
		},
		{
			Op:    "replace",
			Path:  "/spec/zone",
			Value: k.Spec.Zone,
		},
		{
			Op:   "replace",
			Path: "/metadata/ownerReferences",
			Value: []metav1.OwnerReference{
				{
					APIVersion: groupVersion.String(),
					Kind:       keyKind,
					Name:       k.Spec.UpdateKey,
				},
				{
					APIVersion: groupVersion.String(),
					Kind:       zoneKind,
					Name:       k.Spec.Zone,
				},
			},
		},
	}
}

type keyBindingSpec struct {
	Zone      string `json:"zone"`
	UpdateKey string `json:"updateKey"`
}

const keyBindingKind = "UpdateKeyZoneBinding"
const keyBindingResource = "updatekeyzonebindings"

var keyBindingGroupVersionResource = schema.GroupVersionResource{Group: groupVersion.Group, Version: groupVersion.Version, Resource: keyBindingResource}
