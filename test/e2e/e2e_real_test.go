//go:build e2e_real

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	"github.com/armanfeyzi/hcloud-operator/internal/controller"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
	"github.com/armanfeyzi/hcloud-operator/internal/reconcile"
)

const (
	e2eServerType = "cx22"
	e2eImage      = "ubuntu-22.04"
	e2eLocation   = "fsn1"
)

func TestHCloudServerRealE2E(t *testing.T) {
	token := os.Getenv("HCLOUD_TOKEN")
	if token == "" {
		t.Skip("HCLOUD_TOKEN is not set; skipping real-Hetzner E2E")
	}

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	useExisting := os.Getenv("USE_EXISTING_CLUSTER") == "true"
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    &useExisting,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}
	defer func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("stop envtest: %v", err)
		}
	}()

	if err := infrav1alpha1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		t.Fatalf("create k8s client: %v", err)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}

	hcloud := hcloudclient.NewClient(token)
	if err := (&controller.HCloudServerReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		HCloudClient: hcloud,
	}).SetupWithManager(mgr); err != nil {
		t.Fatalf("setup server reconciler: %v", err)
	}

	go func() {
		if err := mgr.Start(ctx); err != nil {
			fmt.Printf("manager stopped: %v\n", err)
		}
	}()
	time.Sleep(2 * time.Second)

	prefix := NamePrefix()
	serverName := ResourceName(prefix, "server")
	lookupKey := types.NamespacedName{Name: serverName}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()

		hcs := &infrav1alpha1.HCloudServer{}
		if err := k8sClient.Get(cleanupCtx, lookupKey, hcs); err == nil {
			t.Logf("cleanup: deleting HCloudServer %s", serverName)
			_ = client.IgnoreNotFound(k8sClient.Delete(cleanupCtx, hcs))
		}

		if info, err := hcloud.GetServerByName(cleanupCtx, serverName); err != nil {
			t.Logf("cleanup: GetServerByName(%q): %v", serverName, err)
		} else if info != nil {
			t.Logf("cleanup: deleting orphaned Hetzner server id=%d name=%q", info.ID, serverName)
			_ = hcloud.DeleteServer(cleanupCtx, info.ID)
		}
	})

	hcs := &infrav1alpha1.HCloudServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: serverName,
			Labels: map[string]string{
				"hkic.io/e2e": "true",
			},
		},
		Spec: infrav1alpha1.HCloudServerSpec{
			ServerType: e2eServerType,
			Image:      e2eImage,
			Location:   e2eLocation,
			Labels: map[string]string{
				"hkic.io/e2e": "true",
			},
		},
	}

	t.Logf("creating HCloudServer %s (%s in %s)", serverName, e2eServerType, e2eLocation)
	if err := k8sClient.Create(ctx, hcs); err != nil {
		t.Fatalf("create HCloudServer: %v", err)
	}

	if err := waitForServerReady(ctx, t, k8sClient, lookupKey); err != nil {
		t.Fatalf("server did not become Ready: %v", err)
	}

	got := &infrav1alpha1.HCloudServer{}
	if err := k8sClient.Get(ctx, lookupKey, got); err != nil {
		t.Fatalf("get HCloudServer after Ready: %v", err)
	}
	if got.Status.ServerID == 0 {
		t.Fatalf("expected status.serverID to be set")
	}
	t.Logf("server Ready: id=%d ipv4=%s", got.Status.ServerID, got.Status.PublicIPv4)

	t.Log("deleting HCloudServer")
	if err := k8sClient.Delete(ctx, got); err != nil {
		t.Fatalf("delete HCloudServer: %v", err)
	}
	if err := waitForObjectDeleted(ctx, k8sClient, lookupKey); err != nil {
		t.Fatalf("kubernetes object not removed: %v", err)
	}

	if err := waitForHetznerServerGone(ctx, hcloud, serverName); err != nil {
		t.Fatalf("hetzner server still exists after delete: %v", err)
	}
	t.Log("real-Hetzner E2E completed successfully")
}

func waitForServerReady(ctx context.Context, t *testing.T, c client.Client, key types.NamespacedName) error {
	t.Helper()
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		obj := &infrav1alpha1.HCloudServer{}
		if err := c.Get(ctx, key, obj); err != nil {
			return fmt.Errorf("get while waiting for Ready: %w", err)
		}
		synced := meta.FindStatusCondition(obj.Status.Conditions, reconcile.ConditionSynced)
		ready := meta.FindStatusCondition(obj.Status.Conditions, reconcile.ConditionReady)
		if synced != nil && synced.Status == metav1.ConditionTrue &&
			ready != nil && ready.Status == metav1.ConditionTrue &&
			obj.Status.State == "running" && obj.Status.PublicIPv4 != "" {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timed out waiting for Synced=True, Ready=True, and running state")
}

func waitForObjectDeleted(ctx context.Context, c client.Client, key types.NamespacedName) error {
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		obj := &infrav1alpha1.HCloudServer{}
		err := c.Get(ctx, key, obj)
		if err != nil && errors.IsNotFound(err) {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("timed out waiting for HCloudServer deletion")
}

func waitForHetznerServerGone(ctx context.Context, hcloud *hcloudclient.Client, name string) error {
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		info, err := hcloud.GetServerByName(ctx, name)
		if err != nil {
			return err
		}
		if info == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("server %q still exists in Hetzner after kubernetes delete", name)
}
