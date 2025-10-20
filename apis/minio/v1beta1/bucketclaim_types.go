package v1beta1

import (
	"reflect"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func init() {
	SchemeBuilder.Register(&BucketClaim{}, &BucketClaimList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="External Name",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Bucket Name",type="string",JSONPath=".spec.bucketName"
// +kubebuilder:printcolumn:name="Region",type="string",JSONPath=".spec.region"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,minio}
// +kubebuilder:webhook:verbs=create;update,path=/validate-minio-m-crossplane-io-v1beta1-bucketclaim,mutating=false,failurePolicy=fail,groups=minio.m.crossplane.io,resources=bucketclaims,versions=v1beta1,name=bucketclaims.minio.m.crossplane.io,sideEffects=None,admissionReviewVersions=v1

// BucketClaim is a namespaced managed resource that represents a claim for a MinIO bucket.
// This allows XRD compositions to create buckets with direct APISecretRef authentication.
type BucketClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketClaimSpec   `json:"spec"`
	Status BucketClaimStatus `json:"status,omitempty"`
}

// BucketClaimSpec defines the desired state of a BucketClaim
type BucketClaimSpec struct {
	xpv1.ResourceSpec `json:",inline"`

	// CredentialsSecretRef specifies the secret containing MinIO credentials
	CredentialsSecretRef *xpv1.SecretReference `json:"credentialsSecretRef,omitempty"`

	// BucketName is the name of the bucket to create.
	// Defaults to `metadata.name` if unset.
	BucketName string `json:"bucketName,omitempty"`

	// Region is the name of the region where the bucket shall be created.
	// Defaults to us-east-1 if unset.
	Region string `json:"region,omitempty"`

	// BucketDeletionPolicy determines how buckets should be deleted when BucketClaim is deleted.
	// `DeleteIfEmpty` only deletes the bucket if the bucket is empty.
	// `DeleteAll` recursively deletes all objects in the bucket and then removes it.
	BucketDeletionPolicy BucketDeletionPolicy `json:"bucketDeletionPolicy,omitempty"`

	// Policy is a raw S3 bucket policy.
	Policy *string `json:"policy,omitempty"`
}

// BucketClaimStatus defines the observed state of a BucketClaim
type BucketClaimStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// Endpoint is the MinIO endpoint URL
	Endpoint string `json:"endpoint,omitempty"`

	// AtProvider contains the observed state from the provider
	AtProvider BucketClaimProviderStatus `json:"atProvider,omitempty"`
}

// BucketClaimProviderStatus defines the observed state of a BucketClaim from the provider
type BucketClaimProviderStatus struct {
	// BucketName is the name of the actual bucket.
	BucketName string `json:"bucketName,omitempty"`
}

// +kubebuilder:object:root=true

// BucketClaimList contains a list of BucketClaim resources
type BucketClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BucketClaim `json:"items"`
}

// Dummy type metadata.
var (
	BucketClaimKind             = reflect.TypeOf(BucketClaim{}).Name()
	BucketClaimGroupKind        = schema.GroupKind{Group: Group, Kind: BucketClaimKind}.String()
	BucketClaimKindAPIVersion   = BucketClaimKind + "." + SchemeGroupVersion.String()
	BucketClaimGroupVersionKind = SchemeGroupVersion.WithKind(BucketClaimKind)
)

// GetBucketName returns the spec.bucketName if given, otherwise defaults to metadata.name.
func (in *BucketClaim) GetBucketName() string {
	if in.Spec.BucketName == "" {
		return in.GetName()
	}
	return in.Spec.BucketName
}