package serviceaccount

import (
	"context"
	"testing"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/go-logr/logr"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		obj     *miniov1.ServiceAccount
		wantErr bool
	}{
		{
			name: "GivenValidObject_ThenNoError",
			obj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "admin",
					},
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GivenMissingParentUser_ThenError",
			obj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GivenMissingProviderConfigRef_ThenNoError",
			obj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "admin",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GivenValidObjectWithPolicies_ThenNoError",
			obj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "admin",
						Policies:   []string{"readwrite", "diagnostics"},
					},
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GivenValidObjectWithExpiry_ThenNoError",
			obj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "admin",
						Expiry:     &metav1.Time{Time: mustParseTime("2025-12-31T23:59:59Z")},
					},
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{
				log:  logr.Discard(),
				kube: &mockClient{},
			}
			_, err := v.ValidateCreate(context.TODO(), tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validator.ValidateCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		oldObj  *miniov1.ServiceAccount
		newObj  *miniov1.ServiceAccount
		wantErr bool
	}{
		{
			name: "GivenSameParentUser_ThenNoError",
			oldObj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "admin",
					},
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test",
						},
					},
				},
			},
			newObj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "admin",
						Policies:   []string{"readwrite"},
					},
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GivenDifferentParentUser_ThenError",
			oldObj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "admin",
					},
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test",
						},
					},
				},
			},
			newObj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "different-user",
					},
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GivenPolicyUpdate_ThenNoError",
			oldObj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "admin",
						Policies:   []string{"readonly"},
					},
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test",
						},
					},
				},
			},
			newObj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser: "admin",
						Policies:   []string{"readwrite"},
					},
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GivenDescriptionUpdate_ThenNoError",
			oldObj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser:  "admin",
						Description: "old description",
					},
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test",
						},
					},
				},
			},
			newObj: &miniov1.ServiceAccount{
				Spec: miniov1.ServiceAccountSpec{
					ForProvider: miniov1.ServiceAccountParameters{
						ParentUser:  "admin",
						Description: "new description",
					},
					ResourceSpec: xpv1.ResourceSpec{
						ProviderConfigReference: &xpv1.Reference{
							Name: "test",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{
				log:  logr.Discard(),
				kube: &mockClient{},
			}
			_, err := v.ValidateUpdate(context.TODO(), tt.oldObj, tt.newObj)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validator.ValidateUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidateDelete(t *testing.T) {
	v := &Validator{
		log:  logr.Discard(),
		kube: &mockClient{},
	}

	obj := &miniov1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sa",
		},
	}

	_, err := v.ValidateDelete(context.TODO(), obj)
	if err != nil {
		t.Errorf("Validator.ValidateDelete() should never return error, got: %v", err)
	}
}

type mockClient struct {
	client.Client
}

func (m *mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return nil
}

func mustParseTime(timeStr string) time.Time {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		panic(err)
	}
	return t
}
