package controller

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("DoltDB Replication Controller", Ordered, func() {
	var (
		doltReplKey = types.NamespacedName{
			Name:      "dolt-repl",
			Namespace: "default",
		}

		doltReplCredsKey = types.NamespacedName{
			Name:      "dolt-repl-credentials",
			Namespace: doltReplKey.Namespace,
		}

		doltdbRepl *doltv1alpha.DoltDB
	)

	BeforeAll(func() {
		secret := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      doltReplCredsKey.Name,
				Namespace: doltReplCredsKey.Namespace,
			},
			Type: "Opaque",
			StringData: map[string]string{
				"admin-user":     "root",
				"admin-password": "12345",
			},
		}

		err := k8sClient.Delete(ctx, &secret)
		if err != nil {
			if err != client.IgnoreNotFound(err) {
				log.FromContext(ctx).Error(err, fmt.Sprintf("error cleaning test environment doltdb %s secret", doltReplCredsKey.Name))
			}
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		doltdbRepl = &doltv1alpha.DoltDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      doltReplKey.Name,
				Namespace: doltReplKey.Namespace,
				Labels: map[string]string{
					"k8s.dolthub.com/test": "test",
				},
				Annotations: map[string]string{
					"k8s.dolthub.com/test": "test",
				},
			},
			Spec: doltv1alpha.DoltDBSpec{
				Image:               "dolthub/dolt",
				EngineVersion:       "1.45.1",
				Replicas:            3,
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
						InitialDelaySeconds: 15,
						PeriodSeconds:       20,
					},
					ReadinessProbe: &corev1.Probe{
						InitialDelaySeconds: 15,
						PeriodSeconds:       20,
					},
				},
			},
		}

		if err := k8sClient.Delete(ctx, doltdbRepl); err != nil {
			if err != client.IgnoreNotFound(err) {
				log.FromContext(ctx).Error(err, "error cleaning test environment doltdb")
			}
		}

		By("Creating DoltDB with replication")
		Expect(k8sClient.Create(ctx, doltdbRepl)).To(Succeed())
		DeferCleanup(func() {
			deleteDoltDB(ctx, doltReplKey, doltReplCredsKey)
		})
	})

	It("should reconcile", func() {
		By("Expecting DoltDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, doltReplKey, doltdbRepl); err != nil {
				return false
			}
			return doltdbRepl.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		var svc corev1.Service

		By("Expecting to create a Service")
		Expect(k8sClient.Get(ctx, doltdbRepl.InternalServiceKey(), &svc)).To(Succeed())

		By("Expecting to create a primary Service")
		Expect(k8sClient.Get(ctx, doltdbRepl.PrimaryServiceKey(), &svc)).To(Succeed())
		Expect(svc.Spec.Selector["statefulset.kubernetes.io/pod-name"]).To(Equal(statefulset.PodName(doltdbRepl.ObjectMeta, 0)))

		By("Expecting to create a reader Service")
		Expect(k8sClient.Get(ctx, doltdbRepl.ReaderServiceKey(), &svc)).To(Succeed())

		By("Expecting role label to be set to primary")
		Eventually(func() bool {
			if doltdbRepl.Status.CurrentPrimary == nil {
				return false
			}

			currentPrimary := *doltdbRepl.Status.CurrentPrimary
			primaryPodKey := types.NamespacedName{
				Name:      currentPrimary,
				Namespace: doltdbRepl.Namespace,
			}
			var primaryPod corev1.Pod
			if err := k8sClient.Get(ctx, primaryPodKey, &primaryPod); err != nil {
				return apierrors.IsNotFound(err)
			}

			return primaryPod.Labels[string(dolt.RoleLabel)] == dolt.PrimaryRoleValue.String()
		}, testHighTimeout, testInterval).Should(BeTrue())

		var pdb policyv1.PodDisruptionBudget
		Expect(k8sClient.Get(ctx, doltdbRepl.PodDisruptionBudgetKey(), &pdb)).To(Succeed())

		By("Expecting SQL Connection to primary to be ready eventually")
		Eventually(func(g Gomega) bool {
			sqlClient, err := sql.NewClientWithDoltDB(ctx, doltdbRepl, refResolver)
			if err != nil {
				return false
			}
			defer sqlClient.Close()
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting SQL Connection to replicas to be ready eventually")
		Eventually(func(g Gomega) bool {
			host := statefulset.ServiceFQDNWithService(
				doltdbRepl.ObjectMeta,
				doltdbRepl.ReaderServiceKey().Name,
			)
			sqlClient, err := sql.NewClientWithDoltDB(ctx, doltdbRepl, refResolver, sql.WitHost(host))
			if err != nil {
				return false
			}
			defer sqlClient.Close()
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should fail and switch over primary", func() {
		By("Expecting DoltDB primary to be set")
		Eventually(func() bool {
			return doltdbRepl.Status.CurrentPrimary != nil
		}, testTimeout, testInterval).Should(BeTrue())

		currentPrimary := *doltdbRepl.Status.CurrentPrimary
		By("Tearing down primary Pod consistently")
		Consistently(func() bool {
			primaryPodKey := types.NamespacedName{
				Name:      currentPrimary,
				Namespace: doltdbRepl.Namespace,
			}
			var primaryPod corev1.Pod
			if err := k8sClient.Get(ctx, primaryPodKey, &primaryPod); err != nil {
				return apierrors.IsNotFound(err)
			}
			return k8sClient.Delete(ctx, &primaryPod) == nil
		}, 10*time.Second, testInterval).Should(BeTrue())

		By("Expecting DoltDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, doltReplKey, doltdbRepl); err != nil {
				return false
			}
			return doltdbRepl.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting DoltDB to eventually change primary")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, doltReplKey, doltdbRepl); err != nil {
				return false
			}
			if !doltdbRepl.IsReady() || doltdbRepl.Status.CurrentPrimary == nil {
				return false
			}
			return *doltdbRepl.Status.CurrentPrimary != currentPrimary
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting DoltDB to eventually update primary")
		var podIndex int
		for i := 0; i < int(doltdbRepl.Spec.Replicas); i++ {
			if i != *doltdbRepl.Status.CurrentPrimaryPodIndex {
				podIndex = i
				break
			}
		}
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(ctx, doltReplKey, doltdbRepl)).To(Succeed())
			doltdbRepl.Replication().Primary.PodIndex = &podIndex
			g.Expect(k8sClient.Update(ctx, doltdbRepl)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting DoltDB to eventually change primary")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, doltReplKey, doltdbRepl); err != nil {
				return false
			}
			if !doltdbRepl.IsReady() || doltdbRepl.Status.CurrentPrimaryPodIndex == nil {
				return false
			}
			return *doltdbRepl.Status.CurrentPrimaryPodIndex == podIndex
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting primary Service to eventually change primary")
		Eventually(func(g Gomega) bool {
			var svc corev1.Service
			if err := k8sClient.Get(ctx, doltdbRepl.PrimaryServiceKey(), &svc); err != nil {
				return false
			}
			return svc.Spec.Selector["statefulset.kubernetes.io/pod-name"] == statefulset.PodName(doltdbRepl.ObjectMeta, podIndex)
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should resize storage", func() {
		By("Resizing DoltDB PVCs")
		testDoltDBStorageResize(doltdbRepl, "10Gi")
	})

	It("should update", func() {
		By("Updating DoltDB")
		testDoltDBUpdate(doltdbRepl)
	})
})
