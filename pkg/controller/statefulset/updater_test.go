// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package statefulset

import (
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldReconcileUpdates(t *testing.T) {
	tests := []struct {
		name   string
		doltdb *doltv1alpha.DoltDB
		want   bool
	}{
		{
			name: "ReplicasFirstPrimaryLast strategy",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.ReplicasFirstPrimaryLastUpdateType,
				},
			},
			want: true,
		},
		{
			name: "empty strategy defaults to true",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: "",
				},
			},
			want: true,
		},
		{
			name: "RollingUpdate strategy",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.RollingUpdateUpdateType,
				},
			},
			want: false,
		},
		{
			name: "OnDelete strategy",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.OnDeleteUpdateType,
				},
			},
			want: false,
		},
		{
			name: "Never strategy",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.NeverUpdateType,
				},
			},
			want: false,
		},
		{
			name: "resizing storage skips updates",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.ReplicasFirstPrimaryLastUpdateType,
				},
				Status: doltv1alpha.DoltDBStatus{
					Conditions: []metav1.Condition{
						{
							Type:   doltv1alpha.ConditionTypeStorageResized,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "switching primary skips updates",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.ReplicasFirstPrimaryLastUpdateType,
				},
				Status: doltv1alpha.DoltDBStatus{
					Conditions: []metav1.Condition{
						{
							Type:   doltv1alpha.ConditionTypePrimarySwitched,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReconcileUpdates(tt.doltdb); got != tt.want {
				t.Errorf("shouldReconcileUpdates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetStalePodNames(t *testing.T) {
	updateRevision := "rev-2"

	tests := []struct {
		name       string
		podsByRole podRoleSet
		wantCount  int
		wantNames  []string
	}{
		{
			name: "all pods up to date",
			podsByRole: podRoleSet{
				primary: podWithRevision("primary-0", updateRevision),
				replicas: []corev1.Pod{
					podWithRevision("replica-1", updateRevision),
				},
			},
			wantCount: 0,
		},
		{
			name: "stale replica only",
			podsByRole: podRoleSet{
				primary: podWithRevision("primary-0", updateRevision),
				replicas: []corev1.Pod{
					podWithRevision("replica-1", "rev-1"),
				},
			},
			wantCount: 1,
			wantNames: []string{"replica-1"},
		},
		{
			name: "stale primary only",
			podsByRole: podRoleSet{
				primary: podWithRevision("primary-0", "rev-1"),
				replicas: []corev1.Pod{
					podWithRevision("replica-1", updateRevision),
				},
			},
			wantCount: 1,
			wantNames: []string{"primary-0"},
		},
		{
			name: "all pods stale",
			podsByRole: podRoleSet{
				primary: podWithRevision("primary-0", "rev-1"),
				replicas: []corev1.Pod{
					podWithRevision("replica-1", "rev-1"),
					podWithRevision("replica-2", "rev-1"),
				},
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names := tt.podsByRole.getStalePodNames(updateRevision)
			if len(names) != tt.wantCount {
				t.Errorf("expected %d stale pods, got %d: %v", tt.wantCount, len(names), names)
			}
			for _, wantName := range tt.wantNames {
				found := false
				for _, name := range names {
					if name == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected stale pod %q not found in %v", wantName, names)
				}
			}
		})
	}
}

func podWithRevision(name, revision string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"controller-revision-hash": revision,
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
}
