// +kubebuilder:object:generate=true
// +groupName=minio.m.crossplane.io
// +versionName=v1beta1

// Package v1beta1 contains the v1beta1 group minio.m.crossplane.io resources of provider-minio.
// This is the namespaced version of the minio provider following Crossplane v2 patterns.
package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// Package type metadata.
const (
	Group   = "minio.m.crossplane.io"
	Version = "v1beta1"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)