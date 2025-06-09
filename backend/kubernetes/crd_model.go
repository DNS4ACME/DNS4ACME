package kubernetes

import (
	"github.com/dns4acme/dns4acme/backend"
	"k8s.io/apimachinery/pkg/runtime"
)
import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Domain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec backend.ProviderResponse `json:"spec"`
}

func (in *Domain) DeepCopyInto(out *Domain) {
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = in.ObjectMeta
	newAnswers := make([]string, len(in.Spec.ACMEChallengeAnswers))
	copy(newAnswers, in.Spec.ACMEChallengeAnswers)
	out.Spec = backend.ProviderResponse{
		Serial:               in.Spec.Serial,
		UpdateKey:            in.Spec.UpdateKey,
		ACMEChallengeAnswers: newAnswers,
	}
}

func (in *Domain) DeepCopyObject() runtime.Object {
	out := Domain{}
	in.DeepCopyInto(&out)
	return &out
}

type DomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Domain `json:"items"`
}

func (in *DomainList) DeepCopyObject() runtime.Object {
	out := DomainList{}
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		out.Items = make([]Domain, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}

	return &out
}
