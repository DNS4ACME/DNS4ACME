package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

const kind = "Domain"
const groupName = "dns4acme.github.io"
const groupVersion = "v1"

var schemeGroupVersion = schema.GroupVersion{Group: groupName, Version: groupVersion}

var (
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	addToScheme   = schemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(schemeGroupVersion,
		&Domain{},
		&DomainList{},
	)

	metav1.AddToGroupVersion(scheme, schemeGroupVersion)
	return nil
}

func init() {
	if err := addToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
}
