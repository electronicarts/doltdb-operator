package pod

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodUpdated(t *testing.T) {
	tests := []struct {
		name            string
		pod             *corev1.Pod
		updateRevision  string
		expectedUpdated bool
	}{
		{
			name: "pod updated",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"controller-revision-hash": "revision-123",
					},
				},
			},
			updateRevision:  "revision-123",
			expectedUpdated: true,
		},
		{
			name: "pod not updated",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"controller-revision-hash": "revision-123",
					},
				},
			},
			updateRevision:  "revision-456",
			expectedUpdated: false,
		},
		{
			name: "pod without revision label",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			updateRevision:  "revision-123",
			expectedUpdated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := PodUpdated(tt.pod, tt.updateRevision)
			if updated != tt.expectedUpdated {
				t.Errorf("expected %v, got %v", tt.expectedUpdated, updated)
			}
		})
	}
}
func TestPodReadyCondition(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected *corev1.PodCondition
	}{
		{
			name: "pod ready condition present",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: &corev1.PodCondition{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
		},
		{
			name: "pod ready condition absent",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			condition := PodReadyCondition(tt.pod)
			if condition == nil && tt.expected != nil {
				t.Errorf("expected %v, got nil", tt.expected)
			} else if condition != nil && tt.expected == nil {
				t.Errorf("expected nil, got %v", condition)
			} else if condition != nil && tt.expected != nil && *condition != *tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, condition)
			}
		})
	}
}

func TestPodReady(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod not ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod ready condition absent",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready := PodReady(tt.pod)
			if ready != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, ready)
			}
		})
	}
}
