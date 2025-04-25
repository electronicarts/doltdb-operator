/*
Copyright 2024.

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
	"crypto/tls"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/internal/controller"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/controller/configmap"
	"github.com/electronicarts/doltdb-operator/pkg/controller/database"
	"github.com/electronicarts/doltdb-operator/pkg/controller/rbac"
	"github.com/electronicarts/doltdb-operator/pkg/controller/replication"
	"github.com/electronicarts/doltdb-operator/pkg/controller/service"
	"github.com/electronicarts/doltdb-operator/pkg/controller/statefulset"
	"github.com/electronicarts/doltdb-operator/pkg/controller/status"
	"github.com/electronicarts/doltdb-operator/pkg/controller/storage"
	"github.com/electronicarts/doltdb-operator/pkg/controller/volumesnapshot"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(doltv1alpha.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	var maxConcurrentReconciles int
	var maxDoltDBMaxConcurrentReconciles int
	var logDevMode bool
	var logSql bool
	var requeueSql time.Duration
	var watchNamespaces string

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", 1,
		"Global maximum number of concurrent reconciles per resource.")
	flag.IntVar(&maxDoltDBMaxConcurrentReconciles, "max-doltdb-concurrent-reconciles", 10,
		"DoltDB controller maximum number of concurrent reconciles per resource.")
	flag.BoolVar(&logDevMode, "log-dev-mode", true, "Enable development logs.")
	flag.BoolVar(&logSql, "log-sql", false, "Enable SQL resource logs.")
	flag.DurationVar(&requeueSql, "requeue-sql", 30*time.Second, "The interval at which SQL objects are requeued.")
	flag.StringVar(&watchNamespaces, "watch-namespaces", "",
		"The comma-separated list of namespaces to watch for changes. If not set, all namespaces are watched.")

	opts := zap.Options{
		Development: logDevMode,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	ctx, cancel := signal.NotifyContext(context.Background(), []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL,
		syscall.SIGHUP,
		syscall.SIGQUIT}...,
	)
	defer cancel()

	mgrOpts := ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "k8s.dolthub.com",
		Controller: config.Controller{
			MaxConcurrentReconciles: maxConcurrentReconciles,
		},
	}

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		// TODO(user): TLSOpts is used to allow configuring the TLS config used for the server. If certificates are
		// not provided, self-signed certificates will be generated by default. This option is not recommended for
		// production environments as self-signed certificates do not offer the same level of trust and security
		// as certificates issued by a trusted Certificate Authority (CA). The primary risk is potentially allowing
		// unauthorized access to sensitive metrics data. Consider replacing with CertDir, CertName, and KeyName
		// to provide certificates, ensuring the server communicates using trusted and secure certificates.
		TLSOpts: tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	mgrOpts.Metrics = metricsServerOptions
	mgrOpts.WebhookServer = webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	if watchNamespaces != "" {
		namespaces := strings.Split(watchNamespaces, ",")

		setupLog.Info("Watching namespaces", "namespaces", namespaces)
		mgrOpts.Cache.DefaultNamespaces = make(map[string]cache.Config, len(namespaces))

		for _, ns := range namespaces {
			mgrOpts.Cache.DefaultNamespaces[ns] = cache.Config{}
		}
	} else {
		setupLog.Info("Watching all namespaces")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	client := mgr.GetClient()
	scheme := mgr.GetScheme()
	replRecorder := mgr.GetEventRecorderFor("replication")

	conditionReady := conditions.NewReady()
	builder := builder.NewBuilder(scheme)
	refResolver := refresolver.New(client)

	// controllers
	rbacReconciler := rbac.NewReconciler(client, builder)
	configMapReconciler := configmap.NewReconciler(client, builder)
	volumeSnapshotReconciler := volumesnapshot.NewReconciler(client, builder)
	serviceReconciler := service.NewReconciler(client)
	statefulSetReconciler := statefulset.NewReconciler(client, refResolver, builder)
	statusReconciler := status.NewReconciler(client, refResolver)
	storageReconciler := storage.NewReconciler(client, statefulSetReconciler)
	replConfig := replication.NewReplicationConfig(client, builder)
	replicationReconciler, err := replication.NewReconciler(
		client,
		replRecorder,
		builder,
		replConfig,
		replication.WithRefResolver(refResolver),
		replication.WithServiceReconciler(serviceReconciler),
	)
	if err != nil {
		setupLog.Error(err, "Error creating Replication reconciler")
		os.Exit(1)
	}

	podReplicationController := controller.NewPodController(
		"pod-replication",
		client,
		refResolver,
		replication.NewPodReadinessController(
			client,
			replRecorder,
			builder,
			refResolver,
			replConfig,
		),
		[]string{
			dolt.Annotation,
			dolt.ReplicationAnnotation,
		},
	)

	if err = (&controller.DoltDBReconciler{
		Client:                client,
		Scheme:                scheme,
		Builder:               builder,
		ConditionReady:        conditionReady,
		RefResolver:           refResolver,
		StorageReconciler:     storageReconciler,
		StatusReconciler:      statusReconciler,
		RBACReconciler:        rbacReconciler,
		ConfigMapReconciler:   configMapReconciler,
		ServiceReconciler:     serviceReconciler,
		StatefulSetReconciler: statefulSetReconciler,
		ReplicationReconciler: replicationReconciler,
	}).SetupWithManager(mgr, ctrlcontroller.Options{MaxConcurrentReconciles: maxDoltDBMaxConcurrentReconciles}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DoltDB")
		os.Exit(1)
	}

	if err = podReplicationController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create controller", "controller", "PodReplication")
		os.Exit(1)
	}

	if err = (&controller.SnapshotReconciler{
		Client:                   client,
		Scheme:                   scheme,
		Builder:                  builder,
		RefResolver:              refResolver,
		VolumeSnapshotReconciler: volumeSnapshotReconciler,
		ConditionReady:           conditionReady,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Snapshot")
		os.Exit(1)
	}

	sqlOpts := []database.SqlOpt{
		database.WithRequeueInterval(requeueSql),
		database.WithLogSql(logSql),
	}
	if err = controller.NewDatabaseReconciler(client, refResolver, conditionReady, sqlOpts...).
		SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create controller", "controller", "Database")
		os.Exit(1)
	}

	if err = controller.NewUserReconciler(client, refResolver, conditionReady, sqlOpts...).
		SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "Unable to create controller", "controller", "User")
		os.Exit(1)
	}

	if err = controller.NewGrantReconciler(client, refResolver, conditionReady, sqlOpts...).
		SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "Unable to create controller", "controller", "Grant")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

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
