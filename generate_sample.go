// Regenerates the structured files in samples/.
//
// Only the samples listed in generatedSamples are owned by this generator. The
// remaining files in samples/ are hand-written (they carry explanatory comments
// and inline PEM material that a typed serializer cannot reproduce) and are
// never touched or removed here.
//
// Regenerate with: go generate ./...

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/rossigee/provider-minio/apis"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	providerv1beta1 "github.com/rossigee/provider-minio/apis/provider/v1beta1"
	"github.com/rossigee/provider-minio/operator/minioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	serializerjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

// providerConfigKind is the kind this provider actually serves. Crossplane v2
// CRDs inherit a default of "ClusterProviderConfig" for
// spec.providerConfigRef.kind, but that default only applies when the whole
// providerConfigRef object is absent. Because kind is a required field, an
// explicit value must be emitted here or the API server rejects the sample.
const providerConfigKind = "ProviderConfig"

// Generate the sample files.
//
//go:generate go run generate_sample.go ./samples
func main() {
	failIfError(apis.AddToScheme(scheme.Scheme))
	dir := "./samples"

	generated := []runtime.Object{
		newBucketSample(),
		newPolicySample(),
		newProviderConfigSample(),
		newSecretSample(),
		newUserSample(),
	}

	// os.Create truncates, so existing outputs are simply overwritten. Only the
	// pre-rename files need removing, otherwise a regeneration leaves stale
	// duplicates behind.
	for _, name := range legacyGeneratedSamples {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			log.Fatal(err)
		}
	}
	for _, o := range generated {
		serialize(o, dir)
	}
}

// legacyGeneratedSamples are files this generator emitted before the API group
// moved to minio.m.crossplane.io. They are removed so a regeneration does not
// leave stale duplicates behind. Rename anything listed here rather than
// deleting it by hand.
var legacyGeneratedSamples = []string{
	"minio.crossplane.io_bucket.yaml",
	"minio.crossplane.io_policy.yaml",
	"minio.crossplane.io_providerconfig.yaml",
	"minio.crossplane.io_user.yaml",
}

func newBucketSample() *miniov1beta1.Bucket {
	return &miniov1beta1.Bucket{
		TypeMeta: metav1.TypeMeta{
			APIVersion: miniov1beta1.BucketGroupVersionKind.GroupVersion().String(),
			Kind:       miniov1beta1.BucketKind,
		},
		ObjectMeta: metav1.ObjectMeta{Name: "bucket-local-dev"},
		Spec: miniov1beta1.BucketSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{
					Kind: providerConfigKind,
					Name: "provider-config",
				},
			},
			ForProvider: miniov1beta1.BucketParameters{
				Region: "us-east-1",
			},
		},
	}
}

func newPolicySample() *miniov1beta1.Policy {
	return &miniov1beta1.Policy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: miniov1beta1.PolicyGroupVersionKind.GroupVersion().String(),
			Kind:       miniov1beta1.PolicyKind,
		},
		ObjectMeta: metav1.ObjectMeta{Name: "mypolicy"},
		Spec: miniov1beta1.PolicySpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{
					Kind: providerConfigKind,
					Name: "provider-config",
				},
			},
			ForProvider: miniov1beta1.PolicyParameters{
				AllowBucket: "bucket-local-dev",
			},
		},
	}
}

func newProviderConfigSample() *providerv1beta1.ProviderConfig {
	return &providerv1beta1.ProviderConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: providerv1beta1.ProviderConfigGroupVersionKind.GroupVersion().String(),
			Kind:       providerv1beta1.ProviderConfigKind,
		},
		ObjectMeta: metav1.ObjectMeta{Name: "provider-config"},
		Spec: providerv1beta1.ProviderConfigSpec{
			MinioURL: "http://minio.127.0.0.1.nip.io:8088/",
			Credentials: providerv1beta1.ProviderCredentials{
				Source: xpv1.CredentialsSourceInjectedIdentity,
				APISecretRef: corev1.SecretReference{
					Name:      "minio-secret",
					Namespace: "crossplane-system",
				},
			},
		},
	}
}

func newSecretSample() *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "minio-secret",
			Namespace: "crossplane-system",
		},
		Data: map[string][]byte{
			minioutil.MinioIDKey:     []byte("minioadmin"),
			minioutil.MinioSecretKey: []byte("minioadmin"),
		},
	}
}

func newUserSample() *miniov1beta1.User {
	return &miniov1beta1.User{
		TypeMeta: metav1.TypeMeta{
			APIVersion: miniov1beta1.UserGroupVersionKind.GroupVersion().String(),
			Kind:       miniov1beta1.UserKind,
		},
		ObjectMeta: metav1.ObjectMeta{Name: "devuser"},
		Spec: miniov1beta1.UserSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{
					Kind: providerConfigKind,
					Name: "provider-config",
				},
				WriteConnectionSecretToReference: &xpv1.LocalSecretReference{
					Name: "devuser",
				},
			},
		},
	}
}

func failIfError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func fileName(object runtime.Object) string {
	gvk := object.GetObjectKind().GroupVersionKind()
	return fmt.Sprintf("%s_%s.yaml", strings.ToLower(gvk.Group), strings.ToLower(gvk.Kind))
}

func serialize(object runtime.Object, dir string) {
	f, err := os.Create(filepath.Join(dir, fileName(object)))
	failIfError(err)

	serializer := serializerjson.NewSerializerWithOptions(
		serializerjson.DefaultMetaFactory,
		scheme.Scheme,
		scheme.Scheme,
		serializerjson.SerializerOptions{Yaml: true, Pretty: true},
	)
	if err := serializer.Encode(object, f); err != nil {
		_ = f.Close()
		log.Fatal(err)
	}
	// Closed explicitly rather than deferred so that a write error surfaced at
	// close time is reported instead of being discarded.
	failIfError(f.Close())
}
