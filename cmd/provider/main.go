package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"

	"github.com/rossigee/provider-minio/apis"
	"github.com/rossigee/provider-minio/operator"
)

// generateSelfSignedCerts creates self-signed TLS certificates for the webhook server
func generateSelfSignedCerts(certDir string) error {
	certPath := filepath.Join(certDir, "tls.crt")
	keyPath := filepath.Join(certDir, "tls.key")

	// Check if certificates already exist
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return nil // Certificates already exist
		}
	}

	// Create certificate directory
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return err
	}

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Crossplane"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:    []string{"localhost", "provider-minio.crossplane-system.svc", "provider-minio.crossplane-system.svc.cluster.local"},
	}

	// Generate certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}

	// Write certificate
	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	defer func() { _ = certOut.Close() }()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return err
	}

	// Write private key
	keyOut, err := os.Create(keyPath)
	if err != nil {
		return err
	}
	defer func() { _ = keyOut.Close() }()

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}); err != nil {
		return err
	}

	return nil
}

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

	// Always set the controller-runtime logger to prevent stacktraces
	// This must be called before any controller-runtime operations
	ctrl.SetLogger(zl)

	log.Debug("Starting", "sync-period", syncPeriod.String())

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")

	// Generate self-signed certificates for webhook server
	certDir := "/tmp/k8s-webhook-server/serving-certs"
	kingpin.FatalIfError(generateSelfSignedCerts(certDir), "Cannot generate webhook certificates")
	log.Info("Webhook certificates generated", "certDir", certDir)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:   *leaderElect,
		LeaderElectionID: "crossplane-leader-election-provider-minio",
		Cache: cache.Options{
			SyncPeriod: syncPeriod,
		},
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaseDuration:              func() *time.Duration { d := 60 * time.Second; return &d }(),
		RenewDeadline:              func() *time.Duration { d := 50 * time.Second; return &d }(),
		WebhookServer:              &webhook.DefaultServer{Options: webhook.Options{Port: 9443}},
	})
	kingpin.FatalIfError(err, "Cannot create controller manager")

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

	kingpin.FatalIfError(apis.AddToScheme(mgr.GetScheme()), "Cannot add MinIO APIs to scheme")
	kingpin.FatalIfError(operator.SetupControllers(mgr), "Cannot setup MinIO controllers")
	kingpin.FatalIfError(operator.SetupWebhooks(mgr), "Cannot setup MinIO webhooks")

	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}
