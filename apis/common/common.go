/*
Copyright 2023 The Crossplane Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package common contains shared types that are used in multiple CRDs.
// +kubebuilder:object:generate=true
package common

import (
	corev1 "k8s.io/api/core/v1"
)

// TLSConfig represents the TLS configuration for MinIO connections.
type TLSConfig struct {
	// CASecretRef references a Kubernetes Secret or ConfigMap containing the CA certificate.
	// The referenced secret should contain the CA certificate in PEM format.
	// +optional
	CASecretRef *corev1.SecretKeySelector `json:"caSecretRef,omitempty"`

	// ClientCertSecretRef references a Kubernetes Secret containing the client certificate.
	// The referenced secret should contain the client certificate in PEM format.
	// +optional
	ClientCertSecretRef *corev1.SecretKeySelector `json:"clientCertSecretRef,omitempty"`

	// ClientKeySecretRef references a Kubernetes Secret containing the client private key.
	// The referenced secret should contain the client private key in PEM format.
	// +optional
	ClientKeySecretRef *corev1.SecretKeySelector `json:"clientKeySecretRef,omitempty"`

	// InsecureSkipVerify controls whether the client verifies the server's certificate chain and host name.
	// If InsecureSkipVerify is true, crypto/tls accepts any certificate presented by the server
	// and any host name in that certificate. This should be used only for testing.
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}
