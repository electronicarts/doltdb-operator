// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("DoltDB Standalone Controller", Ordered, func() {
	var (
		standaloneKey = types.NamespacedName{
			Name:      "dolt-standalone",
			Namespace: "default",
		}

		standaloneCredsKey = types.NamespacedName{
			Name:      "dolt-standalone-credentials",
			Namespace: standaloneKey.Namespace,
		}

		doltdbStandalone *doltv1alpha.DoltDB
	)

	BeforeAll(func() {
		secret := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      standaloneCredsKey.Name,
				Namespace: standaloneCredsKey.Namespace,
			},
			Type: "Opaque",
			StringData: map[string]string{
				"admin-user":     "root",
				"admin-password": "12345",
			},
		}

		if err := k8sClient.Delete(ctx, &secret); err != nil {
			if err != client.IgnoreNotFound(err) {
				log.FromContext(ctx).Error(err, "error cleaning standalone test secret")
			}
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		doltdbStandalone = &doltv1alpha.DoltDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      standaloneKey.Name,
				Namespace: standaloneKey.Namespace,
				Labels: map[string]string{
					"k8s.dolthub.com/test": "test",
				},
				Annotations: map[string]string{
					"k8s.dolthub.com/test": "test",
				},
			},
			Spec: doltv1alpha.DoltDBSpec{
				Image:         "dolthub/dolt",
				EngineVersion: doltdbEngineVersion,
				Replicas:      1,
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

		if err := k8sClient.Delete(ctx, doltdbStandalone); err != nil {
			if err != client.IgnoreNotFound(err) {
				log.FromContext(ctx).Error(err, "error cleaning standalone test doltdb")
			}
		}

		By("Creating standalone DoltDB with Replicas=1 and no replication")
		Expect(k8sClient.Create(ctx, doltdbStandalone)).To(Succeed())
		DeferCleanup(func() {
			deleteDoltDB(ctx, standaloneKey, standaloneCredsKey)
		})
	})

	It("should reconcile a standalone DoltDB instance", func() {
		By("Expecting DoltDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, standaloneKey, doltdbStandalone); err != nil {
				return false
			}
			return doltdbStandalone.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting ConfigMap without cluster section")
		Eventually(func(g Gomega) bool {
			var cm corev1.ConfigMap
			cmKey := doltdbStandalone.DefaultConfigMapKey()
			if err := k8sClient.Get(ctx, cmKey, &cm); err != nil {
				return false
			}
			g.Expect(cm.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "dolt-standalone"))

			data, err := dolt.GenerateConfigMapData(doltdbStandalone)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cm.Data).To(Equal(data))

			// Verify the config does not contain cluster section
			for _, v := range cm.Data {
				g.Expect(v).NotTo(ContainSubstring("cluster:"))
			}

			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting StatefulSet with 1 replica")
		Eventually(func(g Gomega) bool {
			var sts appsv1.StatefulSet
			if err := k8sClient.Get(ctx, standaloneKey, &sts); err != nil {
				return false
			}
			g.Expect(sts.Spec.Replicas).NotTo(BeNil())
			g.Expect(*sts.Spec.Replicas).To(Equal(int32(1)))
			g.Expect(sts.ObjectMeta.Labels).To(
				HaveKeyWithValue("app.kubernetes.io/name", "dolt-standalone"),
			)
			g.Expect(sts.ObjectMeta.Annotations).To(
				HaveKeyWithValue("k8s.dolthub.com/doltdb", doltdbStandalone.Name),
			)
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting internal headless Service to exist")
		Eventually(func(g Gomega) bool {
			var svc corev1.Service
			if err := k8sClient.Get(ctx, doltdbStandalone.InternalServiceKey(), &svc); err != nil {
				return false
			}
			g.Expect(svc.Spec.ClusterIP).To(Equal("None"))
			g.Expect(svc.Spec.Selector).To(
				HaveKeyWithValue("app.kubernetes.io/name", doltdbStandalone.Name),
			)
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting standalone ClusterIP Service to exist")
		Eventually(func(g Gomega) bool {
			var svc corev1.Service
			if err := k8sClient.Get(ctx, doltdbStandalone.ServiceKey(), &svc); err != nil {
				return false
			}
			g.Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			g.Expect(svc.Spec.Selector).To(
				HaveKeyWithValue("app.kubernetes.io/name", doltdbStandalone.Name),
			)

			mysqlPort := findServicePort(svc.Spec.Ports, builder.DoltMySQLPortName)
			g.Expect(mysqlPort).NotTo(BeNil())
			g.Expect(mysqlPort.Port).To(Equal(doltdbStandalone.Spec.Server.Listener.Port))

			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting primary Service to NOT exist")
		var primarySvc corev1.Service
		err := k8sClient.Get(ctx, doltdbStandalone.PrimaryServiceKey(), &primarySvc)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

		By("Expecting reader Service to NOT exist")
		var readerSvc corev1.Service
		err = k8sClient.Get(ctx, doltdbStandalone.ReaderServiceKey(), &readerSvc)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

		By("Expecting a single Pod with correct labels")
		Eventually(func(g Gomega) bool {
			var podList corev1.PodList
			listOpts := &client.ListOptions{
				LabelSelector: klabels.SelectorFromSet(
					builder.NewLabelsBuilder().
						WithDoltSelectorLabels(doltdbStandalone).
						Build(),
				),
				Namespace: doltdbStandalone.GetNamespace(),
			}
			if err := k8sClient.List(ctx, &podList, listOpts); err != nil {
				return false
			}
			g.Expect(podList.Items).To(HaveLen(1))

			pod := podList.Items[0]
			g.Expect(pod.ObjectMeta.Labels).To(
				HaveKeyWithValue("app.kubernetes.io/name", doltdbStandalone.Name),
			)
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should connect to the standalone instance via SQL", func() {
		By("Expecting SQL connection to standalone DoltDB")
		Eventually(func(g Gomega) bool {
			sqlClient, err := sql.NewClientWithDoltDB(ctx, doltdbStandalone, refResolver)
			if err != nil {
				return false
			}
			defer func() {
				g.Expect(sqlClient.Close()).To(Succeed())
			}()
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should resize storage", func() {
		By("Resizing standalone DoltDB PVCs")
		testDoltDBStorageResize(doltdbStandalone, "10Gi")
	})

	It("should update", func() {
		By("Updating standalone DoltDB")
		testDoltDBUpdate(doltdbStandalone)
	})
})
