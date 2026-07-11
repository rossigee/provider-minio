// +kubebuilder:object:generate=true
// +groupName=minio.crossplane.io
// +versionName=v1

// Package v1 contains the core resources of the provider-minio.
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Package type metadata.
const (
	Group   = "minio.crossplane.io"
	Version = "v1"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	// TODO: migrate to runtime.NewSchemeBuilder (controller-runtime scheme.Builder is deprecated).
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
)

func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(SchemeGroupVersion,
		&ProviderConfigUsage{},
		&ProviderConfigUsageList{},
		&ProviderConfig{},
		&ProviderConfigList{},
	)
	metav1.AddToGroupVersion(s, SchemeGroupVersion)
	return nil
}
