package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
)

var (
	cfg        *rest.Config
	k8sClient  client.Client
	testEnv    *envtest.Environment
	fakeHCloud *hcloudclient.FakeClient
	ctx        context.Context
	cancel     context.CancelFunc
)

// Specs share one envtest cluster and Hetzner fake with random order. Drain CRs then reset the fake so async
// reconciles from a prior example cannot consume fake IDs or leave stale objects.
var _ = BeforeEach(func() {
	if k8sClient != nil {
		drainClusterCRs(ctx, k8sClient)
	}
	if fakeHCloud != nil {
		fakeHCloud.Reset()
	}
})

func drainClusterCRs(ctx context.Context, c client.Client) {
	var servers infrav1alpha1.HCloudServerList
	if err := c.List(ctx, &servers); err == nil {
		for i := range servers.Items {
			_ = client.IgnoreNotFound(c.Delete(ctx, &servers.Items[i]))
		}
	}
	var volumes infrav1alpha1.HCloudVolumeList
	if err := c.List(ctx, &volumes); err == nil {
		for i := range volumes.Items {
			_ = client.IgnoreNotFound(c.Delete(ctx, &volumes.Items[i]))
		}
	}
	var lbs infrav1alpha1.HCloudLoadBalancerList
	if err := c.List(ctx, &lbs); err == nil {
		for i := range lbs.Items {
			_ = client.IgnoreNotFound(c.Delete(ctx, &lbs.Items[i]))
		}
	}
	var nets infrav1alpha1.HCloudNetworkList
	if err := c.List(ctx, &nets); err == nil {
		for i := range nets.Items {
			_ = client.IgnoreNotFound(c.Delete(ctx, &nets.Items[i]))
		}
	}
	var fws infrav1alpha1.HCloudFirewallList
	if err := c.List(ctx, &fws); err == nil {
		for i := range fws.Items {
			_ = client.IgnoreNotFound(c.Delete(ctx, &fws.Items[i]))
		}
	}
	var pgs infrav1alpha1.HCloudPlacementGroupList
	if err := c.List(ctx, &pgs); err == nil {
		for i := range pgs.Items {
			_ = client.IgnoreNotFound(c.Delete(ctx, &pgs.Items[i]))
		}
	}
	_ = wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 20*time.Second, true, func(ctx context.Context) (bool, error) {
		var s infrav1alpha1.HCloudServerList
		if err := c.List(ctx, &s); err != nil {
			return false, nil
		}
		if len(s.Items) > 0 {
			return false, nil
		}
		var v infrav1alpha1.HCloudVolumeList
		if err := c.List(ctx, &v); err != nil {
			return false, nil
		}
		if len(v.Items) > 0 {
			return false, nil
		}
		var l infrav1alpha1.HCloudLoadBalancerList
		if err := c.List(ctx, &l); err != nil {
			return false, nil
		}
		if len(l.Items) > 0 {
			return false, nil
		}
		var n infrav1alpha1.HCloudNetworkList
		if err := c.List(ctx, &n); err != nil {
			return false, nil
		}
		if len(n.Items) > 0 {
			return false, nil
		}
		var fw infrav1alpha1.HCloudFirewallList
		if err := c.List(ctx, &fw); err != nil {
			return false, nil
		}
		if len(fw.Items) > 0 {
			return false, nil
		}
		var pg infrav1alpha1.HCloudPlacementGroupList
		if err := c.List(ctx, &pg); err != nil {
			return false, nil
		}
		if len(pg.Items) > 0 {
			return false, nil
		}
		return true, nil
	})
}

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HCloudServer Controller Suite")
}

func TestMain(m *testing.M) {
	logf.SetLogger(zap.New(zap.WriteTo(os.Stderr), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.Background())

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

	if err := infrav1alpha1.AddToScheme(scheme.Scheme); err != nil {
		fmt.Printf("failed to add scheme: %v\n", err)
		os.Exit(1)
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		fmt.Printf("failed to create client: %v\n", err)
		os.Exit(1)
	}

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	if err != nil {
		fmt.Printf("failed to create manager: %v\n", err)
		os.Exit(1)
	}

	// All tests share the same fake client; Reset() runs in BeforeEach each spec.
	fakeHCloud = hcloudclient.NewFakeClient()

	if err = (&HCloudServerReconciler{
		Client:       k8sManager.GetClient(),
		Scheme:       k8sManager.GetScheme(),
		HCloudClient: fakeHCloud,
	}).SetupWithManager(k8sManager); err != nil {
		fmt.Printf("failed to setup server reconciler: %v\n", err)
		os.Exit(1)
	}

	if err = (&HCloudVolumeReconciler{
		Client:       k8sManager.GetClient(),
		Scheme:       k8sManager.GetScheme(),
		HCloudClient: fakeHCloud,
	}).SetupWithManager(k8sManager); err != nil {
		fmt.Printf("failed to setup volume reconciler: %v\n", err)
		os.Exit(1)
	}

	if err = (&HCloudLoadBalancerReconciler{
		Client:       k8sManager.GetClient(),
		Scheme:       k8sManager.GetScheme(),
		HCloudClient: fakeHCloud,
	}).SetupWithManager(k8sManager); err != nil {
		fmt.Printf("failed to setup load balancer reconciler: %v\n", err)
		os.Exit(1)
	}

	if err = (&HCloudNetworkReconciler{
		Client:       k8sManager.GetClient(),
		Scheme:       k8sManager.GetScheme(),
		HCloudClient: fakeHCloud,
	}).SetupWithManager(k8sManager); err != nil {
		fmt.Printf("failed to setup network reconciler: %v\n", err)
		os.Exit(1)
	}

	if err = (&HCloudFirewallReconciler{
		Client:       k8sManager.GetClient(),
		Scheme:       k8sManager.GetScheme(),
		HCloudClient: fakeHCloud,
	}).SetupWithManager(k8sManager); err != nil {
		fmt.Printf("failed to setup firewall reconciler: %v\n", err)
		os.Exit(1)
	}

	if err = (&HCloudPlacementGroupReconciler{
		Client:       k8sManager.GetClient(),
		Scheme:       k8sManager.GetScheme(),
		HCloudClient: fakeHCloud,
	}).SetupWithManager(k8sManager); err != nil {
		fmt.Printf("failed to setup placement group reconciler: %v\n", err)
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
