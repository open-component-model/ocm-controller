package main

import (
	"flag"
	"os"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/runtime/events"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	glog "gopkg.in/op/go-logging.v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	//+kubebuilder:scaffold:imports

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/controllers"
	"github.com/open-component-model/ocm-controller/pkg/oci"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
	"github.com/open-component-model/ocm-controller/pkg/snapshot"
)

const (
	controllerName      = "ocm-controller"
	defaultKubeAPIQPS   = 20
	defaultKubeAPIBurst = 30
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(sourcev1.AddToScheme(scheme))
	utilruntime.Must(kustomizev1.AddToScheme(scheme))
	utilruntime.Must(helmv2.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

type flags struct {
	metricsAddr                   string
	eventsAddr                    string
	enableLeaderElection          bool
	probeAddr                     string
	pprofAddr                     string
	ociRegistryAddr               string
	ociRegistryCertSecretName     string
	ociRegistryInsecureSkipVerify bool
	ociRegistryNamespace          string
	kubeAPIQPS                    float64
	kubeAPIBurst                  int
	kubeAPIRateLimiterDisabled    bool
	concurrencyComponentVersion   int
	concurrencyResource           int
	concurrencySnapshot           int
	concurrencyLocalization       int
	concurrencyConfiguration      int
	concurrencyFluxDeployer       int
}

func bindFlags() (flags, zap.Options) {
	var f flags
	flag.StringVar(
		&f.metricsAddr,
		"metrics-bind-address",
		":9443",
		"The address the metric endpoint binds to.",
	)
	flag.StringVar(&f.eventsAddr, "events-addr", "", "The address of the events receiver.")
	flag.StringVar(
		&f.probeAddr,
		"health-probe-bind-address",
		":8081",
		"The address the probe endpoint binds to.",
	)
	flag.StringVar(
		&f.pprofAddr,
		"pprof-bind-address",
		":6060",
		"The address the pprof endpoint binds to (only used in builds with the pprof tag).",
	)
	flag.StringVar(
		&f.ociRegistryAddr,
		"oci-registry-addr",
		":5000",
		"The address of the OCI registry.",
	)
	flag.StringVar(
		&f.ociRegistryCertSecretName,
		"certificate-secret-name",
		v1alpha1.DefaultRegistryCertificateSecretName,
		"",
	)
	flag.StringVar(
		&f.ociRegistryNamespace,
		"oci-registry-namespace",
		"ocm-system",
		"The namespace in which the registry is running in.",
	)
	flag.BoolVar(
		&f.ociRegistryInsecureSkipVerify,
		"oci-registry-insecure-skip-verify",
		false,
		"Skip verification of the certificate that the registry is using.",
	)
	flag.BoolVar(&f.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Float64Var(
		&f.kubeAPIQPS,
		"kube-api-qps",
		defaultKubeAPIQPS,
		"Maximum queries per second to the Kubernetes API. Ignored when --kube-api-rate-limiter-disabled is set.",
	)
	flag.IntVar(
		&f.kubeAPIBurst,
		"kube-api-burst",
		defaultKubeAPIBurst,
		"Maximum burst for Kubernetes API client throttling. Ignored when --kube-api-rate-limiter-disabled is set.",
	)
	flag.BoolVar(
		&f.kubeAPIRateLimiterDisabled,
		"kube-api-rate-limiter-disabled",
		false,
		"Disable the client-side Kubernetes API rate limiter and rely on API priority and fairness instead.",
	)
	flag.IntVar(&f.concurrencyComponentVersion, "concurrency-component-version", 1, "Number of workers for the ComponentVersion controller.")
	flag.IntVar(&f.concurrencyResource, "concurrency-resource", 1, "Number of workers for the Resource controller.")
	flag.IntVar(&f.concurrencySnapshot, "concurrency-snapshot", 1, "Number of workers for the Snapshot controller.")
	flag.IntVar(&f.concurrencyLocalization, "concurrency-localization", 1, "Number of workers for the Localization controller.")
	flag.IntVar(&f.concurrencyConfiguration, "concurrency-configuration", 1, "Number of workers for the Configuration controller.")
	flag.IntVar(&f.concurrencyFluxDeployer, "concurrency-flux-deployer", 1, "Number of workers for the FluxDeployer controller.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	return f, opts
}

func main() {
	f, opts := bindFlags()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	gk := func(kind string) string { return v1alpha1.GroupVersion.WithKind(kind).GroupKind().String() }
	groupKindConcurrency := map[string]int{
		gk("ComponentVersion"): f.concurrencyComponentVersion,
		gk("Resource"):         f.concurrencyResource,
		gk("Snapshot"):         f.concurrencySnapshot,
		gk("Localization"):     f.concurrencyLocalization,
		gk("Configuration"):    f.concurrencyConfiguration,
		gk("FluxDeployer"):     f.concurrencyFluxDeployer,
	}
	// 2024-05-11 :
	// We're setting the go-log leve because it is used by yqlib
	// If we ever make it possible to set the log level for the service
	// , I mean stop hard coding the log level, then we will need to remember
	// to set it both for controller-runtime ( i.e. zap )  and yqlib ( i.e. go-log )
	glog.SetLevel(glog.WARNING, "yq-lib")

	restConfig := ctrl.GetConfigOrDie()
	if f.kubeAPIRateLimiterDisabled {
		restConfig.QPS = -1
		restConfig.Burst = 0
	} else {
		restConfig.QPS = float32(f.kubeAPIQPS)
		restConfig.Burst = f.kubeAPIBurst
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: f.metricsAddr,
		},
		HealthProbeBindAddress: f.probeAddr,
		LeaderElection:         f.enableLeaderElection,
		LeaderElectionID:       "f8b21459.ocm.software",
		Controller: controllerconfig.Controller{
			GroupKindConcurrency: groupKindConcurrency,
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ociRegistryAddr := f.ociRegistryAddr
	if v, found := os.LookupEnv("OCI_REGISTRY_LOCALHOST"); found {
		ociRegistryAddr = v
	}

	if pprofEnabled {
		setupLog.Info("starting pprof server", "address", f.pprofAddr)
		startPprof(f.pprofAddr)
	}

	setupManagers(ociRegistryAddr, mgr, f.ociRegistryNamespace, f.ociRegistryCertSecretName, f.ociRegistryInsecureSkipVerify, restConfig, f.eventsAddr)

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()
	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupManagers(
	ociRegistryAddr string,
	mgr manager.Manager,
	ociRegistryNamespace, ociRegistryCertSecretName string,
	ociRegistryInsecureSkipVerify bool,
	restConfig *rest.Config,
	eventsAddr string,
) {
	cache := oci.NewClient(
		ociRegistryAddr,
		oci.WithClient(mgr.GetClient()),
		oci.WithNamespace(ociRegistryNamespace),
		oci.WithCertificateSecret(ociRegistryCertSecretName),
		oci.WithInsecureSkipVerify(ociRegistryInsecureSkipVerify),
	)
	ocmClient := ocm.NewClient(mgr.GetClient(), cache)
	snapshotWriter := snapshot.NewOCIWriter(mgr.GetClient(), cache, mgr.GetScheme())
	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		setupLog.Error(err, "unable to get dynamic config client", "controller", "ocm-controller")
		os.Exit(1)
	}

	var eventsRecorder events.Recorder
	if eventsRecorder, err = events.NewRecorder(ctrl.Log, eventsAddr, controllerName, events.WithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create event recorder")
		os.Exit(1)
	}

	if err = (&controllers.ComponentVersionReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		EventRecorder: eventsRecorder,
		OCMClient:     ocmClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ComponentVersion")
		os.Exit(1)
	}

	if err = (&controllers.SnapshotReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		EventRecorder:       eventsRecorder,
		RegistryServiceName: ociRegistryAddr,
		Cache:               cache,
		InsecureSkipVerify:  ociRegistryInsecureSkipVerify,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Snapshot")
		os.Exit(1)
	}

	if err = (&controllers.ResourceReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		EventRecorder: eventsRecorder,
		OCMClient:     ocmClient,
		Cache:         cache,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Resource")
		os.Exit(1)
	}

	mutationReconciler := controllers.MutationReconcileLooper{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		OCMClient:      ocmClient,
		DynamicClient:  dynClient,
		Cache:          cache,
		SnapshotWriter: snapshotWriter,
	}

	if err = (&controllers.LocalizationReconciler{
		Client:             mgr.GetClient(),
		DynamicClient:      dynClient,
		Scheme:             mgr.GetScheme(),
		EventRecorder:      eventsRecorder,
		ReconcileInterval:  time.Hour,
		RetryInterval:      time.Minute,
		OCMClient:          ocmClient,
		Cache:              cache,
		MutationReconciler: mutationReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Localization")
		os.Exit(1)
	}
	if err = (&controllers.ConfigurationReconciler{
		Client:             mgr.GetClient(),
		DynamicClient:      dynClient,
		Scheme:             mgr.GetScheme(),
		EventRecorder:      eventsRecorder,
		ReconcileInterval:  time.Hour,
		RetryInterval:      time.Minute,
		OCMClient:          ocmClient,
		Cache:              cache,
		MutationReconciler: mutationReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Configuration")
		os.Exit(1)
	}
	if err = (&controllers.FluxDeployerReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		EventRecorder:       eventsRecorder,
		ReconcileInterval:   time.Hour,
		RetryInterval:       time.Minute,
		DynamicClient:       dynClient,
		RegistryServiceName: ociRegistryAddr,
		CertSecretName:      ociRegistryCertSecretName,
		Cache:               cache,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "FluxDeployer")
		os.Exit(1)
	}
}
