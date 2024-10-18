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
	"context"
	"flag"
	"os"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	replicationstoragev1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	controllers "github.com/csi-addons/kubernetes-csi-addons/internal/controller/csiaddons"
	replicationController "github.com/csi-addons/kubernetes-csi-addons/internal/controller/replication.storage"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"
	"github.com/csi-addons/kubernetes-csi-addons/internal/version"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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
	var (
		metricsAddr                 string
		probeAddr                   string
		enableLeaderElection        bool
		leaderElectionLeaseDuration time.Duration
		leaderElectionRenewDeadline time.Duration
		leaderElectionRetryPeriod   time.Duration
		showVersion                 bool
		enableAdmissionWebhooks     bool
		ctx                         = context.Background()
		cfg                         = util.NewConfig()
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&leaderElectionLeaseDuration, "leader-election-lease-duration", 15*time.Second, "Duration, in seconds, that non-leader candidates will wait to force acquire leadership. Defaults to 15 seconds.")
	flag.DurationVar(&leaderElectionRenewDeadline, "leader-election-renew-deadline", 10*time.Second, "Duration, in seconds, that the acting leader will retry refreshing leadership before giving up. Defaults to 10 seconds.")
	flag.DurationVar(&leaderElectionRetryPeriod, "leader-election-retry-period", 5*time.Second, "Duration, in seconds, the LeaderElector clients should wait between tries of actions. Defaults to 5 seconds.")
	flag.DurationVar(&cfg.ReclaimSpaceTimeout, "reclaim-space-timeout", cfg.ReclaimSpaceTimeout, "Timeout for reclaimspace operation")
	flag.IntVar(&cfg.MaxConcurrentReconciles, "max-concurrent-reconciles", cfg.MaxConcurrentReconciles, "Maximum number of concurrent reconciles")
	flag.StringVar(&cfg.Namespace, "namespace", cfg.Namespace, "Namespace where the CSIAddons pod is deployed")
	flag.BoolVar(&enableAdmissionWebhooks, "enable-admission-webhooks", false, "[DEPRECATED] Enable the admission webhooks")
	flag.BoolVar(&showVersion, "version", false, "Print Version details")
	flag.StringVar(&cfg.SchedulePrecedence, "schedule-precedence", "", "The order of precedence in which schedule of reclaimspace and keyrotation is considered. Possible values are sc-only")
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if showVersion {
		version.PrintVersion()
		return
	}

	if enableAdmissionWebhooks {
		setupLog.Info("enable-admission-webhooks flag is deprecated and will be removed in a future release")
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	kubeConfig := ctrl.GetConfigOrDie()
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		setupLog.Error(err, "unable to create client")
		os.Exit(1)
	}

	err = cfg.ReadConfigMap(ctx, kubeClient)
	if err != nil {
		setupLog.Error(err, "unable to read configmap")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(kubeConfig, ctrl.Options{
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "e8cd140a.openshift.io",
		LeaseDuration:          &leaderElectionLeaseDuration,
		RenewDeadline:          &leaderElectionRenewDeadline,
		RetryPeriod:            &leaderElectionRetryPeriod,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	connPool := connection.NewConnectionPool()

	ctrlOptions := controller.Options{
		MaxConcurrentReconciles: cfg.MaxConcurrentReconciles,
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
		Timeout:  cfg.ReclaimSpaceTimeout,
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
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		ConnPool:           connPool,
		SchedulePrecedence: cfg.SchedulePrecedence,
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

	if err = (&replicationController.VolumeGroupReplicationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VolumeGroupReplication")
		os.Exit(1)
	}
	if err = (&replicationController.VolumeGroupReplicationContentReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VolumeGroupReplicationContent")
		os.Exit(1)
	}
	if err = (&controllers.EncryptionKeyRotationJobReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		ConnPool: connPool,
		Timeout:  defaultTimeout,
	}).SetupWithManager(mgr, ctrlOptions); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "EncryptionKeyRotationJob")
		os.Exit(1)
	}
	if err = (&controllers.EncryptionKeyRotationCronJobReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, ctrlOptions); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "EncryptionKeyRotationCronJob")
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

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
