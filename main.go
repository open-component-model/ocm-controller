// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"
	"time"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/fluxcd/pkg/runtime/events"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	deliveryv1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/controllers"
	"github.com/open-component-model/ocm-controller/pkg/oci"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
	"github.com/open-component-model/ocm-controller/pkg/snapshot"
	//+kubebuilder:scaffold:imports
)

const controllerName = "ocm-controller"

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(deliveryv1alpha1.AddToScheme(scheme))
	utilruntime.Must(sourcev1.AddToScheme(scheme))
	utilruntime.Must(kustomizev1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var eventsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var ociRegistryAddr string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&eventsAddr, "events-addr", "", "The address of the events receiver.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&ociRegistryAddr, "oci-registry-addr", ":5000", "The address of the OCI registry.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	restConfig := ctrl.GetConfigOrDie()

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f8b21459.ocm.software",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if v, found := os.LookupEnv("OCI_REGISTRY_LOCALHOST"); found {
		ociRegistryAddr = v
	}

	cache := oci.NewClient(ociRegistryAddr)
	ocmClient := ocm.NewClient(mgr.GetClient(), cache)
	snapshotWriter := snapshot.NewOCIWriter(mgr.GetClient(), cache, mgr.GetScheme())
	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		setupLog.Error(err, "unable to get dynamic config client", "controller", "ocm-controller")
		os.Exit(1)
	}

	var eventsRecorder *events.Recorder
	if eventsRecorder, err = events.NewRecorder(mgr, ctrl.Log, eventsAddr, controllerName); err != nil {
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
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Snapshot")
		os.Exit(1)
	}

	if err = (&controllers.ResourceReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		EventRecorder:  eventsRecorder,
		OCMClient:      ocmClient,
		Cache:          cache,
		SnapshotWriter: snapshotWriter,
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
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "FluxDeployer")
		os.Exit(1)
	}
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
