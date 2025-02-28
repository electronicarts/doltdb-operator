package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Database Controller", func() {
	BeforeEach(func() {
		By("Waiting for DoltDB to be ready")
		expectReady(ctx, k8sClient, testDoltKey)
	})

	It("should reconcile", func() {
		var testDoltDB doltv1alpha.DoltDB

		By("Getting DoltDB")
		Expect(k8sClient.Get(ctx, testDoltKey, &testDoltDB)).To(Succeed())

		By("Creating a Database")
		database := doltv1alpha.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testDatabaseKey.Name,
				Namespace: testDatabaseKey.Namespace,
			},
			Spec: doltv1alpha.DatabaseSpec{
				Name: ptr.To("testdb"),
				SystemBranches: []string{
					"master",
					"global",
				},
				DoltIgnorePatterns: []string{
					"log_%",
				},
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name: testDoltKey.Name,
					},
				},
			},
		}

		if err := k8sClient.Delete(ctx, &database); err != nil {
			if !apierrors.IsNotFound(err) {
				GinkgoWriter.Printf("Error deleting Database: %v\n", err)
			}
		}

		Expect(k8sClient.Create(ctx, &database)).To(Succeed())
		DeferCleanup(func() {
			By("Cleaning up Database")
			Expect(k8sClient.Delete(ctx, &database)).To(Succeed())
		})

		By("Expecting Database to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, testDatabaseKey, &database); err != nil {
				return false
			}
			return database.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting Database to eventually have finalizer")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, testDatabaseKey, &database); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&database, databaseFinalizerName)
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Database to be created eventually")
		Eventually(func(g Gomega) bool {
			sqlClient, err := sql.NewClientWithDoltDB(ctx, &testDoltDB, refResolver)
			if err != nil {
				return false
			}
			defer func() {
				g.Expect(sqlClient.Close()).To(Succeed())
			}()

			dbs, err := sqlClient.Query(ctx, "SHOW DATABASES")
			if err != nil {
				return false
			}
			defer func() {
				err := dbs.Close()
				g.Expect(err).NotTo(HaveOccurred())
			}()

			var dbCreated bool
			for dbs.Next() {
				var dbName string
				if err := dbs.Scan(&dbName); err != nil {
					return false
				}
				if dbName == *database.Spec.Name {
					return true
				}
			}

			return dbCreated
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting DoltIgnore to be created eventually")
		Eventually(func(g Gomega) bool {
			sqlClient, err := sql.NewClientWithDoltDB(ctx, &testDoltDB, refResolver)
			if err != nil {
				return false
			}
			defer func() {
				g.Expect(sqlClient.Close()).To(Succeed())
			}()

			err = sqlClient.UseDatabase(ctx, *database.Spec.Name)
			g.Expect(err).NotTo(HaveOccurred())

			doltIgnores, err := sqlClient.GetDoltIgnore(ctx)
			if err != nil {
				return false
			}

			doltIgnore := make(map[string]struct{})
			for _, di := range doltIgnores {
				doltIgnore[di.Pattern] = struct{}{}
			}

			for _, di := range database.Spec.DoltIgnorePatterns {
				if _, exists := doltIgnore[di]; !exists {
					return false
				}
			}

			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting System Branches to be created eventually")
		Eventually(func(g Gomega) bool {
			sqlClient, err := sql.NewClientWithDoltDB(ctx, &testDoltDB, refResolver)
			if err != nil {
				return false
			}
			defer func() {
				g.Expect(sqlClient.Close()).To(Succeed())
			}()

			err = sqlClient.UseDatabase(ctx, *database.Spec.Name)
			g.Expect(err).NotTo(HaveOccurred())

			branches, err := sqlClient.GetBranches(ctx)
			if err != nil {
				return false
			}

			branchSet := make(map[string]struct{})
			for _, b := range branches {
				branchSet[b] = struct{}{}
			}

			for _, branch := range database.Spec.SystemBranches {
				if _, exists := branchSet[branch]; !exists {
					return false
				}
			}

			return true
		}, testTimeout, testInterval).Should(BeTrue())
	})
})
