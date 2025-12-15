// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var (
	testHighTimeout = 5 * time.Minute
	testTimeout     = 2 * time.Minute
	testInterval    = 1 * time.Second

	testDoltKey = types.NamespacedName{
		Name:      "dolt",
		Namespace: "default",
	}

	testDoltCredentialsKey = types.NamespacedName{
		Name:      "dolt-credentials",
		Namespace: testDoltKey.Namespace,
	}

	testDoltDBVolumeSnapshotKey = types.NamespacedName{
		Name:      "dolt-volume-snapshot",
		Namespace: testDoltKey.Namespace,
	}

	testDatabaseKey = types.NamespacedName{
		Name:      "dolt-database-create-test",
		Namespace: testDoltKey.Namespace,
	}

	testDoltAppUserPwdKey = types.NamespacedName{
		Name:      "dolt-app-user",
		Namespace: testDoltKey.Namespace,
	}
	testDoltAppUserPwdSecretKey = "dolt-user-secret-key"
)

func testCreateInitialData(ctx context.Context) {
	createDefaultDoltUsers(ctx)

	doltdb := doltv1alpha.DoltDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDoltKey.Name,
			Namespace: testDoltKey.Namespace,
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
				VolumeSnapshot: "dolt-volume-snapshot",
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
			PodAnnotations: map[string]string{
				"pod-annotation": "true",
			},
			Server: doltv1alpha.Server{
				LogLevel: "trace",
				Listener: doltv1alpha.Listener{
					MaxConnections: 1024,
				},
				Metrics: &doltv1alpha.Metrics{
					Enabled: true,
					Labels: map[string]string{
						"env": "integration-test",
					},
				},
			},
			GlobalConfig: doltv1alpha.GlobalConfig{
				DisableClientUsageMetricsCollection: ptr.To(false),
				CommitAuthor: doltv1alpha.CommitAuthor{
					Name:  "dolt kubernetes deployment",
					Email: "dolt@kubernetes.deployment",
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

	if err := k8sClient.Delete(ctx, &doltdb); err != nil {
		if err != client.IgnoreNotFound(err) {
			log.FromContext(ctx).Error(err, "error cleaning test environment doltdb")
		}
	}
	Expect(k8sClient.Create(ctx, &doltdb)).To(Succeed())
	expectReady(ctx, k8sClient, testDoltKey)
}

func testCleanupInitialData(ctx context.Context) {
	By("Deleting App user Secret")
	var doltAppUserSecret v1.Secret
	Expect(k8sClient.Get(ctx, testDoltAppUserPwdKey, &doltAppUserSecret)).To(Succeed())
	Expect(k8sClient.Delete(ctx, &doltAppUserSecret)).To(Succeed())

	deleteDoltDB(ctx, testDoltKey, testDoltCredentialsKey)
}

func expectReady(ctx context.Context, k8sClient client.Client, key types.NamespacedName) {
	By("Expecting DoltDB to be ready eventually")
	expectFn(ctx, k8sClient, key, func(doltdb *doltv1alpha.DoltDB) bool {
		return doltdb.IsReady()
	})
}

func createDefaultDoltUsers(ctx context.Context) {
	By("Creating Admin user Secret")
	adminUserTest := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDoltCredentialsKey.Name,
			Namespace: testDoltCredentialsKey.Namespace,
		},
		Type: "Opaque",
		StringData: map[string]string{
			"admin-user":     "admin-user-test",
			"admin-password": "12345",
		},
	}
	if err := k8sClient.Delete(ctx, &adminUserTest); err != nil {
		if err != client.IgnoreNotFound(err) {
			log.FromContext(ctx).Error(err, "error cleaning test environment doltdb secret")
		}
	}
	Expect(k8sClient.Create(ctx, &adminUserTest)).To(Succeed())

	By("Creating App user Secret")
	defaultAppUser := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDoltAppUserPwdKey.Name,
			Namespace: testDoltAppUserPwdKey.Namespace,
			Labels: map[string]string{
				dolt.WatchLabel: "true",
			},
		},
		Type: "Opaque",
		StringData: map[string]string{
			testDoltAppUserPwdSecretKey: "dolt!#123",
		},
	}
	if err := k8sClient.Delete(ctx, &defaultAppUser); err != nil {
		if err != client.IgnoreNotFound(err) {
			log.FromContext(ctx).Error(err, "error cleaning test environment doltdb secret")
		}
	}
	Expect(k8sClient.Create(ctx, &defaultAppUser)).To(Succeed())
}

func expectFn(ctx context.Context, k8sClient client.Client, key types.NamespacedName, fn func(doltdb *doltv1alpha.DoltDB) bool) {
	var doltdb doltv1alpha.DoltDB
	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(ctx, key, &doltdb)).To(Succeed())
		return fn(&doltdb)
	}, testHighTimeout, testInterval).Should(BeTrue())
}

func deleteDoltDB(ctx context.Context, key types.NamespacedName, adminUserSecretKey types.NamespacedName) {
	By("Deleting Admin user Secret")
	var doltAdminUserSecret v1.Secret
	Expect(k8sClient.Get(ctx, adminUserSecretKey, &doltAdminUserSecret)).To(Succeed())
	Expect(k8sClient.Delete(ctx, &doltAdminUserSecret)).To(Succeed())

	var doltdb doltv1alpha.DoltDB
	By("Deleting DoltDB")
	Expect(k8sClient.Get(ctx, key, &doltdb)).To(Succeed())
	Expect(k8sClient.Delete(ctx, &doltdb)).To(Succeed())

	By("Deleting PVCs")
	opts := []client.DeleteAllOfOption{
		client.MatchingLabels(
			builder.NewLabelsBuilder().
				WithDoltSelectorLabels(&doltdb).
				Build(),
		),
		client.InNamespace(doltdb.Namespace),
	}
	Expect(k8sClient.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, opts...)).To(Succeed())
}

func testDoltDBStorageResize(doltdb *doltv1alpha.DoltDB, newVolumeSize string) {
	key := client.ObjectKeyFromObject(doltdb)

	By("Updating storage")
	doltdb.Spec.Storage.Size = ptr.To(resource.MustParse(newVolumeSize))
	Expect(k8sClient.Update(ctx, doltdb)).To(Succeed())

	By("Expecting DoltDB to have resized storage eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, key, doltdb); err != nil {
			return false
		}
		return doltdb.IsReady() && meta.IsStatusConditionTrue(doltdb.Status.Conditions, doltv1alpha.ConditionTypeStorageResized)
	}, testHighTimeout, testInterval).Should(BeTrue())

	By("Expecting StatefulSet storage to have been resized")
	var sts appsv1.StatefulSet
	Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
	doltDBSize := doltdb.Spec.Storage.Size
	stsSize := statefulset.GetStorageSize(&sts, builder.DoltDataVolume)
	Expect(doltDBSize).NotTo(BeNil())
	Expect(stsSize).NotTo(BeNil())
	Expect(doltDBSize.Cmp(*stsSize)).To(Equal(0))

	By("Expecting PVCs to have been resized")
	pvcList := corev1.PersistentVolumeClaimList{}
	listOpts := client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			builder.NewLabelsBuilder().
				WithDoltSelectorLabels(doltdb).
				WithPVCRole(builder.DoltDataVolume).
				Build(),
		),
		Namespace: doltdb.GetNamespace(),
	}
	Expect(k8sClient.List(ctx, &pvcList, &listOpts)).To(Succeed())
	for _, p := range pvcList.Items {
		pvcSize := p.Spec.Resources.Requests[corev1.ResourceStorage]
		Expect(doltDBSize.Cmp(pvcSize)).To(Equal(0))
	}
}

func testDoltDBUpdate(doltdb *doltv1alpha.DoltDB) {
	key := client.ObjectKeyFromObject(doltdb)

	By("Updating DoltDB compute resources")
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, key, doltdb); err != nil {
			return false
		}

		doltdb.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		}

		doltdb.Annotations["k8s.dolthub.com/updated-at"] = time.Now().String()

		return k8sClient.Update(ctx, doltdb) == nil
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting DoltDB to be updated eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, key, doltdb); err != nil {
			return false
		}
		return doltdb.IsReady() && meta.IsStatusConditionTrue(doltdb.Status.Conditions, doltv1alpha.ConditionTypeUpdated)
	}, testHighTimeout, testInterval).Should(BeTrue())
}

func testSQLConnection(ctx context.Context, doltdb *doltv1alpha.DoltDB, username string, pwdSecretKeyRef doltv1alpha.SecretKeySelector) {
	By("Resolve DoltDB Password secret")
	password, err := refResolver.SecretKeyRef(ctx, pwdSecretKeyRef, doltdb.Namespace)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() bool {
		By("Checking connection to DoltDB")
		client, err := sql.NewClientWithDoltDB(
			ctx,
			doltdb,
			refResolver,
			sql.WithPassword(password),
			sql.WithUsername(username),
		)
		if err != nil {
			return false
		}
		defer func() {
			err := client.Close()
			Expect(err).NotTo(HaveOccurred())
		}()

		return true
	}, testTimeout, testInterval).Should(BeTrue())
}

// findServicePort is a helper function to find a service port by name
func findServicePort(ports []corev1.ServicePort, name string) *corev1.ServicePort {
	for i := range ports {
		if ports[i].Name == name {
			return &ports[i]
		}
	}
	return nil
}
