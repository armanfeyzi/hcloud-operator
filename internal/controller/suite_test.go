package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	// All tests share the same fake client. Individual tests reset its state
	// or inject errors via the exported fields on FakeClient.
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
