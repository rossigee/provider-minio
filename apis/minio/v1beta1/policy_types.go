package v1beta1

import (
	"reflect"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func init() {
	SchemeBuilder.Register(&Policy{}, &PolicyList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="External Name",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,minio}
// +kubebuilder:webhook:verbs=create;update,path=/validate-minio-m-crossplane-io-v1beta1-policy,mutating=false,failurePolicy=fail,groups=minio.m.crossplane.io,resources=policies,versions=v1beta1,name=policies.minio.m.crossplane.io,sideEffects=None,admissionReviewVersions=v1

// Policy is a namespaced managed resource that represents a MinIO policy.
// This is the Crossplane v2 namespaced version.
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicySpec   `json:"spec"`
	Status PolicyStatus `json:"status,omitempty"`
}

// PolicySpec defines the desired state of a Policy
type PolicySpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       PolicyParameters `json:"forProvider,omitempty"`
}

// PolicyStatus defines the observed state of a Policy
type PolicyStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          PolicyProviderStatus `json:"atProvider,omitempty"`
}

// PolicyProviderStatus defines the observed state of a Policy from the provider
type PolicyProviderStatus struct {
	// Policy contains the rendered policy in JSON format as it's applied on minio.
	Policy string `json:"policy,omitempty"`
}

// PolicyParameters define the desired state of a MinIO Policy
type PolicyParameters struct {
	// AllowBucket will create a simple policy that allows all operations for the given bucket.
	// Mutually exclusive to `RawPolicy`.
	AllowBucket string `json:"allowBucket,omitempty"`

	// RawPolicy describes a raw S3 policy ad verbatim.
	// Please consult https://min.io/docs/minio/linux/administration/identity-access-management/policy-based-access-control.html for more details about the policy.
	// Mutually exclusive to `AllowBucket`.
	RawPolicy string `json:"rawPolicy,omitempty"`
}

// +kubebuilder:object:root=true

// PolicyList contains a list of Policy resources
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Policy `json:"items"`
}

// Dummy type metadata.
var (
	PolicyKind             = reflect.TypeOf(Policy{}).Name()
	PolicyGroupKind        = schema.GroupKind{Group: Group, Kind: PolicyKind}.String()
	PolicyKindAPIVersion   = PolicyKind + "." + SchemeGroupVersion.String()
	PolicyGroupVersionKind = SchemeGroupVersion.WithKind(PolicyKind)
)