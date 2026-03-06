// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Grant", func() {
	BeforeEach(func() {
		By("Waiting for DoltDB to be ready")
		expectReady(ctx, k8sClient, testDoltKey)
	})

	It("should grant privileges for all tables and databases", func() {
		By("Creating a User")
		userKey := types.NamespacedName{
			Name:      "grant-user-all-test",
			Namespace: testDoltKey.Namespace,
		}
		user := doltv1alpha.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userKey.Name,
				Namespace: userKey.Namespace,
			},
			Spec: doltv1alpha.UserSpec{
				Name: userKey.Name,
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name:      testDoltKey.Name,
						Namespace: testDoltKey.Namespace,
					},
				},
				PasswordSecretKeyRef: &doltv1alpha.SecretKeySelector{
					LocalObjectReference: doltv1alpha.LocalObjectReference{
						Name: testDoltAppUserPwdKey.Name,
					},
					Key: testDoltAppUserPwdSecretKey,
				},
			},
		}
		Expect(k8sClient.Create(ctx, &user)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
		})

		By("Expecting User to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, userKey, &user); err != nil {
				return false
			}
			return user.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Creating a Grant")
		grantKey := types.NamespacedName{
			Name:      "grant-select-insert-update-test",
			Namespace: testDoltKey.Namespace,
		}
		grant := doltv1alpha.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grantKey.Name,
				Namespace: grantKey.Namespace,
			},
			Spec: doltv1alpha.GrantSpec{
				SQLTemplate: doltv1alpha.SQLTemplate{
					RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
				},
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name:      testDoltKey.Name,
						Namespace: testDoltKey.Namespace,
					},
				},
				Privileges: []string{
					"SELECT",
					"INSERT",
					"UPDATE",
				},
				Database:    "*",
				Table:       "*",
				Username:    userKey.Name,
				GrantOption: true,
			},
		}
		Expect(k8sClient.Create(ctx, &grant)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, &grant)).To(Succeed())
		})

		By("Expecting Grant to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, grantKey, &grant); err != nil {
				return false
			}
			return grant.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Grant to eventually have finalizer")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, grantKey, &grant); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&grant, dolt.GrantFinalizerName)
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should grant privileges for a database", func() {
		By("Creating a Database")
		databaseKey := types.NamespacedName{
			Name:      "grantdbtest",
			Namespace: testDoltKey.Namespace,
		}
		database := doltv1alpha.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      databaseKey.Name,
				Namespace: databaseKey.Namespace,
			},
			Spec: doltv1alpha.DatabaseSpec{
				Name: ptr.To(databaseKey.Name),
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name:      testDoltKey.Name,
						Namespace: testDoltKey.Namespace,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &database)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, &database)).To(Succeed())
		})

		By("Expecting Database to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, databaseKey, &database); err != nil {
				return false
			}
			return database.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Creating a User")
		userKey := types.NamespacedName{
			Name:      "grant-user-database-test",
			Namespace: testDoltKey.Namespace,
		}
		user := doltv1alpha.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userKey.Name,
				Namespace: userKey.Namespace,
			},
			Spec: doltv1alpha.UserSpec{
				Name: userKey.Name,
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name:      testDoltKey.Name,
						Namespace: testDoltKey.Namespace,
					},
				},
				PasswordSecretKeyRef: &doltv1alpha.SecretKeySelector{
					LocalObjectReference: doltv1alpha.LocalObjectReference{
						Name: testDoltAppUserPwdKey.Name,
					},
					Key: testDoltAppUserPwdSecretKey,
				},
			},
		}
		Expect(k8sClient.Create(ctx, &user)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
		})

		By("Expecting User to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, userKey, &user); err != nil {
				return false
			}
			return user.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Creating a Grant")
		grantKey := types.NamespacedName{
			Name:      "grant-all-test",
			Namespace: testDoltKey.Namespace,
		}
		grant := doltv1alpha.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grantKey.Name,
				Namespace: grantKey.Namespace,
			},
			Spec: doltv1alpha.GrantSpec{
				SQLTemplate: doltv1alpha.SQLTemplate{
					RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
				},
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name:      testDoltKey.Name,
						Namespace: testDoltKey.Namespace,
					},
				},
				Privileges: []string{
					"ALL",
				},
				Database:    testDatabaseKey.Name,
				Table:       "*",
				Username:    userKey.Name,
				GrantOption: true,
			},
		}
		Expect(k8sClient.Create(ctx, &grant)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, &grant)).To(Succeed())
		})

		By("Expecting Grant to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, grantKey, &grant); err != nil {
				return false
			}
			return grant.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Grant to eventually have finalizer")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, grantKey, &grant); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&grant, dolt.GrantFinalizerName)
		}, testTimeout, testInterval).Should(BeTrue())
	})
})
