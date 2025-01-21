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

package controller

import (
	"context"
	"github.com/electronicarts/doltdb-operator/pkg/controller/volumesnapshot"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	log "sigs.k8s.io/controller-runtime/pkg/log"

	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	k8sdolthubcomv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
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
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient   client.Client
	ctx         context.Context
	cancel      context.CancelFunc
	refResolver *refresolver.RefResolver
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "DoltDB Controllers Suite")
}

var _ = BeforeSuite(func() {
	testLogger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), zap.Level(zapcore.DebugLevel))

	log.SetLogger(testLogger)

	ctx, cancel = context.WithCancel(context.TODO())

	var err error

	cfg := &rest.Config{
		Host: "https://kubernetes.default.svc",
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
		},
		BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
	}
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	err = doltv1alpha.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = k8sdolthubcomv1alpha.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Controller: config.Controller{
			MaxConcurrentReconciles: 1,
		},
	})
	Expect(err).ToNot(HaveOccurred())

	client := k8sManager.GetClient()
	scheme := k8sManager.GetScheme()
	replRecorder := k8sManager.GetEventRecorderFor("replication")

	builder := builder.NewBuilder(scheme)
	refResolver = refresolver.New(k8sClient)
	conditionReady := conditions.NewReady()

	// controllers
	rbacReconciler := rbac.NewReconciler(client, builder)
	configMapReconciler := configmap.NewReconciler(client, builder)
	serviceReconciler := service.NewReconciler(client)
	statefulSetReconciler := statefulset.NewReconciler(client, refResolver, builder)
	statusReconciler := status.NewReconciler(client, refResolver)
	storageReconciler := storage.NewReconciler(client, statefulSetReconciler)
	replConfig := replication.NewReplicationConfig(client, builder)
	volumeSnapshotReconciler := volumesnapshot.NewReconciler(client, builder)
	replicationReconciler, err := replication.NewReconciler(
		client,
		replRecorder,
		builder,
		replConfig,
		replication.WithRefResolver(refResolver),
		replication.WithServiceReconciler(serviceReconciler),
	)
	Expect(err).NotTo(HaveOccurred())

	podReconciler := NewPodController(
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

	err = (&DoltDBReconciler{
		Client:                k8sClient,
		Scheme:                k8sClient.Scheme(),
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
	}).SetupWithManager(k8sManager, ctrlcontroller.Options{MaxConcurrentReconciles: 10})
	Expect(err).ToNot(HaveOccurred())

	err = NewDatabaseReconciler(
		k8sClient,
		refResolver,
		conditionReady,
		database.WithRequeueInterval(30*time.Second),
		database.WithLogSql(false),
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = NewUserReconciler(
		k8sClient,
		refResolver,
		conditionReady,
		database.WithRequeueInterval(30*time.Second),
		database.WithLogSql(false),
	).SetupWithManager(ctx, k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = NewGrantReconciler(
		k8sClient,
		refResolver,
		conditionReady,
		database.WithRequeueInterval(30*time.Second),
		database.WithLogSql(false),
	).SetupWithManager(ctx, k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = podReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = (&SnapshotReconciler{
		Client:                   k8sClient,
		Scheme:                   k8sClient.Scheme(),
		ConditionReady:           conditionReady,
		RefResolver:              refResolver,
		VolumeSnapshotReconciler: volumeSnapshotReconciler,
		Builder:                  builder,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	By("Creating initial test data")
	testCreateInitialData(ctx)

	DeferCleanup(func() {
		By("Cleaning up initial test data")
		testCleanupInitialData(ctx)
		cancel()
	})
})
