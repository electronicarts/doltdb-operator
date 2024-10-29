package pvc

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestIsPersistentVolumeClaimResizing(t *testing.T) {
	tests := []struct {
		name     string
		pvc      *corev1.PersistentVolumeClaim
		expected bool
	}{
		{
			name: "PVC is resizing",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{
						{
							Type:   corev1.PersistentVolumeClaimResizing,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "PVC is not resizing",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{
						{
							Type:   corev1.PersistentVolumeClaimResizing,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "PVC has FileSystemResizePending condition",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{
						{
							Type:   corev1.PersistentVolumeClaimFileSystemResizePending,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "PVC has no conditions",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPersistentVolumeClaimResizing(tt.pvc)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsPersistentVolumeClaimFileSystemResizePending(t *testing.T) {
	tests := []struct {
		name     string
		pvc      *corev1.PersistentVolumeClaim
		expected bool
	}{
		{
			name: "PVC has FileSystemResizePending condition",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{
						{
							Type:   corev1.PersistentVolumeClaimFileSystemResizePending,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "PVC does not have FileSystemResizePending condition",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{
						{
							Type:   corev1.PersistentVolumeClaimFileSystemResizePending,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "PVC has no conditions",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPersistentVolumeClaimFileSystemResizePending(tt.pvc)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsResizing(t *testing.T) {
	tests := []struct {
		name     string
		pvc      *corev1.PersistentVolumeClaim
		expected bool
	}{
		{
			name: "PVC is resizing",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{
						{
							Type:   corev1.PersistentVolumeClaimResizing,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "PVC has FileSystemResizePending condition",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{
						{
							Type:   corev1.PersistentVolumeClaimFileSystemResizePending,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "PVC is not resizing",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{
						{
							Type:   corev1.PersistentVolumeClaimResizing,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "PVC has no conditions",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsResizing(tt.pvc)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
