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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
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
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, testDoltDBVolumeSnapshotKey, &snapshot); err != nil {
				return false
			}
			return snapshot.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

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
			}

			return true
		}, testHighTimeout, testInterval).Should(BeTrue())

	})
})
