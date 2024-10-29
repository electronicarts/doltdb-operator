package builder

import (
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodDisruptionBudgetMeta(t *testing.T) {
	builder := newTestBuilder()
	tests := []struct {
		name     string
		opts     PodDisruptionBudgetOpts
		wantMeta *doltv1alpha.DoltCluster
	}{
		{
			name: "no meta",
			opts: PodDisruptionBudgetOpts{},
			wantMeta: &doltv1alpha.DoltCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
			},
		},
		{
			name: "meta",
			opts: PodDisruptionBudgetOpts{
				Metadata: &metav1.ObjectMeta{
					Labels: map[string]string{
						"doltdb.org.com": "dolt",
					},
					Annotations: map[string]string{
						"doltdb.org.com": "dolt",
					},
				},
			},
			wantMeta: &doltv1alpha.DoltCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"doltdb.org.com": "dolt",
					},
					Annotations: map[string]string{
						"doltdb.org.com": "dolt",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMap, err := builder.BuildPodDisruptionBudget(tt.opts, &doltv1alpha.DoltCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "doltdb",
				},
			})
			if err != nil {
				t.Fatalf("unexpected error building PDB: %v", err)
			}
			assertObjectMeta(t, &configMap.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}
