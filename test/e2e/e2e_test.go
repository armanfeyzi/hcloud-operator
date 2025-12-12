package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	infrav1alpha1 "github.com/afeyzirealyticsio/hcloud-operator/api/v1alpha1"
	"github.com/afeyzirealyticsio/hcloud-operator/internal/controller"
	hcloudclient "github.com/afeyzirealyticsio/hcloud-operator/internal/hcloud"
)

func TestHCloudServerE2E(t *testing.T) {
	token := os.Getenv("HCLOUD_TOKEN")
	if token == "" {
		t.Skip("Skipping E2E test; HCLOUD_TOKEN environment variable is not set")
	}

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Setup envtest
	useExisting := os.Getenv("USE_EXISTING_CLUSTER") == "true"
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    &useExisting,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("Failed to start envtest: %v", err)
	}
	defer func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("Failed to stop envtest: %v", err)
		}
	}()

	err = infrav1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		t.Fatalf("Failed to create k8s client: %v", err)
	}

	// 2. Setup controller manager
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0", // Disable metrics server to avoid port conflicts
		},
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = (&controller.HCloudServerReconciler{
		Client:       k8sManager.GetClient(),
		Scheme:       k8sManager.GetScheme(),
		HCloudClient: hcloudclient.NewClient(token),
	}).SetupWithManager(k8sManager)
	if err != nil {
		t.Fatalf("Failed to setup reconciler: %v", err)
	}

	go func() {
		if err := k8sManager.Start(ctx); err != nil {
			fmt.Printf("Manager failed: %v\n", err)
		}
	}()

	// Wait for manager to start
	time.Sleep(2 * time.Second)

	// 3. Create an HCloudServer resource
	serverName := "e2e-test-node-1"
	hcs := &infrav1alpha1.HCloudServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverName,
			Namespace: "default", // Although cluster-scoped, client requires namespace for some operations or we omit it.
		},
		Spec: infrav1alpha1.HCloudServerSpec{
			ServerType: "cx22",
			Image:      "ubuntu-22.04",
			Location:   "fsn1",
			Labels: map[string]string{
				"e2e-test": "true",
			},
		},
	}

	// For cluster-scoped resources, Namespace should be empty
	hcs.Namespace = ""

	t.Logf("Creating HCloudServer %s", serverName)
	err = k8sClient.Create(ctx, hcs)
	if err != nil {
		t.Fatalf("Failed to create HCloudServer: %v", err)
	}

	lookupKey := types.NamespacedName{Name: serverName, Namespace: ""}
	createdHCS := &infrav1alpha1.HCloudServer{}

	// 4. Wait for the server to be running
	t.Log("Waiting for server to become running...")
	var serverID int64
	for i := 0; i < 60; i++ {
		err = k8sClient.Get(ctx, lookupKey, createdHCS)
		if err == nil && createdHCS.Status.State == "running" && createdHCS.Status.PublicIPv4 != "" {
			serverID = createdHCS.Status.ServerID
			t.Logf("Server is running! ID: %d, IP: %s", serverID, createdHCS.Status.PublicIPv4)
			break
		}
		time.Sleep(2 * time.Second)
	}

	if serverID == 0 {
		t.Fatalf("Server did not reach running state in time. Current status: %+v", createdHCS.Status)
	}

	// 5. Delete the server
	t.Log("Deleting HCloudServer...")
	err = k8sClient.Delete(ctx, createdHCS)
	if err != nil {
		t.Fatalf("Failed to delete HCloudServer: %v", err)
	}

	// 6. Wait for deletion to complete (finalizer logic)
	t.Log("Waiting for finalizer cleanup...")
	deleted := false
	for i := 0; i < 30; i++ {
		err = k8sClient.Get(ctx, lookupKey, createdHCS)
		if err != nil && errors.IsNotFound(err) {
			deleted = true
			break
		}
		time.Sleep(2 * time.Second)
	}

	if !deleted {
		t.Fatalf("HCloudServer was not deleted from Kubernetes (finalizer blocked?)")
	}

	t.Log("E2E test completed successfully!")
}
