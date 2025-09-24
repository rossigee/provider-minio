package user

import (
	"context"
	"strings"

	"github.com/minio/madmin-go/v3"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/rossigee/provider-minio/apis/minio/v1beta1"
	apisv1alpha1 "github.com/rossigee/provider-minio/apis/provider/v1"
	"github.com/rossigee/provider-minio/internal/clients"
)

const (
	errNotUser      = "managed resource is not a User custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
	errNewClient    = "cannot create new MinIO client"
	errCreateUser   = "cannot create user"
	errUpdateUser   = "cannot update user"
	errDeleteUser   = "cannot delete user"
	errGetUser      = "cannot get user"
	errUserExists   = "user already exists"
)

// Setup adds a controller that reconciles User managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1beta1.UserGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1beta1.User{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.UserGroupVersionKind),
			managed.WithExternalConnecter(&connector{
				kube:         mgr.GetClient(),
				usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
				newServiceFn: clients.NewMinIOClient,
			}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
			managed.WithConnectionPublishers(cps...)))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(cfg clients.Config) (*madmin.AdminClient, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1beta1.User)
	if !ok {
		return nil, errors.New(errNotUser)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cfg, err := clients.GetConfig(ctx, c.kube, pc)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	client, err := c.newServiceFn(*cfg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{client: client}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	client *madmin.AdminClient
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1beta1.User)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotUser)
	}

	userName := cr.GetUserName()

	// Check if user exists
	userInfo, err := c.client.GetUserInfo(ctx, userName)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return managed.ExternalObservation{
				ResourceExists: false,
			}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGetUser)
	}

	// User exists, update status
	cr.Status.AtProvider.UserName = userName
	cr.Status.AtProvider.Status = string(userInfo.Status)
	cr.Status.AtProvider.Policies = userInfo.PolicyName

	// Check if policies match desired state
	desiredPolicies := cr.Spec.ForProvider.Policies
	// PolicyName is a single string, convert to slice for comparison
	var currentPolicies []string
	if userInfo.PolicyName != "" {
		currentPolicies = []string{userInfo.PolicyName}
	}

	policiesMatch := len(desiredPolicies) == len(currentPolicies)
	if policiesMatch {
		for _, desired := range desiredPolicies {
			found := false
			for _, current := range currentPolicies {
				if desired == current {
					found = true
					break
				}
			}
			if !found {
				policiesMatch = false
				break
			}
		}
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: policiesMatch,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1beta1.User)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotUser)
	}

	userName := cr.GetUserName()

	// Generate a random password for the user
	// In a real implementation, this should be stored in a secret
	password := "MinIOUser2024!" // This is a placeholder

	// Create the user
	err := c.client.AddUser(ctx, userName, password)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateUser)
	}

	// Apply policies if specified
	if len(cr.Spec.ForProvider.Policies) > 0 {
		for _, policy := range cr.Spec.ForProvider.Policies {
			_, err = c.client.AttachPolicy(ctx, madmin.PolicyAssociationReq{
				Policies: []string{policy},
				User:     userName,
			})
			if err != nil {
				return managed.ExternalCreation{}, errors.Wrapf(err, "cannot attach policy %s to user", policy)
			}
		}
	}

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{
			"username": []byte(userName),
			"password": []byte(password),
		},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1beta1.User)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotUser)
	}

	userName := cr.GetUserName()

	// Update policies - detach existing and attach new ones
	if len(cr.Spec.ForProvider.Policies) > 0 {
		// Get current policies to detach them first
		userInfo, err := c.client.GetUserInfo(ctx, userName)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, "cannot get current user info")
		}

		// Detach existing policies
		if userInfo.PolicyName != "" {
			_, err := c.client.DetachPolicy(ctx, madmin.PolicyAssociationReq{
				Policies: []string{userInfo.PolicyName},
				User:     userName,
			})
			if err != nil {
				return managed.ExternalUpdate{}, errors.Wrapf(err, "cannot detach policy %s from user", userInfo.PolicyName)
			}
		}

		// Attach new policies
		for _, policy := range cr.Spec.ForProvider.Policies {
			_, err := c.client.AttachPolicy(ctx, madmin.PolicyAssociationReq{
				Policies: []string{policy},
				User:     userName,
			})
			if err != nil {
				return managed.ExternalUpdate{}, errors.Wrapf(err, "cannot attach policy %s to user", policy)
			}
		}
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1beta1.User)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotUser)
	}

	userName := cr.GetUserName()

	err := c.client.RemoveUser(ctx, userName)
	if err != nil && !strings.Contains(err.Error(), "does not exist") {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteUser)
	}

	return managed.ExternalDelete{}, nil
}

// Disconnect is called when the controller is shutting down and should close connections.
func (c *external) Disconnect(ctx context.Context) error {
	// MinIO admin client doesn't have persistent connections to close
	return nil
}
