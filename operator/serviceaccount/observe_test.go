package serviceaccount

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/minio/madmin-go/v3"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceAccountClient_IsUpToDate(t *testing.T) {
	client := &serviceAccountClient{}

	tests := []struct {
		name           string
		serviceAccount *miniov1beta1.ServiceAccount
		info           madmin.InfoServiceAccountResp
		expected       bool
	}{
		{
			name: "Policies match - up to date",
			serviceAccount: &miniov1beta1.ServiceAccount{
				Spec: miniov1beta1.ServiceAccountSpec{
					ForProvider: miniov1beta1.ServiceAccountParameters{
						Policy: `{"Version":"2012-10-17","Statement":[]}`,
					},
				},
			},
			info: madmin.InfoServiceAccountResp{
				Policy: `{"Version":"2012-10-17","Statement":[]}`,
			},
			expected: true,
		},
		{
			name: "Policies don't match - needs update",
			serviceAccount: &miniov1beta1.ServiceAccount{
				Spec: miniov1beta1.ServiceAccountSpec{
					ForProvider: miniov1beta1.ServiceAccountParameters{
						Policy: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":["*"]}]}`,
					},
				},
			},
			info: madmin.InfoServiceAccountResp{
				Policy: `{"Version":"2012-10-17","Statement":[]}`,
			},
			expected: false,
		},
		{
			name: "No policy specified - up to date",
			serviceAccount: &miniov1beta1.ServiceAccount{
				Spec: miniov1beta1.ServiceAccountSpec{
					ForProvider: miniov1beta1.ServiceAccountParameters{},
				},
			},
			info: madmin.InfoServiceAccountResp{
				Policy: `{"Version":"2012-10-17","Statement":[]}`,
			},
			expected: true,
		},
		{
			name: "Expiration matches - up to date",
			serviceAccount: &miniov1beta1.ServiceAccount{
				Spec: miniov1beta1.ServiceAccountSpec{
					ForProvider: miniov1beta1.ServiceAccountParameters{
						Expiration: &metav1.Time{Time: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)},
					},
				},
			},
			info: madmin.InfoServiceAccountResp{
				Expiration: &time.Time{},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handle expiration time for the test case
			if tt.name == "Expiration matches - up to date" {
				expectedTime := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
				tt.info.Expiration = &expectedTime
			}

			result := client.isUpToDate(tt.serviceAccount, tt.info)
			assert.Equal(t, tt.expected, result)
		})
	}
}

}

// TestServiceAccountSpec_ConnectionSecretReferenceIsInherited guards the fix for
// the field-shadowing defect.
//
// ServiceAccountSpec used to re-declare WriteConnectionSecretToRef with the same
// JSON tag as the field inherited from the embedded ManagedResourceSpec. Go's
// JSON decoder resolves the duplicate tag to the shallower (outer) field, while
// the generated GetWriteConnectionSecretToReference accessor reads the embedded
// field. A manifest applied from YAML therefore populated a field nothing read,
// the accessor returned nil, and the managed reconciler never wrote a connection
// secret for any ServiceAccount.
func TestServiceAccountSpec_ConnectionSecretReferenceIsInherited(t *testing.T) {
	const manifest = `{"metadata":{"name":"sa","namespace":"default"},` +
		`"spec":{"writeConnectionSecretToRef":{"name":"conn"}}}`

	sa := &miniov1beta1.ServiceAccount{}
	require.NoError(t, json.Unmarshal([]byte(manifest), sa),
		"a manifest setting writeConnectionSecretToRef must apply cleanly")

	got := sa.GetWriteConnectionSecretToReference()
	require.NotNil(t, got,
		"the connection secret reference set from a manifest must be readable, "+
			"otherwise the reconciler silently skips writing the secret")
	assert.Equal(t, "conn", got.Name)
}

// TestPoliciesEqual covers the MinIO re-serialisation problem: the policy stored
// by MinIO is pretty-printed, so comparing it to the spec as a raw string never
// matches and the controller updates the resource on every reconcile.
func TestPoliciesEqual(t *testing.T) {
	compact := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::b/*"]}]}`
	pretty := `{
  "Version": "2012-10-17",
  "Statement": [
   {
    "Effect": "Allow",
    "Action": [
     "s3:GetObject"
    ],
    "Resource": [
     "arn:aws:s3:::b/*"
    ]
   }
  ]
}`

	assert.True(t, policiesEqual(compact, pretty), "reformatted but equivalent policies must compare equal")
	assert.True(t, policiesEqual(compact, compact))
	assert.False(t, policiesEqual(compact, `{"Version":"2012-10-17","Statement":[]}`))
	assert.False(t, policiesEqual("not json", pretty))
}

// TestServiceAccountClientObserve_ConnectionSecretMissing covers the case where a
// service account already exists in MinIO but its connection secret is gone.
//
// MinIO stores secret keys hashed and never returns them, so the credentials
// written at creation time are the only copy that will ever exist. The provider
// used to log this at debug level and leave the resource reporting
// Ready/Available, which hid the fact that the credentials were gone.
func TestServiceAccountClientObserve_ConnectionSecretMissing(t *testing.T) {
	newAccount := func(externalName string) *miniov1beta1.ServiceAccount {
		sa := &miniov1beta1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "example",
				Namespace:   "default",
				Annotations: map[string]string{},
			},
		}
		if externalName != "" {
			meta.SetExternalName(sa, externalName)
		}
		sa.Spec.WriteConnectionSecretToReference = &xpv1.LocalSecretReference{
			Name: "example-conn",
		}
		return sa
	}

	t.Run("ExistingAccountWithMissingSecretIsReported", func(t *testing.T) {
		ma := newFakeSAAdmin()
		ma.accounts["EXAMPLEKEY"] = madmin.InfoServiceAccountResp{AccountStatus: "on"}
		sa := newTestSAClient(t, ma)

		_, err := sa.Observe(context.Background(), newAccount("EXAMPLEKEY"))

		require.Error(t, err, "a missing connection secret must surface as an error")
		assert.ErrorIs(t, err, errConnectionSecretUnrecoverable)
	})

	t.Run("NotYetCreatedDoesNotReportMissingSecret", func(t *testing.T) {
		ma := newFakeSAAdmin()
		sa := newTestSAClient(t, ma)

		// No external-name, so Create has not run yet and the absence of the
		// connection secret is expected rather than a lost credential.
		_, err := sa.Observe(context.Background(), newAccount(""))

		require.NoError(t, err)
	})

	t.Run("ExistingAccountWithSecretIsHealthy", func(t *testing.T) {
		ma := newFakeSAAdmin()
		ma.accounts["EXAMPLEKEY"] = madmin.InfoServiceAccountResp{AccountStatus: "on"}
		sa := newTestSAClient(t, ma)

		conn := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "example-conn", Namespace: "default"},
			Data:       map[string][]byte{AccessKeyName: []byte("EXAMPLEKEY")},
		}
		require.NoError(t, sa.kube.Create(context.Background(), conn))

		_, err := sa.Observe(context.Background(), newAccount("EXAMPLEKEY"))

		require.NoError(t, err)
	})
}
