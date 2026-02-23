// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package replication

import (
	"testing"

	"github.com/go-logr/logr"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestShouldReconcileSwitchover(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name   string
		doltdb *doltv1alpha.DoltDB
		want   bool
	}{
		{
			name: "different indices triggers switchover",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Replication: &doltv1alpha.Replication{
						Enabled: true,
						ReplicationSpec: doltv1alpha.ReplicationSpec{
							Primary: &doltv1alpha.PrimaryReplication{
								PodIndex: ptr.To(1),
							},
						},
					},
				},
				Status: doltv1alpha.DoltDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			want: true,
		},
		{
			name: "matching indices skips switchover",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Replication: &doltv1alpha.Replication{
						Enabled: true,
						ReplicationSpec: doltv1alpha.ReplicationSpec{
							Primary: &doltv1alpha.PrimaryReplication{
								PodIndex: ptr.To(0),
							},
						},
					},
				},
				Status: doltv1alpha.DoltDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			want: false,
		},
		{
			name: "nil CurrentPrimaryPodIndex skips switchover",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Replication: &doltv1alpha.Replication{
						Enabled: true,
						ReplicationSpec: doltv1alpha.ReplicationSpec{
							Primary: &doltv1alpha.PrimaryReplication{
								PodIndex: ptr.To(1),
							},
						},
					},
				},
				Status: doltv1alpha.DoltDBStatus{
					CurrentPrimaryPodIndex: nil,
				},
			},
			want: false,
		},
		{
			name: "resizing storage skips switchover",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Replication: &doltv1alpha.Replication{
						Enabled: true,
						ReplicationSpec: doltv1alpha.ReplicationSpec{
							Primary: &doltv1alpha.PrimaryReplication{
								PodIndex: ptr.To(1),
							},
						},
					},
				},
				Status: doltv1alpha.DoltDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
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
			name: "nil desired PodIndex defaults to 0",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Replication: &doltv1alpha.Replication{
						Enabled: true,
						ReplicationSpec: doltv1alpha.ReplicationSpec{
							Primary: &doltv1alpha.PrimaryReplication{},
						},
					},
				},
				Status: doltv1alpha.DoltDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReconcileSwitchover(logger, tt.doltdb); got != tt.want {
				t.Errorf("shouldReconcileSwitchover() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldReconcile(t *testing.T) {
	tests := []struct {
		name   string
		doltdb *doltv1alpha.DoltDB
		want   bool
	}{
		{
			name: "automatic failover enabled and configured",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Replication: &doltv1alpha.Replication{
						Enabled: true,
						ReplicationSpec: doltv1alpha.ReplicationSpec{
							Primary: &doltv1alpha.PrimaryReplication{
								AutomaticFailover: ptr.To(true),
							},
						},
					},
				},
				Status: doltv1alpha.DoltDBStatus{
					ReplicationStatus: doltv1alpha.ReplicationStatus{
						"pod-0": doltv1alpha.ReplicationStatePrimary,
						"pod-1": doltv1alpha.ReplicationStateStandby,
					},
				},
			},
			want: true,
		},
		{
			name: "replication disabled",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Replication: &doltv1alpha.Replication{
						Enabled: false,
						ReplicationSpec: doltv1alpha.ReplicationSpec{
							Primary: &doltv1alpha.PrimaryReplication{
								AutomaticFailover: ptr.To(true),
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "automatic failover disabled",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Replication: &doltv1alpha.Replication{
						Enabled: true,
						ReplicationSpec: doltv1alpha.ReplicationSpec{
							Primary: &doltv1alpha.PrimaryReplication{
								AutomaticFailover: ptr.To(false),
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "nil replication",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReconcile(tt.doltdb); got != tt.want {
				t.Errorf("shouldReconcile() = %v, want %v", got, tt.want)
			}
		})
	}
}
