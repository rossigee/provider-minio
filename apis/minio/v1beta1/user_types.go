package v1beta1

import (
	"reflect"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="External Name",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="Policies",type="string",JSONPath=".status.atProvider.policies"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,minio}
// +kubebuilder:webhook:verbs=create;update,path=/validate-minio-m-crossplane-io-v1beta1-user,mutating=false,failurePolicy=fail,groups=minio.m.crossplane.io,resources=users,versions=v1beta1,name=users.minio.m.crossplane.io,sideEffects=None,admissionReviewVersions=v1

// User is a namespaced managed resource that represents a MinIO user.
// This is the Crossplane v2 namespaced version.
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec"`
	Status UserStatus `json:"status,omitempty"`
}

// UserSpec defines the desired state of a User
type UserSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       UserParameters `json:"forProvider,omitempty"`
}

// UserStatus defines the observed state of a User
type UserStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          UserProviderStatus `json:"atProvider,omitempty"`
}

// UserProviderStatus defines the observed state of a User from the provider
type UserProviderStatus struct {
	// UserName is populated if the user actually exists in minio during observe.
	UserName string `json:"userName,omitempty"`
	// Status indicates the user's status on the minio instance.
	Status string `json:"status,omitempty"`

	// Policies contains a list of policies that are applied to this user
	Policies string `json:"policies,omitempty"`
}

// UserParameters define the desired state of a MinIO User
type UserParameters struct {
	// UserName is the name of the user to create.
	// Defaults to `metadata.name` if unset.
	// Cannot be changed after user is created.
	UserName string `json:"userName,omitempty"`

	// Policies contains a list of policies that should get assigned to this user.
	// These policies need to be created separately by using the policy CRD.
	Policies []string `json:"policies,omitempty"`
}

// +kubebuilder:object:root=true

// UserList contains a list of User resources
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

// GetUserName returns the spec.forProvider.userName if given, otherwise defaults to metadata.name.
func (in *User) GetUserName() string {
	if in.Spec.ForProvider.UserName == "" {
		return in.Name
	}
	return in.Spec.ForProvider.UserName
}

// Dummy type metadata.
var (
	UserKind             = reflect.TypeOf(User{}).Name()
	UserGroupKind        = schema.GroupKind{Group: Group, Kind: UserKind}.String()
	UserKindAPIVersion   = UserKind + "." + SchemeGroupVersion.String()
	UserGroupVersionKind = SchemeGroupVersion.WithKind(UserKind)
)
