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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("DoltCluster Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		BeforeEach(func() {
			By("creating the custom resource for the Kind DoltCluster")
			testCreateInitialData(ctx)
		})

		AfterEach(func() {
			By("Cleanup the instance DoltCluster")
			testCleanupInitialData(ctx)
		})

		It("should successfully reconcile the resource", func() {
			var testDoltDB doltv1alpha.DoltCluster

			By("Reconciling the created resource")
			_, err := doltDBReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: testDoltKey,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting DoltDB")
			Expect(k8sClient.Get(ctx, testDoltKey, &testDoltDB)).To(Succeed())

			By("Expecting to create a ServiceAccount")
			var svcAcc corev1.ServiceAccount
			svcAccKey := testDoltDB.ServiceAccountKey()
			Expect(k8sClient.Get(ctx, svcAccKey, &svcAcc)).To(Succeed())
			Expect(svcAcc.ObjectMeta.Labels).NotTo(BeNil())
			Expect(svcAcc.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			Expect(svcAcc.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt"))
			Expect(svcAcc.ObjectMeta.Annotations).NotTo(BeNil())
			Expect(svcAcc.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))

			By("Expecting to create a ConfigMap")
			var cm corev1.ConfigMap
			cmKey := types.NamespacedName{
				Name:      testDoltDB.DefaultConfigMapKey().Name,
				Namespace: testDoltDB.DefaultConfigMapKey().Namespace,
			}
			Expect(k8sClient.Get(ctx, cmKey, &cm)).To(Succeed())
			Expect(svcAcc.ObjectMeta.Labels).NotTo(BeNil())
			Expect(svcAcc.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			Expect(svcAcc.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt"))
			Expect(svcAcc.ObjectMeta.Annotations).NotTo(BeNil())
			Expect(svcAcc.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))

			data, err := dolt.GenerateConfigMapData(&testDoltDB)
			Expect(err).NotTo(HaveOccurred())
			Expect(cm.Data).To(Equal(data))

			By("Expecting to create a StatefulSet")
			var sts appsv1.StatefulSet
			Expect(k8sClient.Get(ctx, testDoltKey, &sts)).To(Succeed())
			Expect(sts.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			Expect(sts.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt"))
			Expect(sts.ObjectMeta.Annotations).NotTo(BeNil())
			Expect(sts.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			Expect(sts.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/doltdb", "true"))
			Expect(sts.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/replication", "true"))

			// By("Expecting to create Pod")
			// podKey := types.NamespacedName{
			// 	Name:      statefulset.PodName(testDoltDB.ObjectMeta, 0),
			// 	Namespace: testDoltDB.Namespace,
			// }
			// var pod corev1.Pod
			// Expect(k8sClient.Get(ctx, podKey, &pod)).To(Succeed())
			// Expect(pod.ObjectMeta.Labels).NotTo(BeNil())
			// Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			// Expect(pod.ObjectMeta.Annotations).NotTo(BeNil())
			// Expect(pod.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			// Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt"))

			By("Expecting to create a Headless Service")
			var headlessService corev1.Service
			Expect(k8sClient.Get(ctx, testDoltDB.InternalServiceKey(), &headlessService)).To(Succeed())
			Expect(headlessService.ObjectMeta.Labels).NotTo(BeNil())
			Expect(headlessService.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			Expect(headlessService.ObjectMeta.Annotations).NotTo(BeNil())
			Expect(headlessService.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			Expect(headlessService.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt"))

			By("Expecting to create a Service for Primary instance")
			var primaryService corev1.Service
			Expect(k8sClient.Get(ctx, testDoltDB.PrimaryServiceKey(), &primaryService)).To(Succeed())
			Expect(primaryService.ObjectMeta.Labels).NotTo(BeNil())
			Expect(primaryService.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			Expect(primaryService.ObjectMeta.Annotations).NotTo(BeNil())
			Expect(primaryService.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			Expect(primaryService.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt"))

			By("Expecting to create a service for Reader instances")
			var readerService corev1.Service
			Expect(k8sClient.Get(ctx, testDoltDB.ReaderServiceKey(), &readerService)).To(Succeed())
			Expect(readerService.ObjectMeta.Labels).NotTo(BeNil())
			Expect(readerService.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			Expect(readerService.ObjectMeta.Annotations).NotTo(BeNil())
			Expect(readerService.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			Expect(readerService.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt"))

			By("Expecting to create a PodDisruptionBudget for High Availability")
			var pdb policyv1.PodDisruptionBudget
			Expect(k8sClient.Get(ctx, testDoltDB.PodDisruptionBudgetKey(), &pdb)).To(Succeed())
			Expect(readerService.ObjectMeta.Labels).NotTo(BeNil())
			Expect(readerService.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			Expect(readerService.ObjectMeta.Annotations).NotTo(BeNil())
			Expect(readerService.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			Expect(readerService.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt"))

			// By("Expecting to create a PVC for High Availability")
			// var pvc corev1.PersistentVolumeClaim
			// Expect(k8sClient.Get(ctx, testDoltDB.PodDisruptionBudgetKey(), &pdb)).To(Succeed())
			// Expect(readerService.ObjectMeta.Labels).NotTo(BeNil())
			// Expect(readerService.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			// Expect(readerService.ObjectMeta.Annotations).NotTo(BeNil())
			// Expect(readerService.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			// Expect(readerService.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt"))
		})
	})
})
