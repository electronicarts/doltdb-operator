// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("BackupSchedule Controller", func() {
	BeforeEach(func() {
		By("Waiting for DoltDB to be ready")
		expectReady(ctx, k8sClient, testDoltKey)
	})

	It("should create Backup objects on schedule", func() {
		By("Creating a BackupSchedule with every-minute cron")
		bs := doltv1alpha.BackupSchedule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testBackupScheduleKey.Name,
				Namespace: testBackupScheduleKey.Namespace,
			},
			Spec: doltv1alpha.BackupScheduleSpec{
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name: testDoltKey.Name,
					},
				},
				Storage: doltv1alpha.BackupStorage{
					Local: &doltv1alpha.LocalBackupStorage{
						Path: "/tmp/dolt-backup-schedule-test",
					},
				},
				Schedule: "* * * * *",
			},
		}

		if err := k8sClient.Delete(ctx, &bs); err != nil {
			if !apierrors.IsNotFound(err) {
				GinkgoWriter.Printf("Error deleting BackupSchedule: %v\n", err)
			}
		}

		Expect(k8sClient.Create(ctx, &bs)).To(Succeed())
		DeferCleanup(func() {
			By("Cleaning up BackupSchedule")
			_ = k8sClient.Delete(ctx, &bs)
		})

		By("Expecting BackupSchedule to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, testBackupScheduleKey, &bs); err != nil {
				return false
			}
			return bs.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting BackupSchedule to have NextScheduleTime set")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, testBackupScheduleKey, &bs); err != nil {
				return false
			}
			return bs.Status.NextScheduleTime != nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting at least one Backup to be created by the schedule")
		Eventually(func() bool {
			var backupList doltv1alpha.BackupList
			if err := k8sClient.List(ctx, &backupList, client.InNamespace(testBackupScheduleKey.Namespace),
				client.MatchingLabels{"k8s.dolthub.com/backup-schedule": testBackupScheduleKey.Name}); err != nil {
				return false
			}
			return len(backupList.Items) > 0
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting BackupSchedule to have LastScheduleTime set")
		Expect(k8sClient.Get(ctx, testBackupScheduleKey, &bs)).To(Succeed())
		Expect(bs.Status.LastScheduleTime).NotTo(BeNil())
		Expect(bs.Status.LastBackupRef).NotTo(BeEmpty())
	})
})
