package wait

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPollWithDoltCluster(t *testing.T) {
	logger := logr.Discard()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	scheme := runtime.NewScheme()
	_ = doltv1alpha.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	if err := client.Create(ctx, &doltv1alpha.DoltCluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}}); err != nil {
		t.Fatalf("failed to create DoltCluster: %v", err)
	}

	doltdbKey := types.NamespacedName{Name: "test", Namespace: "default"}

	t.Run("success", func(t *testing.T) {
		err := PollWithDoltCluster(ctx, doltdbKey, client, logger, func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("doltcluster not found", func(t *testing.T) {
		err := PollWithDoltCluster(ctx, types.NamespacedName{Namespace: "default", Name: "another-unknown-doltdb"}, client, logger, func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("doltcluster get error", func(t *testing.T) {
		err := PollWithDoltCluster(ctx, doltdbKey, client, logger, func(ctx context.Context) error {
			return errors.New("unexpected error")
		})
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})
}
