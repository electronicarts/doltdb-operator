// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package builder

import (
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuildServiceAccount_Annotations(t *testing.T) {
	tests := []struct {
		name                      string
		serviceAccountAnnotations map[string]string
		wantAnnotations           map[string]string
	}{
		{
			name:                      "no service account annotations",
			serviceAccountAnnotations: nil,
			wantAnnotations:           map[string]string{},
		},
		{
			name: "with IRSA annotation",
			serviceAccountAnnotations: map[string]string{
				"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789:role/test",
			},
			wantAnnotations: map[string]string{
				"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789:role/test",
			},
		},
		{
			name: "with multiple annotations",
			serviceAccountAnnotations: map[string]string{
				"eks.amazonaws.com/role-arn":        "arn:aws:iam::123456789:role/test",
				"iam.gke.io/gcp-service-account":    "sa@project.iam.gserviceaccount.com",
				"azure.workload.identity/client-id": "00000000-0000-0000-0000-000000000000",
			},
			wantAnnotations: map[string]string{
				"eks.amazonaws.com/role-arn":        "arn:aws:iam::123456789:role/test",
				"iam.gke.io/gcp-service-account":    "sa@project.iam.gserviceaccount.com",
				"azure.workload.identity/client-id": "00000000-0000-0000-0000-000000000000",
			},
		},
	}

	builder := newTestBuilder()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doltdb := &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-doltdb",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					ServiceAccountAnnotations: tt.serviceAccountAnnotations,
				},
			}

			key := types.NamespacedName{Name: "test-sa", Namespace: "default"}
			sa, err := builder.BuildServiceAccount(key, doltdb)
			if !assert.NoError(t, err) || !assert.NotNil(t, sa) {
				return
			}

			for k, v := range tt.wantAnnotations {
				assert.Equal(t, v, sa.Annotations[k], "annotation %s mismatch", k)
			}
		})
	}
}
