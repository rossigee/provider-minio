package serviceaccount

import (
	"context"
	"fmt"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	providerv1 "github.com/vshn/provider-minio/apis/provider/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestConnector_Connect(t *testing.T) {
	tests := []struct {
		name    string
		mg      resource.Managed
		wantErr bool
	}{
		{
			name: "GivenValidServiceAccount_ThenNoError",
			mg: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test-config",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "GivenInvalidManagedResource_ThenError",
			mg:      &miniov1.User{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &connector{
				kube:     &mockKubeClient{},
				recorder: &mockEventRecorder{},
				usage:    &mockUsageTracker{},
			}

			_, err := c.Connect(context.TODO(), tt.mg)
			if (err != nil) != tt.wantErr {
				t.Errorf("connector.Connect() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsServiceAccountNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "GivenNilError_ThenFalse",
			err:  nil,
			want: false,
		},
		{
			name: "GivenDoesNotExistError_ThenTrue",
			err:  fmt.Errorf("service account does not exist"),
			want: true,
		},
		{
			name: "GivenNotFoundError_ThenTrue",
			err:  fmt.Errorf("service account not found"),
			want: true,
		},
		{
			name: "GivenNoSuchServiceAccountError_ThenTrue",
			err:  fmt.Errorf("NoSuchServiceAccount"),
			want: true,
		},
		{
			name: "GivenOtherError_ThenFalse",
			err:  fmt.Errorf("connection refused"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isServiceAccountNotFound(tt.err)
			if got != tt.want {
				t.Errorf("isServiceAccountNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mockKubeClient struct {
	client.Client
}

func (m *mockKubeClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if pc, ok := obj.(*providerv1.ProviderConfig); ok {
		pc.Spec = providerv1.ProviderConfigSpec{
			MinioURL: "http://localhost:9000",
			Credentials: providerv1.ProviderCredentials{
				APISecretRef: corev1.SecretReference{
					Name:      "minio-creds",
					Namespace: "default",
				},
			},
		}
	}
	return nil
}

type mockEventRecorder struct {
	event.Recorder
}

func (m *mockEventRecorder) Event(object runtime.Object, event event.Event) {}

type mockUsageTracker struct {
	resource.Tracker
}

func (m *mockUsageTracker) Track(ctx context.Context, mg resource.Managed) error {
	return nil
}
