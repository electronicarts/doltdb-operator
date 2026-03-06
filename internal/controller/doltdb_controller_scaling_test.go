// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("DoltDB Scaling Controller", Ordered, func() {
	var (
		scalingKey = types.NamespacedName{
			Name:      "dolt-scaling",
			Namespace: "default",
		}

		scalingCredsKey = types.NamespacedName{
			Name:      "dolt-scaling-credentials",
			Namespace: scalingKey.Namespace,
		}

		doltdbScaling *doltv1alpha.DoltDB
	)

	BeforeAll(func() {
		secret := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      scalingCredsKey.Name,
				Namespace: scalingCredsKey.Namespace,
			},
			Type: "Opaque",
			StringData: map[string]string{
				"admin-user":     "root",
				"admin-password": "12345",
			},
		}

		if err := k8sClient.Delete(ctx, &secret); err != nil {
			if err != client.IgnoreNotFound(err) {
				log.FromContext(ctx).Error(err, "error cleaning scaling test secret")
			}
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		// Start with a replicated cluster (2 replicas, replication enabled)
		doltdbScaling = &doltv1alpha.DoltDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      scalingKey.Name,
				Namespace: scalingKey.Namespace,
				Labels: map[string]string{
					"k8s.dolthub.com/test": "test",
				},
				Annotations: map[string]string{
					"k8s.dolthub.com/test": "test",
				},
			},
			Spec: doltv1alpha.DoltDBSpec{
				Image:               "dolthub/dolt",
				EngineVersion:       doltdbEngineVersion,
				Replicas:            2,
				ReplicationStrategy: doltv1alpha.DirectStandby,
				Storage: doltv1alpha.Storage{
					Size:             ptr.To(resource.MustParse("1Gi")),
					StorageClassName: ptr.To("standard-resize"),
					RetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
						WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
						WhenScaled:  appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
					},
				},
				Resources: &v1.ResourceRequirements{
					Requests: v1.ResourceList{
						"cpu":    resource.MustParse("500m"),
						"memory": resource.MustParse("1Gi"),
					},
					Limits: v1.ResourceList{
						"memory": resource.MustParse("1Gi"),
					},
				},
				Replication: &doltv1alpha.Replication{
					Enabled: true,
					ReplicationSpec: doltv1alpha.ReplicationSpec{
						Primary: &doltv1alpha.PrimaryReplication{
							PodIndex:          ptr.To(0),
							AutomaticFailover: ptr.To(true),
						},
					},
				},
				Probes: doltv1alpha.Probes{
					LivenessProbe: &corev1.Probe{
						InitialDelaySeconds: 20,
						PeriodSeconds:       10,
						TimeoutSeconds:      5,
					},
					ReadinessProbe: &corev1.Probe{
						InitialDelaySeconds: 15,
						PeriodSeconds:       10,
						TimeoutSeconds:      5,
					},
				},
			},
		}

		if err := k8sClient.Delete(ctx, doltdbScaling); err != nil {
			if err != client.IgnoreNotFound(err) {
				log.FromContext(ctx).Error(err, "error cleaning scaling test doltdb")
			}
		}

		By("Creating replicated DoltDB with Replicas=2")
		Expect(k8sClient.Create(ctx, doltdbScaling)).To(Succeed())
		DeferCleanup(func() {
			deleteDoltDB(ctx, scalingKey, scalingCredsKey)
		})
	})

	It("should start as a replicated cluster with 2 replicas", func() {
		By("Expecting DoltDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, scalingKey, doltdbScaling); err != nil {
				return false
			}
			return doltdbScaling.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting primary and reader services to exist")
		Eventually(func() error {
			var svc corev1.Service
			return k8sClient.Get(ctx, doltdbScaling.PrimaryServiceKey(), &svc)
		}, testTimeout, testInterval).Should(Succeed())

		Eventually(func() error {
			var svc corev1.Service
			return k8sClient.Get(ctx, doltdbScaling.ReaderServiceKey(), &svc)
		}, testTimeout, testInterval).Should(Succeed())

		By("Expecting standalone service to NOT exist")
		var standaloneSvc corev1.Service
		err := k8sClient.Get(ctx, doltdbScaling.ServiceKey(), &standaloneSvc)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

		By("Expecting PDB to exist")
		Eventually(func() error {
			var pdb policyv1.PodDisruptionBudget
			return k8sClient.Get(ctx, doltdbScaling.PodDisruptionBudgetKey(), &pdb)
		}, testTimeout, testInterval).Should(Succeed())

		By("Expecting pods to have role labels")
		Eventually(func(g Gomega) bool {
			var podList corev1.PodList
			listOpts := &client.ListOptions{
				LabelSelector: klabels.SelectorFromSet(
					builder.NewLabelsBuilder().
						WithDoltSelectorLabels(doltdbScaling).
						Build(),
				),
				Namespace: doltdbScaling.GetNamespace(),
			}
			if err := k8sClient.List(ctx, &podList, listOpts); err != nil {
				return false
			}
			g.Expect(podList.Items).To(HaveLen(2))
			for _, pod := range podList.Items {
				g.Expect(pod.Labels).To(HaveKey(dolt.RoleLabel))
			}
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should scale down from 2 replicas to 1 (standalone mode)", func() {
		By("Updating DoltDB to Replicas=1 with replication disabled")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, scalingKey, doltdbScaling); err != nil {
				return err
			}
			doltdbScaling.Spec.Replicas = 1
			doltdbScaling.Spec.Replication = &doltv1alpha.Replication{Enabled: false}
			return k8sClient.Update(ctx, doltdbScaling)
		}, testTimeout, testInterval).Should(Succeed())

		By("Expecting DoltDB to become ready with 1 replica")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, scalingKey, doltdbScaling); err != nil {
				return false
			}
			return doltdbScaling.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting StatefulSet with 1 replica")
		Eventually(func(g Gomega) bool {
			var sts appsv1.StatefulSet
			if err := k8sClient.Get(ctx, scalingKey, &sts); err != nil {
				return false
			}
			g.Expect(sts.Spec.Replicas).NotTo(BeNil())
			g.Expect(*sts.Spec.Replicas).To(Equal(int32(1)))
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting standalone service to exist")
		Eventually(func() error {
			var svc corev1.Service
			return k8sClient.Get(ctx, doltdbScaling.ServiceKey(), &svc)
		}, testTimeout, testInterval).Should(Succeed())

		By("Expecting primary and reader services to be cleaned up")
		Eventually(func() bool {
			var svc corev1.Service
			err := k8sClient.Get(ctx, doltdbScaling.PrimaryServiceKey(), &svc)
			return apierrors.IsNotFound(err)
		}, testTimeout, testInterval).Should(BeTrue())

		Eventually(func() bool {
			var svc corev1.Service
			err := k8sClient.Get(ctx, doltdbScaling.ReaderServiceKey(), &svc)
			return apierrors.IsNotFound(err)
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting PDB to be deleted for single instance")
		Eventually(func() bool {
			var pdb policyv1.PodDisruptionBudget
			err := k8sClient.Get(ctx, doltdbScaling.PodDisruptionBudgetKey(), &pdb)
			return apierrors.IsNotFound(err)
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting role labels to be removed from pods")
		Eventually(func(g Gomega) bool {
			var podList corev1.PodList
			listOpts := &client.ListOptions{
				LabelSelector: klabels.SelectorFromSet(
					builder.NewLabelsBuilder().
						WithDoltSelectorLabels(doltdbScaling).
						Build(),
				),
				Namespace: doltdbScaling.GetNamespace(),
			}
			if err := k8sClient.List(ctx, &podList, listOpts); err != nil {
				return false
			}
			g.Expect(podList.Items).To(HaveLen(1))
			for _, pod := range podList.Items {
				g.Expect(pod.Labels).NotTo(HaveKey(dolt.RoleLabel))
			}
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting ConfigMap without cluster section")
		Eventually(func(g Gomega) bool {
			var cm corev1.ConfigMap
			cmKey := doltdbScaling.DefaultConfigMapKey()
			if err := k8sClient.Get(ctx, cmKey, &cm); err != nil {
				return false
			}
			for _, v := range cm.Data {
				g.Expect(v).NotTo(ContainSubstring("cluster:"))
			}
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should scale up from 1 replica to 2 (replicated mode)", func() {
		By("Updating DoltDB to Replicas=2 with replication enabled")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, scalingKey, doltdbScaling); err != nil {
				return err
			}
			doltdbScaling.Spec.Replicas = 2
			doltdbScaling.Spec.Replication = &doltv1alpha.Replication{
				Enabled: true,
				ReplicationSpec: doltv1alpha.ReplicationSpec{
					Primary: &doltv1alpha.PrimaryReplication{
						PodIndex:          ptr.To(0),
						AutomaticFailover: ptr.To(true),
					},
				},
			}
			return k8sClient.Update(ctx, doltdbScaling)
		}, testTimeout, testInterval).Should(Succeed())

		By("Expecting DoltDB to become ready with 2 replicas")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, scalingKey, doltdbScaling); err != nil {
				return false
			}
			return doltdbScaling.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting StatefulSet with 2 replicas")
		Eventually(func(g Gomega) bool {
			var sts appsv1.StatefulSet
			if err := k8sClient.Get(ctx, scalingKey, &sts); err != nil {
				return false
			}
			g.Expect(sts.Spec.Replicas).NotTo(BeNil())
			g.Expect(*sts.Spec.Replicas).To(Equal(int32(2)))
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting primary and reader services to exist")
		Eventually(func() error {
			var svc corev1.Service
			return k8sClient.Get(ctx, doltdbScaling.PrimaryServiceKey(), &svc)
		}, testTimeout, testInterval).Should(Succeed())

		Eventually(func() error {
			var svc corev1.Service
			return k8sClient.Get(ctx, doltdbScaling.ReaderServiceKey(), &svc)
		}, testTimeout, testInterval).Should(Succeed())

		By("Expecting standalone service to be cleaned up")
		Eventually(func() bool {
			var svc corev1.Service
			err := k8sClient.Get(ctx, doltdbScaling.ServiceKey(), &svc)
			return apierrors.IsNotFound(err)
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting PDB to be re-created")
		Eventually(func() error {
			var pdb policyv1.PodDisruptionBudget
			return k8sClient.Get(ctx, doltdbScaling.PodDisruptionBudgetKey(), &pdb)
		}, testTimeout, testInterval).Should(Succeed())

		By("Expecting pods to have role labels")
		Eventually(func(g Gomega) bool {
			var podList corev1.PodList
			listOpts := &client.ListOptions{
				LabelSelector: klabels.SelectorFromSet(
					builder.NewLabelsBuilder().
						WithDoltSelectorLabels(doltdbScaling).
						Build(),
				),
				Namespace: doltdbScaling.GetNamespace(),
			}
			if err := k8sClient.List(ctx, &podList, listOpts); err != nil {
				return false
			}
			g.Expect(podList.Items).To(HaveLen(2))
			for _, pod := range podList.Items {
				g.Expect(pod.Labels).To(HaveKey(dolt.RoleLabel))
			}
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting ConfigMap with cluster section")
		Eventually(func(g Gomega) bool {
			var cm corev1.ConfigMap
			cmKey := doltdbScaling.DefaultConfigMapKey()
			if err := k8sClient.Get(ctx, cmKey, &cm); err != nil {
				return false
			}
			found := false
			for _, v := range cm.Data {
				if ok, _ := ContainSubstring("cluster:").Match(v); ok {
					found = true
				}
			}
			g.Expect(found).To(BeTrue())
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should transition primary to pod-0 before scale-down when primary is on higher ordinal", func() {
		By("Simulating primary on pod-1 via spec.replication.primary.podIndex")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, scalingKey, doltdbScaling); err != nil {
				return err
			}
			doltdbScaling.Spec.Replication.Primary.PodIndex = ptr.To(1)
			return k8sClient.Update(ctx, doltdbScaling)
		}, testTimeout, testInterval).Should(Succeed())

		By("Waiting for switchover to complete (primary moves to pod-1)")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, scalingKey, doltdbScaling); err != nil {
				return false
			}
			return doltdbScaling.IsReady() &&
				doltdbScaling.Status.CurrentPrimaryPodIndex != nil &&
				*doltdbScaling.Status.CurrentPrimaryPodIndex == 1
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Scaling down to 1 replica (should trigger pre-scale-down primary transition)")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, scalingKey, doltdbScaling); err != nil {
				return err
			}
			doltdbScaling.Spec.Replicas = 1
			doltdbScaling.Spec.Replication = &doltv1alpha.Replication{Enabled: false}
			return k8sClient.Update(ctx, doltdbScaling)
		}, testTimeout, testInterval).Should(Succeed())

		By("Expecting DoltDB to become ready with 1 replica")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, scalingKey, doltdbScaling); err != nil {
				return false
			}
			return doltdbScaling.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting StatefulSet with 1 replica")
		Eventually(func(g Gomega) bool {
			var sts appsv1.StatefulSet
			if err := k8sClient.Get(ctx, scalingKey, &sts); err != nil {
				return false
			}
			g.Expect(sts.Spec.Replicas).NotTo(BeNil())
			g.Expect(*sts.Spec.Replicas).To(Equal(int32(1)))
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	})
})
