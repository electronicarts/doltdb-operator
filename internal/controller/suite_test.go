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
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	doltctrl "github.com/electronicarts/doltdb-operator/pkg/controller"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/controller/replication"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc
var doltDBReconciler *DoltDBReconciler
var podReconciler *PodController

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    ptr.To(true),

		// // The BinaryAssetsDirectory is only required if you want to run the tests directly
		// // without call the makefile target test. If not informed it will look for the
		// // default path defined in controller-runtime which is /usr/local/kubebuilder/.
		// // Note that you must have the required binaries setup under the bin directory to perform
		// // the tests directly. When we run make test it will be setup and used automatically.
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.31.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	DeferCleanup(testEnv.Stop)

	err = doltv1alpha.AddToScheme(scheme.Scheme)
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

	replRecorder := k8sManager.GetEventRecorderFor("replication")

	builder := builder.NewBuilder(k8sClient.Scheme())
	refResolver := refresolver.New(k8sClient)
	conditionReady := conditions.NewReady()

	// controllers
	rbacReconciler := doltctrl.NewRBACReconiler(k8sClient, builder)
	configMapReconciler := doltctrl.NewConfigMapReconciler(k8sClient, builder)
	serviceReconciler := doltctrl.NewServiceReconciler(k8sClient)
	statefulSetReconciler := doltctrl.NewStatefulSetReconciler(k8sClient)
	replConfig := replication.NewReplicationConfig(k8sClient, builder)
	replicationReconciler, err := replication.NewReplicationReconciler(
		k8sClient,
		replRecorder,
		builder,
		replConfig,
		replication.WithRefResolver(refResolver),
		replication.WithServiceReconciler(serviceReconciler),
	)
	Expect(err).NotTo(HaveOccurred())

	podReconciler = NewPodController(
		"pod-replication",
		k8sClient,
		refResolver,
		replication.NewPodReadinessController(
			k8sClient,
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

	doltDBReconciler = (&DoltDBReconciler{
		Client:                k8sClient,
		Scheme:                k8sClient.Scheme(),
		Builder:               builder,
		ConditionReady:        conditionReady,
		RefResolver:           refResolver,
		RBACReconciler:        rbacReconciler,
		ConfigMapReconciler:   configMapReconciler,
		ServiceReconciler:     serviceReconciler,
		StatefulSetReconciler: statefulSetReconciler,
		ReplicationReconciler: replicationReconciler,
	})
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
