/*
Copyright 2025 The Crossplane Authors.

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

package v1beta1

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Bucket type metadata.
var (
	BucketKind             = reflect.TypeOf(Bucket{}).Name()
	BucketGroupKind        = schema.GroupKind{Group: Group, Kind: BucketKind}.String()
	BucketKindAPIVersion   = BucketKind + "." + SchemeGroupVersion.String()
	BucketGroupVersionKind = SchemeGroupVersion.WithKind(BucketKind)
)



// User type metadata.
var (
	UserKind             = reflect.TypeOf(User{}).Name()
	UserGroupKind        = schema.GroupKind{Group: Group, Kind: UserKind}.String()
	UserKindAPIVersion   = UserKind + "." + SchemeGroupVersion.String()
	UserGroupVersionKind = SchemeGroupVersion.WithKind(UserKind)
)

// ServiceAccount type metadata.
var (
	ServiceAccountKind             = reflect.TypeOf(ServiceAccount{}).Name()
	ServiceAccountGroupKind        = schema.GroupKind{Group: Group, Kind: ServiceAccountKind}.String()
	ServiceAccountKindAPIVersion   = ServiceAccountKind + "." + SchemeGroupVersion.String()
	ServiceAccountGroupVersionKind = SchemeGroupVersion.WithKind(ServiceAccountKind)
)

// NotificationConfiguration type metadata.
var (
	NotificationConfigurationKind             = reflect.TypeOf(NotificationConfiguration{}).Name()
	NotificationConfigurationGroupKind        = schema.GroupKind{Group: Group, Kind: NotificationConfigurationKind}.String()
	NotificationConfigurationKindAPIVersion   = NotificationConfigurationKind + "." + SchemeGroupVersion.String()
	NotificationConfigurationGroupVersionKind = SchemeGroupVersion.WithKind(NotificationConfigurationKind)
)

// Policy type metadata.
var (
	PolicyKind             = reflect.TypeOf(Policy{}).Name()
	PolicyGroupKind        = schema.GroupKind{Group: Group, Kind: PolicyKind}.String()
	PolicyKindAPIVersion   = PolicyKind + "." + SchemeGroupVersion.String()
	PolicyGroupVersionKind = SchemeGroupVersion.WithKind(PolicyKind)
)

// BucketClaim type metadata.
var (
	BucketClaimKind             = reflect.TypeOf(BucketClaim{}).Name()
	BucketClaimGroupKind        = schema.GroupKind{Group: Group, Kind: BucketClaimKind}.String()
	BucketClaimKindAPIVersion   = BucketClaimKind + "." + SchemeGroupVersion.String()
	BucketClaimGroupVersionKind = SchemeGroupVersion.WithKind(BucketClaimKind)
)
