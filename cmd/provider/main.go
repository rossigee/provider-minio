package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/rossigee/provider-minio/apis"
	"github.com/rossigee/provider-minio/internal/tracing"
	"github.com/rossigee/provider-minio/operator"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/utils/ptr"
	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func main() {
	var (
		app                      = kingpin.New(filepath.Base(os.Args[0]), "Crossplane provider for MinIO.").DefaultEnvars()
		debug                    = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		syncPeriod               = app.Flag("sync", "Controller manager sync period such as 300ms, 1.5h, or 2h45m").Short('s').Default("1h").Duration()
		pollInt                  = app.Flag("poll", "Poll interval controls how often an individual resource should be checked for drift.").Default("10m").Duration()
		leaderElect              = app.Flag("leader-elect", "Use leader election for the controller manager.").Short('l').Default("false").Bool()
		maxReconcileRate         = app.Flag("max-reconcile-rate", "The global maximum rate per second at which resources may checked for drift from the desired state.").Default("10").Int()
		enableManagementPolicies = app.Flag("enable-management-policies", "Enable support for Management Policies.").Default("false").Bool()
	)

	kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := zap.New(zap.UseDevMode(*debug))
	log := logging.NewLogrLogger(zl.WithName("provider-minio"))

	shutdownTracing := tracing.Init("provider-minio")
	defer shutdownTracing(context.Background())

	// Always set the controller-runtime logger to prevent stacktraces
	// This must be called before any controller-runtime operations
	ctrl.SetLogger(zl)

	log.Debug("Starting", "sync-period", syncPeriod.String())

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")

	// Create a scheme with both k8s core types and custom minio types to avoid
	// scheme conversion errors when the cache initializes informers.
	// This ensures ListOptions and other k8s types can be properly converted.
	s := runtime.NewScheme()
	kingpin.FatalIfError(scheme.AddToScheme(s), "Cannot add k8s types to scheme")
	// IMPORTANT: Add custom APIs to the scheme BEFORE creating the manager,
	// so the cache initializes with all necessary types registered.
	kingpin.FatalIfError(apis.AddToScheme(s), "Cannot add MinIO APIs to scheme")

	// Use cert-manager issued certificate for webhook server
	log.Info("Using cert-manager issued certificate for webhook server")

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:           s,
		LeaderElection:   *leaderElect,
		LeaderElectionID: "crossplane-leader-election-provider-minio",
		Cache: cache.Options{
			SyncPeriod:                  syncPeriod,
			DefaultEnableWatchBookmarks: ptr.To(true),
		},
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaseDuration:              func() *time.Duration { d := 60 * time.Second; return &d }(),
		RenewDeadline:              func() *time.Duration { d := 50 * time.Second; return &d }(),
		WebhookServer: &webhook.DefaultServer{Options: webhook.Options{
			Port:    9443,
			CertDir: os.Getenv("WEBHOOK_TLS_CERT_DIR"),
		}},
	})
	kingpin.FatalIfError(err, "Cannot create controller manager")

	if err := setupRBAC(mgr.GetClient(), log); err != nil {
		log.Info("RBAC setup warning (may be transient)", "error", err)
	}

	o := controller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: *maxReconcileRate,
		PollInterval:            *pollInt,
		GlobalRateLimiter:       ratelimiter.NewGlobal(*maxReconcileRate),
		Features:                &feature.Flags{},
	}

	if *enableManagementPolicies {
		o.Features.Enable(feature.EnableBetaManagementPolicies)
		log.Info("Beta feature enabled", "flag", feature.EnableBetaManagementPolicies)
	}

	kingpin.FatalIfError(operator.SetupControllers(mgr), "Cannot setup MinIO controllers")
	kingpin.FatalIfError(operator.SetupWebhooks(mgr), "Cannot setup MinIO webhooks")

	kingpin.FatalIfError(mgr.AddHealthzCheck("healthz", healthz.Ping), "Cannot add health check")
	kingpin.FatalIfError(mgr.AddReadyzCheck("readyz", healthz.Ping), "Cannot add ready check")

	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}

func setupRBAC(c client.Client, l logging.Logger) error {
	ctx := context.Background()

	rules := []rbacv1.PolicyRule{
		{APIGroups: []string{"minio.crossplane.io"}, Resources: []string{"providerconfigs", "providerconfigs/status", "providerconfigusages", "providerconfigusages/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"bucket.minio.crossplane.io"}, Resources: []string{"buckets", "buckets/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"user.minio.crossplane.io"}, Resources: []string{"users", "users/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"policy.minio.crossplane.io"}, Resources: []string{"policies", "policies/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"serviceaccount.minio.crossplane.io"}, Resources: []string{"serviceaccounts", "serviceaccounts/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"notificationconfiguration.minio.crossplane.io"}, Resources: []string{"notificationconfigurations", "notificationconfigurations/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{
			APIGroups: []string{"minio.crossplane.io", "bucket.minio.crossplane.io", "user.minio.crossplane.io", "policy.minio.crossplane.io", "serviceaccount.minio.crossplane.io", "notificationconfiguration.minio.crossplane.io"},
			Resources: []string{"*/finalizers"},
			Verbs:     []string{"update"},
		},
		{APIGroups: []string{"", "coordination.k8s.io"}, Resources: []string{"secrets", "configmaps", "events", "leases"}, Verbs: []string{"*"}},
	}

	system := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crossplane:provider:provider-minio:system",
			Labels: map[string]string{"rbac.crossplane.io/system": "provider-minio"},
		},
		Rules: rules,
	}
	if err := c.Create(ctx, system); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	if err := c.Update(ctx, system); err != nil {
		l.Info("system role update", "err", err)
	}

	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane:provider:provider-minio:system"},
		RoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "crossplane:provider:provider-minio:system"},
		Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "provider-minio", Namespace: "crossplane-system"}},
	}
	if err := c.Create(ctx, binding); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	if err := c.Update(ctx, binding); err != nil {
		l.Info("system binding update", "err", err)
	}

	edit := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crossplane:provider:provider-minio:aggregate-to-edit",
			Labels: map[string]string{
				"rbac.crossplane.io/aggregate-to-edit": "true", "rbac.crossplane.io/aggregate-to-admin": "true",
				"rbac.crossplane.io/aggregate-to-crossplane": "true", "rbac.crossplane.io/system": "provider-minio",
			},
		},
		Rules: withVerbs(rules, []string{"*"}),
	}
	if err := c.Create(ctx, edit); err != nil && !errors.IsAlreadyExists(err) {
		l.Info("aggregate-to-edit create warning (non-fatal)", "err", err)
	}
	_ = c.Update(ctx, edit)

	view := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crossplane:provider:provider-minio:aggregate-to-view",
			Labels: map[string]string{"rbac.crossplane.io/aggregate-to-view": "true", "rbac.crossplane.io/system": "provider-minio"},
		},
		Rules: withVerbs(rules, []string{"get", "list", "watch"}),
	}
	if err := c.Create(ctx, view); err != nil && !errors.IsAlreadyExists(err) {
		l.Info("aggregate-to-view create warning (non-fatal)", "err", err)
	}
	_ = c.Update(ctx, view)

	l.Info("provider self-managed RBAC roles ensured")
	return nil
}

func withVerbs(r []rbacv1.PolicyRule, verbs []string) []rbacv1.PolicyRule {
	out := make([]rbacv1.PolicyRule, len(r))
	for i := range r {
		out[i] = r[i]
		out[i].Verbs = verbs
	}
	return out
}
