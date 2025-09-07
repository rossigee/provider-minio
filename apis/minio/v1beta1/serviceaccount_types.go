package v1beta1

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
// +kubebuilder:printcolumn:name="Access Key",type="string",JSONPath=".status.atProvider.accessKey"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.atProvider.accountStatus"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,minio}
// +kubebuilder:webhook:verbs=create;update,path=/validate-minio-m-crossplane-io-v1beta1-serviceaccount,mutating=false,failurePolicy=fail,groups=minio.m.crossplane.io,resources=serviceaccounts,versions=v1beta1,name=serviceaccounts.minio.m.crossplane.io,sideEffects=None,admissionReviewVersions=v1

// ServiceAccount is a namespaced managed resource that represents a MinIO service account.
// This is the Crossplane v2 namespaced version.
type ServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceAccountSpec   `json:"spec"`
	Status ServiceAccountStatus `json:"status,omitempty"`
}

// ServiceAccountSpec defines the desired state of a ServiceAccount
type ServiceAccountSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       ServiceAccountParameters `json:"forProvider,omitempty"`
}

// ServiceAccountStatus defines the observed state of a ServiceAccount
type ServiceAccountStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ServiceAccountProviderStatus `json:"atProvider,omitempty"`
}

// ServiceAccountProviderStatus defines the observed state of a ServiceAccount from the provider
type ServiceAccountProviderStatus struct {
	// AccessKey is the access key ID of the service account
	AccessKey string `json:"accessKey,omitempty"`
	// AccountStatus indicates the service account's status on the MinIO instance
	AccountStatus string `json:"accountStatus,omitempty"`
	// ParentUser is the user that owns this service account
	ParentUser string `json:"parentUser,omitempty"`
	// ImpliedPolicy indicates if the policy is implied from the parent user
	ImpliedPolicy bool `json:"impliedPolicy,omitempty"`
	// Policy contains the policy document applied to this service account
	Policy string `json:"policy,omitempty"`
	// Expiration shows when this service account expires (if set)
	Expiration *metav1.Time `json:"expiration,omitempty"`
}

// ServiceAccountParameters define the desired state of a MinIO ServiceAccount
type ServiceAccountParameters struct {
	// TargetUser is the user that this service account will belong to.
	// If not specified, the service account will be created for the user
	// making the request (typically from the provider configuration).
	TargetUser string `json:"targetUser,omitempty"`

	// AccessKey is the desired access key for the service account.
	// If not specified, MinIO will generate one automatically.
	// Cannot be changed after service account is created.
	AccessKey string `json:"accessKey,omitempty"`

	// SecretKey is the desired secret key for the service account.
	// If not specified, MinIO will generate one automatically.
	// Cannot be changed after service account is created.
	SecretKey string `json:"secretKey,omitempty"`

	// Name is a human-readable name for this service account
	Name string `json:"name,omitempty"`

	// Description provides additional details about this service account
	Description string `json:"description,omitempty"`

	// Policy is a JSON policy document that defines the permissions
	// for this service account. If not specified, the service account
	// will inherit the policies of the target user.
	Policy string `json:"policy,omitempty"`

	// Expiration defines when this service account should expire.
	// If not specified, the service account will not expire.
	Expiration *metav1.Time `json:"expiration,omitempty"`

	// WriteConnectionSecretsToRef specifies the namespace and name of a
	// Secret to which any connection details for this managed resource should
	// be written. Connection details frequently include the endpoint, username,
	// and password required to connect to the managed resource.
	// This field is planned to be replaced in a future release in favor of
	// PublishConnectionDetailsTo. Currently, both could be set independently
	// and connection details would be published to both without affecting
	// each other.
	WriteConnectionSecretsToRef *xpv1.SecretReference `json:"writeConnectionSecretsToRef,omitempty"`

	// PublishConnectionDetailsTo specifies the connection secret config which
	// contains a name, metadata and a reference to secret store config to
	// which any connection details for this managed resource should be written.
	// Connection details frequently include the endpoint, username,
	// and password required to connect to the managed resource.
	PublishConnectionDetailsTo *xpv1.PublishConnectionDetailsTo `json:"publishConnectionDetailsTo,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceAccountList contains a list of ServiceAccount resources
type ServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceAccount `json:"items"`
}

// GetAccessKey returns the spec.forProvider.accessKey if given, otherwise defaults to metadata.name.
func (in *ServiceAccount) GetAccessKey() string {
	if in.Spec.ForProvider.AccessKey == "" {
		return in.ObjectMeta.Name
	}
	return in.Spec.ForProvider.AccessKey
}

// Dummy type metadata.
var (
	ServiceAccountKind             = reflect.TypeOf(ServiceAccount{}).Name()
	ServiceAccountGroupKind        = schema.GroupKind{Group: Group, Kind: ServiceAccountKind}.String()
	ServiceAccountKindAPIVersion   = ServiceAccountKind + "." + SchemeGroupVersion.String()
	ServiceAccountGroupVersionKind = SchemeGroupVersion.WithKind(ServiceAccountKind)
)
