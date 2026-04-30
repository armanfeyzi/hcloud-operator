package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	"github.com/armanfeyzi/hcloud-operator/internal/controller"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(infrav1alpha1.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8082", "The address the metrics endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the health probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for high availability.")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// ── Hetzner Cloud token ────────────────────────────────────────────────
	hcloudToken := os.Getenv("HCLOUD_TOKEN")
	if hcloudToken == "" {
		setupLog.Error(nil, "HCLOUD_TOKEN environment variable is required")
		os.Exit(1)
	}

	// ── Controller Manager ─────────────────────────────────────────────────
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "hcloud-operator.infra.hkc.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// ── Register Reconcilers ───────────────────────────────────────────────
	if err = (&controller.HCloudServerReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		HCloudClient: hcloudclient.NewClient(hcloudToken),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HCloudServer")
		os.Exit(1)
	}

	if err = (&controller.HCloudVolumeReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		HCloudClient: hcloudclient.NewClient(hcloudToken),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HCloudVolume")
		os.Exit(1)
	}

	if err = (&controller.HCloudLoadBalancerReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		HCloudClient: hcloudclient.NewClient(hcloudToken),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HCloudLoadBalancer")
		os.Exit(1)
	}

	if err = (&controller.HCloudNetworkReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		HCloudClient: hcloudclient.NewClient(hcloudToken),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HCloudNetwork")
		os.Exit(1)
	}

	if err = (&controller.HCloudFirewallReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		HCloudClient: hcloudclient.NewClient(hcloudToken),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HCloudFirewall")
		os.Exit(1)
	}

	if err = (&controller.HCloudPlacementGroupReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		HCloudClient: hcloudclient.NewClient(hcloudToken),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HCloudPlacementGroup")
		os.Exit(1)
	}

	if err = (&controller.HCloudPrimaryIPReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		HCloudClient: hcloudclient.NewClient(hcloudToken),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HCloudPrimaryIP")
		os.Exit(1)
	}

	if err = (&controller.HCloudFloatingIPReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		HCloudClient: hcloudclient.NewClient(hcloudToken),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HCloudFloatingIP")
		os.Exit(1)
	}

	// ── Health / Readiness probes ──────────────────────────────────────────
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting hcloud-operator manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
