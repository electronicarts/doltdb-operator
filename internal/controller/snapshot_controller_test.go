// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Snapshot Controller", func() {

	BeforeEach(func() {
		By("Waiting for DoltDB to be ready")
		expectReady(ctx, k8sClient, testDoltKey)
	})

	It("should reconcile", func() {
		var testDoltDB doltv1alpha.DoltDB
		By("Getting DoltDB")
		Expect(k8sClient.Get(ctx, testDoltKey, &testDoltDB)).To(Succeed())

		By("Creating a Snapshot")
		snapshot := doltv1alpha.Snapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testDoltDBVolumeSnapshotKey.Name,
				Namespace: testDoltDBVolumeSnapshotKey.Namespace,
			},
			Spec: doltv1alpha.SnapshotSpec{
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name:      testDoltKey.Name,
						Namespace: testDoltDBVolumeSnapshotKey.Namespace,
					},
				},
				FrequencySchedule: ptr.To("* * * * *"),
			},
		}
		if err := k8sClient.Delete(ctx, &snapshot); err != nil {
			if !apierrors.IsNotFound(err) {
				GinkgoWriter.Printf("Error deleting snapshot: %v\n", err)
			}
		}

		Expect(k8sClient.Create(ctx, &snapshot)).To(Succeed())
		DeferCleanup(func() {
			By("Cleaning up External Snapshot")
			Expect(k8sClient.Delete(ctx, &snapshot)).To(Succeed())
		})
		By("Expecting Snapshot to be ready eventually")
		Eventually(func(g Gomega) bool {
			if err := k8sClient.Get(ctx, testDoltDBVolumeSnapshotKey, &snapshot); err != nil {
				return false
			}
			return snapshot.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		// TODO: should validate that a VolumeSnapshot is created
		// By("Expecting to create an VolumeSnapshot")
		// Eventually(func(g Gomega) bool {
		// 	var volumeSnapshot interface{}
		// 	if err := k8sClient.Get(ctx, testDoltDBVolumeSnapshotKey, &snapshot, ); err != nil {
		// 		return false
		// 	}

		// 	g.Expect(snapshot.ObjectMeta.Labels).NotTo(BeNil())
		// 	g.Expect(snapshot.ObjectMeta.Labels).ToNot(HaveKey("app.kubernetes.io/name"))
		// 	g.Expect(snapshot.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", testDoltDB.Name))

		// 	return snapshot.IsReady()
		// }, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting CronJob to be created eventually")
		Eventually(func(g Gomega) bool {
			var sts appsv1.StatefulSet
			if err := k8sClient.Get(ctx, testDoltKey, &sts); err != nil {
				return false
			}

			// retrieve Pods from the StatefulSet
			listOpts := &ctrlclient.ListOptions{
				LabelSelector: klabels.SelectorFromSet(
					builder.NewLabelsBuilder().
						WithDoltSelectorLabels(&testDoltDB).
						Build(),
				),
				Namespace: testDoltDBVolumeSnapshotKey.Namespace,
			}
			pvcList := corev1.PersistentVolumeClaimList{}
			if err := k8sClient.List(ctx, &pvcList, listOpts); err != nil {
				return false
			}

			// loop over pods and get pvcName
			for _, pvc := range pvcList.Items {
				var createdCronJob batchv1.CronJob
				// if cronjob not found, return false
				if err := k8sClient.Get(ctx, snapshot.CronJobKey(pvc.Name), &createdCronJob); err != nil {
					return false
				}

				g.Expect(createdCronJob.ObjectMeta.Labels).NotTo(BeNil())
				g.Expect(createdCronJob.ObjectMeta.Labels).ToNot(HaveKey("app.kubernetes.io/name"))
				g.Expect(createdCronJob.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", testDoltDB.Name))
				g.Expect(createdCronJob.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", snapshot.Name))
			}

			return true
		}, testHighTimeout, testInterval).Should(BeTrue())

	})
})
