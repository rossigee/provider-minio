package v1

import (
	"reflect"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func init() {
	SchemeBuilder.Register(&ServiceAccount{}, &ServiceAccountList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="External Name",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="Parent User",type="string",JSONPath=".spec.forProvider.parentUser"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,minio}
// +kubebuilder:webhook:verbs=create;update,path=/validate-minio-crossplane-io-v1-serviceaccount,mutating=false,failurePolicy=fail,groups=minio.crossplane.io,resources=serviceaccounts,versions=v1,name=serviceaccounts.minio.crossplane.io,sideEffects=None,admissionReviewVersions=v1

type ServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceAccountSpec   `json:"spec"`
	Status ServiceAccountStatus `json:"status,omitempty"`
}

type ServiceAccountSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ProviderReference *xpv1.Reference `json:"providerReference,omitempty"`

	ForProvider ServiceAccountParameters `json:"forProvider,omitempty"`
}

type ServiceAccountStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ServiceAccountProviderStatus `json:"atProvider,omitempty"`
}

type ServiceAccountProviderStatus struct {
	// AccessKey is the access key ID for this service account.
	AccessKey string `json:"accessKey,omitempty"`
	// Status indicates the service account's status on the minio instance.
	Status string `json:"status,omitempty"`
	// ParentUser indicates the parent user of this service account.
	ParentUser string `json:"parentUser,omitempty"`
	// Policies contains a list of policies that are applied to this service account
	Policies string `json:"policies,omitempty"`
}

type ServiceAccountParameters struct {
	// ParentUser is the name of the parent user for this service account.
	// This user must exist in MinIO and the service account will inherit its permissions.
	// +kubebuilder:validation:Required
	ParentUser string `json:"parentUser"`

	// ServiceAccountName is the name/description of the service account.
	// Defaults to `metadata.name` if unset.
	// Cannot be changed after service account is created.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Policies contains a list of additional policies that should get assigned to this service account.
	// These policies are in addition to the parent user's policies.
	// These policies need to be created separately by using the policy CRD.
	Policies []string `json:"policies,omitempty"`

	// Expiry is the expiration time for the service account.
	// If not set, the service account will not expire.
	// Format: RFC3339 timestamp (e.g., "2023-12-31T23:59:59Z")
	// +kubebuilder:validation:Optional
	Expiry *metav1.Time `json:"expiry,omitempty"`

	// Description is a human-readable description for the service account.
	Description string `json:"description,omitempty"`
}

// +kubebuilder:object:root=true

type ServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceAccount `json:"items"`
}

// GetServiceAccountName returns the spec.forProvider.serviceAccountName if given, otherwise defaults to metadata.name.
func (in *ServiceAccount) GetServiceAccountName() string {
	if in.Spec.ForProvider.ServiceAccountName == "" {
		return in.Name
	}
	return in.Spec.ForProvider.ServiceAccountName
}

// GetParentUser returns the parent user for this service account.
func (in *ServiceAccount) GetParentUser() string {
	return in.Spec.ForProvider.ParentUser
}

// Dummy type metadata.
var (
	ServiceAccountKind             = reflect.TypeOf(ServiceAccount{}).Name()
	ServiceAccountGroupKind        = schema.GroupKind{Group: Group, Kind: ServiceAccountKind}.String()
	ServiceAccountKindAPIVersion   = ServiceAccountKind + "." + SchemeGroupVersion.String()
	ServiceAccountGroupVersionKind = SchemeGroupVersion.WithKind(ServiceAccountKind)
)
