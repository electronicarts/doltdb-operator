// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DoltDB Controller", func() {
	It("should reconcile", func() {
		var testDoltDB doltv1alpha.DoltDB

		By("Getting DoltDB")
		Expect(k8sClient.Get(ctx, testDoltKey, &testDoltDB)).To(Succeed())

		By("Expecting to create a ConfigMap eventually")
		Eventually(func(g Gomega) bool {
			var cm corev1.ConfigMap
			cmKey := types.NamespacedName{
				Name:      testDoltDB.DefaultConfigMapKey().Name,
				Namespace: testDoltDB.DefaultConfigMapKey().Namespace,
			}
			if err := k8sClient.Get(ctx, cmKey, &cm); err != nil {
				return false
			}
			g.Expect(cm.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(cm.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			g.Expect(cm.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt"))
			g.Expect(cm.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(cm.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))

			data, err := dolt.GenerateConfigMapData(&testDoltDB)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cm.Data).To(Equal(data))

			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a StatefulSet eventually")
		Eventually(func(g Gomega) bool {
			var sts appsv1.StatefulSet
			if err := k8sClient.Get(ctx, testDoltKey, &sts); err != nil {
				return false
			}
			g.Expect(sts.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			g.Expect(sts.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt"))
			g.Expect(sts.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(sts.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			g.Expect(sts.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/doltdb", testDoltDB.Name))
			g.Expect(sts.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/replication", "true"))
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a Headless Service eventually")
		Eventually(func(g Gomega) bool {
			var headlessService corev1.Service
			if err := k8sClient.Get(ctx, testDoltDB.InternalServiceKey(), &headlessService); err != nil {
				return false
			}

			g.Expect(headlessService.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(headlessService.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			g.Expect(headlessService.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(headlessService.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			g.Expect(headlessService.Spec.ClusterIP).To(Equal("None"))
			g.Expect(headlessService.Spec.Selector).To(HaveKeyWithValue("app.kubernetes.io/name", testDoltDB.Name))

			// Verify ports
			g.Expect(headlessService.Spec.Ports).To(HaveLen(2))

			// Check MySQL port
			mysqlPort := findServicePort(headlessService.Spec.Ports, builder.DoltMySQLPortName)
			g.Expect(mysqlPort).NotTo(BeNil())
			g.Expect(mysqlPort.Port).To(Equal(testDoltDB.Spec.Server.Listener.Port))
			g.Expect(mysqlPort.Name).To(Equal(builder.DoltMySQLPortName))

			// Check RemotesAPI port
			remotesAPIPort := findServicePort(headlessService.Spec.Ports, builder.DoltRemotesAPIPortName)
			g.Expect(remotesAPIPort).NotTo(BeNil())
			g.Expect(remotesAPIPort.Port).To(Equal(testDoltDB.Spec.Server.Cluster.RemotesAPI.Port))
			g.Expect(remotesAPIPort.Name).To(Equal(builder.DoltRemotesAPIPortName))

			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a Service for Primary instance eventually")
		Eventually(func(g Gomega) bool {
			var primaryService corev1.Service
			if err := k8sClient.Get(ctx, testDoltDB.PrimaryServiceKey(), &primaryService); err != nil {
				return false
			}
			g.Expect(primaryService.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(primaryService.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			g.Expect(primaryService.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(primaryService.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			g.Expect(primaryService.Spec.Selector).To(HaveKeyWithValue(dolt.RoleLabel, dolt.PrimaryRoleValue.String()))
			g.Expect(primaryService.Spec.Selector).To(HaveKeyWithValue("app.kubernetes.io/name", testDoltDB.Name))

			// Verify ports
			g.Expect(primaryService.Spec.Ports).To(HaveLen(1))

			// Check MySQL port
			mysqlPort := findServicePort(primaryService.Spec.Ports, builder.DoltMySQLPortName)
			g.Expect(mysqlPort).NotTo(BeNil())
			g.Expect(mysqlPort.Port).To(Equal(testDoltDB.Spec.Server.Listener.Port))
			g.Expect(mysqlPort.Name).To(Equal(builder.DoltMySQLPortName))

			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a service for Reader instances eventually")
		Eventually(func(g Gomega) bool {
			var readerService corev1.Service
			if err := k8sClient.Get(ctx, testDoltDB.ReaderServiceKey(), &readerService); err != nil {
				return false
			}

			g.Expect(readerService.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(readerService.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			g.Expect(readerService.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(readerService.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			g.Expect(readerService.Spec.Selector).To(HaveKeyWithValue(dolt.RoleLabel, dolt.StandbyRoleValue.String()))
			g.Expect(readerService.Spec.Selector).To(HaveKeyWithValue("app.kubernetes.io/name", testDoltDB.Name))

			// Verify ports
			g.Expect(readerService.Spec.Ports).To(HaveLen(1))

			// Check MySQL port
			mysqlPort := findServicePort(readerService.Spec.Ports, builder.DoltMySQLPortName)
			g.Expect(mysqlPort).NotTo(BeNil())
			g.Expect(mysqlPort.Port).To(Equal(testDoltDB.Spec.Server.Listener.Port))
			g.Expect(mysqlPort.Name).To(Equal(builder.DoltMySQLPortName))

			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create Pods eventually")
		Eventually(func(g Gomega) bool {
			var pod corev1.PodList
			listOpts := &client.ListOptions{
				LabelSelector: klabels.SelectorFromSet(
					builder.NewLabelsBuilder().
						WithDoltSelectorLabels(&testDoltDB).
						Build(),
				),
				Namespace: testDoltDB.GetNamespace(),
			}
			if err := k8sClient.List(ctx, &pod, listOpts); err != nil {
				return false
			}
			if len(pod.Items) != int(testDoltDB.Spec.Replicas) {
				return false
			}

			for _, pod := range pod.Items {
				g.Expect(pod.ObjectMeta.Labels).NotTo(BeNil())
				g.Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
				g.Expect(pod.ObjectMeta.Annotations).NotTo(BeNil())
				g.Expect(pod.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
				g.Expect(pod.ObjectMeta.Annotations).To(HaveKeyWithValue("pod-annotation", "true"))
				g.Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt"))

				v, ok := pod.ObjectMeta.Labels[dolt.RoleLabel]
				if !ok {
					return false
				}
				if v == string(doltv1alpha.ReplicationStateNotConfigured) {
					return false
				}

				if pod.Name == statefulset.PodName(testDoltDB.ObjectMeta, 0) {
					g.Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue(dolt.RoleLabel, dolt.PrimaryRoleValue.String()))
				} else {
					g.Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue(dolt.RoleLabel, dolt.StandbyRoleValue.String()))
				}
			}

			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a PodDisruptionBudget eventually")
		Eventually(func(g Gomega) bool {
			var pdb policyv1.PodDisruptionBudget
			if err := k8sClient.Get(ctx, testDoltDB.PodDisruptionBudgetKey(), &pdb); err != nil {
				return false
			}

			g.Expect(pdb.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(pdb.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
			g.Expect(pdb.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(pdb.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))

			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a PVCs eventually")
		Eventually(func(g Gomega) bool {
			pvcList := corev1.PersistentVolumeClaimList{}
			listOpts := client.ListOptions{
				LabelSelector: klabels.SelectorFromSet(
					builder.NewLabelsBuilder().
						WithDoltSelectorLabels(&testDoltDB).
						Build(),
				),
				Namespace: testDoltDB.GetNamespace(),
			}
			if err := k8sClient.List(ctx, &pvcList, &listOpts); err != nil {
				return false
			}

			if len(pvcList.Items) != int(testDoltDB.Spec.Replicas) {
				return false
			}

			for _, pvc := range pvcList.Items {
				g.Expect(pvc.ObjectMeta.Labels).NotTo(BeNil())
				g.Expect(pvc.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
				g.Expect(pvc.ObjectMeta.Annotations).NotTo(BeNil())
				g.Expect(pvc.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.dolthub.com/test", "test"))
				g.Expect(pvc.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", testDoltDB.Name))
				g.Expect(pvc.Spec.AccessModes).Should(ContainElement(corev1.ReadWriteOnce))
				storageSize := resource.NewQuantity(1, "Gi")
				if testDoltDB.Spec.Storage.Size != nil {
					storageSize = testDoltDB.Spec.Storage.Size
				}
				g.Expect(pvc.Spec.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceStorage, *storageSize))
			}

			return true
		}, testTimeout, testInterval).Should(BeTrue())
	})
})
