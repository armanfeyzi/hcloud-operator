package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	infrav1alpha1 "github.com/afeyzirealyticsio/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/afeyzirealyticsio/hcloud-operator/internal/hcloud"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestMain(m *testing.M) {
	logf.SetLogger(zap.New(zap.WriteTo(os.Stderr), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.Background())

	// Initialize envtest
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	if err != nil {
		fmt.Printf("failed to start envtest: %v\n", err)
		os.Exit(1)
	}

	// Register API types
	if err := infrav1alpha1.AddToScheme(scheme.Scheme); err != nil {
		fmt.Printf("failed to add scheme: %v\n", err)
		os.Exit(1)
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		fmt.Printf("failed to create client: %v\n", err)
		os.Exit(1)
	}

	// Set up the controller manager
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0", // Disable metrics server to avoid port conflicts
		},
	})
	if err != nil {
		fmt.Printf("failed to create manager: %v\n", err)
		os.Exit(1)
	}

	// Setup our reconciler with a dummy Hetzner token if none is provided.
	// For real e2e tests, HCLOUD_TOKEN should be set in the environment.
	token := os.Getenv("HCLOUD_TOKEN")
	if token == "" {
		token = "dummy-token-for-envtest"
	}

	err = (&HCloudServerReconciler{
		Client:       k8sManager.GetClient(),
		Scheme:       k8sManager.GetScheme(),
		HCloudClient: hcloudclient.NewClient(token),
	}).SetupWithManager(k8sManager)
	if err != nil {
		fmt.Printf("failed to setup reconciler: %v\n", err)
		os.Exit(1)
	}

	go func() {
		if err := k8sManager.Start(ctx); err != nil {
			fmt.Printf("failed to start manager: %v\n", err)
			os.Exit(1)
		}
	}()

	code := m.Run()

	cancel()
	if err := testEnv.Stop(); err != nil {
		fmt.Printf("failed to stop envtest: %v\n", err)
	}

	os.Exit(code)
}
