package v1beta1

import (
	"reflect"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func init() {
	SchemeBuilder.Register(&NotificationConfiguration{}, &NotificationConfigurationList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="External Name",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Bucket",type="string",JSONPath=".spec.forProvider.bucketName"
// +kubebuilder:printcolumn:name="Target",type="string",JSONPath=".spec.forProvider.webhookConfiguration.endpoint"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,minio}
// +kubebuilder:webhook:verbs=create;update,path=/validate-minio-m-crossplane-io-v1beta1-notificationconfiguration,mutating=false,failurePolicy=fail,groups=minio.m.crossplane.io,resources=notificationconfigurations,versions=v1beta1,name=notificationconfigurations.minio.m.crossplane.io,sideEffects=None,admissionReviewVersions=v1

// NotificationConfiguration is a namespaced managed resource that represents a MinIO bucket notification configuration.
type NotificationConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NotificationConfigurationSpec   `json:"spec"`
	Status NotificationConfigurationStatus `json:"status,omitempty"`
}

// NotificationConfigurationSpec defines the desired state of a NotificationConfiguration
type NotificationConfigurationSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       NotificationConfigurationParameters `json:"forProvider,omitempty"`
}

// NotificationConfigurationStatus defines the observed state of a NotificationConfiguration
type NotificationConfigurationStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          NotificationConfigurationProviderStatus `json:"atProvider,omitempty"`
}

// NotificationConfigurationParameters define the desired state of a MinIO notification configuration
type NotificationConfigurationParameters struct {
	// BucketName is the name of the bucket to configure notifications for.
	// +kubebuilder:validation:Required
	BucketName string `json:"bucketName"`

	// WebhookConfiguration defines webhook notification settings
	WebhookConfiguration *WebhookConfiguration `json:"webhookConfiguration,omitempty"`

	// QueueConfiguration defines SQS notification settings
	QueueConfiguration *QueueConfiguration `json:"queueConfiguration,omitempty"`

	// TopicConfiguration defines SNS notification settings
	TopicConfiguration *TopicConfiguration `json:"topicConfiguration,omitempty"`

	// Events is the list of S3 events to notify on
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Events []string `json:"events"`

	// Filter specifies object key name filtering rules
	Filter *NotificationFilter `json:"filter,omitempty"`
}

// WebhookConfiguration defines webhook notification settings
type WebhookConfiguration struct {
	// ID is the unique identifier for this webhook configuration
	// +kubebuilder:validation:Required
	ID string `json:"id"`

	// Endpoint is the webhook URL to send notifications to
	// +kubebuilder:validation:Required
	Endpoint string `json:"endpoint"`

	// AuthToken is an optional authentication token for the webhook
	AuthToken *string `json:"authToken,omitempty"`

	// UserAgent is an optional custom user agent string
	UserAgent *string `json:"userAgent,omitempty"`
}

// QueueConfiguration defines SQS notification settings
type QueueConfiguration struct {
	// ID is the unique identifier for this queue configuration
	// +kubebuilder:validation:Required
	ID string `json:"id"`

	// QueueArn is the ARN of the SQS queue
	// +kubebuilder:validation:Required
	QueueArn string `json:"queueArn"`
}

// TopicConfiguration defines SNS notification settings
type TopicConfiguration struct {
	// ID is the unique identifier for this topic configuration
	// +kubebuilder:validation:Required
	ID string `json:"id"`

	// TopicArn is the ARN of the SNS topic
	// +kubebuilder:validation:Required
	TopicArn string `json:"topicArn"`
}

// NotificationFilter specifies object key name filtering rules
type NotificationFilter struct {
	// Key specifies object key name filtering rules
	Key *KeyFilter `json:"key,omitempty"`
}

// KeyFilter specifies object key name filtering rules
type KeyFilter struct {
	// FilterRules is the list of filter rules
	FilterRules []FilterRule `json:"filterRules,omitempty"`
}

// FilterRule specifies a single filter rule
type FilterRule struct {
	// Name is the filter rule name (prefix or suffix)
	// +kubebuilder:validation:Enum=prefix;suffix
	Name string `json:"name"`

	// Value is the filter rule value
	Value string `json:"value"`
}

// NotificationConfigurationProviderStatus defines the observed state from the provider
type NotificationConfigurationProviderStatus struct {
	// ConfigurationID is the actual configuration ID created in MinIO
	ConfigurationID string `json:"configurationId,omitempty"`

	// BucketName is the name of the bucket this configuration applies to
	BucketName string `json:"bucketName,omitempty"`

	// LastUpdated is the timestamp when the configuration was last updated
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true

// NotificationConfigurationList contains a list of NotificationConfiguration resources
type NotificationConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NotificationConfiguration `json:"items"`
}

// Dummy type metadata.
var (
	NotificationConfigurationKind             = reflect.TypeOf(NotificationConfiguration{}).Name()
	NotificationConfigurationGroupKind        = schema.GroupKind{Group: Group, Kind: NotificationConfigurationKind}.String()
	NotificationConfigurationKindAPIVersion   = NotificationConfigurationKind + "." + SchemeGroupVersion.String()
	NotificationConfigurationGroupVersionKind = SchemeGroupVersion.WithKind(NotificationConfigurationKind)
)