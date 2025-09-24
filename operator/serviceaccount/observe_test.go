package serviceaccount

import (
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
	"testing"
	"time"

	"github.com/minio/madmin-go/v3"
	"github.com/stretchr/testify/assert"
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
