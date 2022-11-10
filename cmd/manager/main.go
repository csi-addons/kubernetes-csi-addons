/*
Copyright 2021 The Kubernetes-CSI-Addons Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/csiaddons/v1alpha1"
	replicationstoragev1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/replication.storage/v1alpha1"
	controllers "github.com/csi-addons/kubernetes-csi-addons/controllers/csiaddons"
	replicationController "github.com/csi-addons/kubernetes-csi-addons/controllers/replication.storage"
	"github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const (
	defaultTimeout = time.Minute * 3
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(csiaddonsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(replicationstoragev1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var reclaimSpaceTimeout time.Duration
	var maxConcurrentReconciles int
	var enableAdmissionWebhooks bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&reclaimSpaceTimeout, "reclaim-space-timeout", defaultTimeout, "Timeout for reclaimspace operation")
	flag.IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", 100, "Maximum number of concurrent reconciles")
	flag.BoolVar(&enableAdmissionWebhooks, "enable-admission-webhooks", true, "Enable the admission webhooks")
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "e8cd140a.openshift.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	connPool := connection.NewConnectionPool()

	ctrlOptions := controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}
	if err = (&controllers.CSIAddonsNodeReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		ConnPool: connPool,
	}).SetupWithManager(mgr, ctrlOptions); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CSIAddonsNode")
		os.Exit(1)
	}

	if err = (&controllers.ReclaimSpaceJobReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		ConnPool: connPool,
		Timeout:  reclaimSpaceTimeout,
	}).SetupWithManager(mgr, ctrlOptions); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ReclaimSpaceJob")
		os.Exit(1)
	}
	if err = (&controllers.NetworkFenceReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Connpool: connPool,
		Timeout:  time.Minute * 3,
	}).SetupWithManager(mgr, ctrlOptions); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NetworkFence")
		os.Exit(1)
	}
	if err = (&controllers.ReclaimSpaceCronJobReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, ctrlOptions); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ReclaimSpaceCronJob")
		os.Exit(1)
	}
	if err = (&controllers.PersistentVolumeClaimReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, ctrlOptions); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PersistentVolumeClaim")
		os.Exit(1)
	}
	if err = (&replicationController.VolumeReplicationReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Connpool: connPool,
		Timeout:  defaultTimeout,
	}).SetupWithManager(mgr, ctrlOptions); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VolumeReplication")
		os.Exit(1)
	}

	if enableAdmissionWebhooks {
		if err = (&replicationstoragev1alpha1.VolumeReplicationClass{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "VolumeReplicationClass")
			os.Exit(1)
		}
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

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
