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

// TLSConfig represents the TLS configuration for MinIO connections.
type TLSConfig struct {
	// CAData contains the CA certificate data in PEM format for verifying the server's certificate.
	// This is useful for self-signed certificates or private CA certificates.
	CAData string `json:"caData,omitempty"`

	// ClientCertData contains the client certificate data in PEM format for mutual TLS authentication.
	ClientCertData string `json:"clientCertData,omitempty"`

	// ClientKeyData contains the client private key data in PEM format for mutual TLS authentication.
	ClientKeyData string `json:"clientKeyData,omitempty"`

	// InsecureSkipVerify controls whether the client verifies the server's certificate chain and host name.
	// If InsecureSkipVerify is true, crypto/tls accepts any certificate presented by the server
	// and any host name in that certificate. This should be used only for testing.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

