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

var _ = Describe("Backup Controller", func() {
	BeforeEach(func() {
		By("Waiting for DoltDB to be ready")
		expectReady(ctx, k8sClient, testDoltKey)
	})

	It("should reconcile a backup to completion", func() {
		By("Creating a Backup with local storage")
		backup := doltv1alpha.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testBackupKey.Name,
				Namespace: testBackupKey.Namespace,
			},
			Spec: doltv1alpha.BackupSpec{
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name: testDoltKey.Name,
					},
				},
				Storage: doltv1alpha.BackupStorage{
					Local: &doltv1alpha.LocalBackupStorage{
						Path: "/tmp/dolt-backup-test",
					},
				},
			},
		}

		if err := k8sClient.Delete(ctx, &backup); err != nil {
			if !apierrors.IsNotFound(err) {
				GinkgoWriter.Printf("Error deleting Backup: %v\n", err)
			}
		}

		Expect(k8sClient.Create(ctx, &backup)).To(Succeed())
		DeferCleanup(func() {
			By("Cleaning up Backup")
			_ = k8sClient.Delete(ctx, &backup)
		})

		By("Expecting Backup phase to reach a terminal state")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, testBackupKey, &backup); err != nil {
				return false
			}
			return backup.IsCompleted() || backup.IsFailed()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting Backup to have completed successfully")
		Expect(backup.IsCompleted()).To(
			BeTrue(),
			"expected backup to complete but got phase: %s, error: %s",
			backup.Status.Phase,
			backup.Status.Error,
		)

		By("Expecting Backup phase to be Completed")
		Expect(backup.Status.Phase).To(Equal(doltv1alpha.BackupPhaseCompleted))

		By("Expecting no error message")
		Expect(backup.Status.Error).To(BeEmpty())

		By("Expecting Backup to have StartedAt set")
		Expect(backup.Status.StartedAt).NotTo(BeNil())

		By("Expecting Backup to have CompletedAt set")
		Expect(backup.Status.CompletedAt).NotTo(BeNil())
	})

	It("should not run concurrently with another backup for the same DoltDB", func() {
		By("Creating the first Backup and waiting for it to start running")
		first := doltv1alpha.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backup-concurrent-first",
				Namespace: testBackupKey.Namespace,
			},
			Spec: doltv1alpha.BackupSpec{
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name: testDoltKey.Name,
					},
				},
				Storage: doltv1alpha.BackupStorage{
					Local: &doltv1alpha.LocalBackupStorage{
						Path: "/tmp/dolt-backup-concurrent-1",
					},
				},
			},
		}
		_ = k8sClient.Delete(ctx, &first)
		Expect(k8sClient.Create(ctx, &first)).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, &first)
		})

		By("Creating a second Backup for the same DoltDB")
		second := doltv1alpha.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backup-concurrent-second",
				Namespace: testBackupKey.Namespace,
			},
			Spec: doltv1alpha.BackupSpec{
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name: testDoltKey.Name,
					},
				},
				Storage: doltv1alpha.BackupStorage{
					Local: &doltv1alpha.LocalBackupStorage{
						Path: "/tmp/dolt-backup-concurrent-2",
					},
				},
			},
		}
		_ = k8sClient.Delete(ctx, &second)
		Expect(k8sClient.Create(ctx, &second)).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, &second)
		})

		By("Expecting both backups to eventually complete (not fail due to concurrency)")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&first), &first); err != nil {
				return false
			}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&second), &second); err != nil {
				return false
			}
			return (first.IsCompleted() || first.IsFailed()) &&
				(second.IsCompleted() || second.IsFailed())
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting both backups completed successfully")
		Expect(first.IsCompleted()).To(BeTrue(),
			"first backup phase: %s, error: %s", first.Status.Phase, first.Status.Error)
		Expect(second.IsCompleted()).To(BeTrue(),
			"second backup phase: %s, error: %s", second.Status.Phase, second.Status.Error)
	})

	It("should fail when DoltDB reference does not exist", func() {
		By("Creating a Backup with a nonexistent DoltDB ref")
		failBackup := doltv1alpha.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backup-fail-test",
				Namespace: testBackupKey.Namespace,
			},
			Spec: doltv1alpha.BackupSpec{
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name: "nonexistent-doltdb",
					},
				},
				Storage: doltv1alpha.BackupStorage{
					Local: &doltv1alpha.LocalBackupStorage{
						Path: "/tmp/dolt-backup-fail-test",
					},
				},
			},
		}

		// Clean up if exists
		_ = k8sClient.Delete(ctx, &failBackup)

		Expect(k8sClient.Create(ctx, &failBackup)).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, &failBackup)
		})

		By("Expecting Backup to have Pending phase (DoltDB never becomes ready)")
		Consistently(func() bool {
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&failBackup), &failBackup); err != nil {
				return false
			}
			return failBackup.Status.Phase == "" || failBackup.Status.Phase == doltv1alpha.BackupPhasePending
		}, "10s", testInterval).Should(BeTrue())
	})
})
