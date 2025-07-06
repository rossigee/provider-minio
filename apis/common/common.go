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
	// CAData contains the CA certificate data in PEM format for verifying the server's certificate.
	// This is useful for self-signed certificates or private CA certificates.
	// +optional
	CAData string `json:"caData,omitempty"`

	// CASecretRef references a Secret containing the CA certificate data.
	// The Secret must contain a key named 'ca.crt' or 'tls.crt' with the CA certificate data.
	// +optional
	CASecretRef *corev1.SecretKeySelector `json:"caSecretRef,omitempty"`

	// CAConfigMapRef references a ConfigMap containing the CA certificate data.
	// The ConfigMap must contain a key with the CA certificate data.
	// +optional
	CAConfigMapRef *corev1.ConfigMapKeySelector `json:"caConfigMapRef,omitempty"`

	// ClientCertData contains the client certificate data in PEM format for mutual TLS authentication.
	// +optional
	ClientCertData string `json:"clientCertData,omitempty"`

	// ClientCertSecretRef references a Secret containing the client certificate data.
	// The Secret must contain a key with the client certificate data.
	// +optional
	ClientCertSecretRef *corev1.SecretKeySelector `json:"clientCertSecretRef,omitempty"`

	// ClientKeyData contains the client private key data in PEM format for mutual TLS authentication.
	// DEPRECATED: Use ClientKeySecretRef instead. Private keys should not be stored in CRDs.
	// +optional
	ClientKeyData string `json:"clientKeyData,omitempty"`

	// ClientKeySecretRef references a Secret containing the client private key data.
	// The Secret must contain a key with the client private key data.
	// +optional
	ClientKeySecretRef *corev1.SecretKeySelector `json:"clientKeySecretRef,omitempty"`

	// InsecureSkipVerify controls whether the client verifies the server's certificate chain and host name.
	// If InsecureSkipVerify is true, crypto/tls accepts any certificate presented by the server
	// and any host name in that certificate. This should be used only for testing.
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

