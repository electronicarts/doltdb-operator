// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package health

import (
	"context"
	"errors"
	"fmt"
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHealthyDoltDBReplica(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = doltv1alpha.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		doltdb        *doltv1alpha.DoltDB
		pods          []corev1.Pod
		expectedIndex *int
		expectedErr   error
	}{
		{
			name: "no current primary pod index",
			doltdb: &doltv1alpha.DoltDB{
				Status: doltv1alpha.DoltDBStatus{},
			},
			expectedErr: errors.New("'status.currentPrimaryPodIndex' must be set"),
		},
		{
			name: "no healthy replicas",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: 2,
				},
				Status: doltv1alpha.DoltDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dolt-0",
						Namespace: "default",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dolt-1",
						Namespace: "default",
					},
				},
			},
			expectedErr: ErrNoHealthyInstancesAvailable,
		},
		{
			name: "healthy replica found",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dolt",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: 2,
				},
				Status: doltv1alpha.DoltDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dolt-0",
						Namespace: "default",
						Labels: map[string]string{
							"app.kubernetes.io/name":     "dolt",
							"app.kubernetes.io/instance": "dolt",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dolt-1",
						Namespace: "default",
						Labels: map[string]string{
							"app.kubernetes.io/name":     "dolt",
							"app.kubernetes.io/instance": "dolt",
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			expectedIndex: ptr.To(1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.doltdb).Build()

			for _, p := range tt.pods {
				err := client.Create(context.Background(), &p)
				if err != nil {
					t.Errorf("failed to create pod: %v", err)
				}
			}

			index, err := HealthyDoltDBReplica(context.Background(), client, tt.doltdb)
			if tt.expectedErr != nil {
				if err == nil || err.Error() != tt.expectedErr.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if index == nil || *index != *tt.expectedIndex {
					t.Errorf("expected index %v, got %v", *tt.expectedIndex, index)
				}
			}
		})
	}
}

func TestIsStatefulSetHealthy(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = doltv1alpha.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		sts           *appsv1.StatefulSet
		endpoints     *corev1.Endpoints
		opts          []HealthOpt
		expected      bool
		expectedError error
	}{
		{
			name: "StatefulSet with ready replicas and no endpoint policy",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "default",
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To(int32(3)),
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas: 3,
				},
			},
			expected: true,
		},
		{
			name: "StatefulSet with not enough ready replicas",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "default",
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To(int32(3)),
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas: 2,
				},
			},
			expected: false,
		},
		{
			name: "StatefulSet with endpoint policy all and matching endpoints",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "default",
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To(int32(3)),
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas: 3,
				},
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "default",
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "1.1.1.1"},
							{IP: "1.1.1.2"},
							{IP: "1.1.1.3"},
						},
						Ports: []corev1.EndpointPort{
							{Port: 8080},
						},
					},
				},
			},
			opts: []HealthOpt{
				WithPort(8080),
				WithEndpointPolicy(EndpointPolicyAll),
			},
			expected: true,
		},
		{
			name: "StatefulSet with endpoint policy at least one and matching endpoints",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "default",
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To(int32(3)),
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas: 3,
				},
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "default",
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "1.1.1.1"},
						},
						Ports: []corev1.EndpointPort{
							{Port: 8080},
						},
					},
				},
			},
			opts: []HealthOpt{
				WithPort(8080),
				WithEndpointPolicy(EndpointPolicyAtLeastOne),
			},
			expected: true,
		},
		{
			name: "StatefulSet with endpoint policy all and non-matching endpoints",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "default",
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To(int32(3)),
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas: 3,
				},
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "default",
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "1.1.1.1"},
						},
						Ports: []corev1.EndpointPort{
							{Port: 8080},
						},
					},
				},
			},
			opts: []HealthOpt{
				WithPort(8080),
				WithEndpointPolicy(EndpointPolicyAll),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()

			if tt.sts != nil {
				if err := client.Create(context.Background(), tt.sts); err != nil {
					t.Errorf("failed to create StatefulSet: %v", err)
				}
			}

			if tt.endpoints != nil {
				if err := client.Create(context.Background(), tt.endpoints); err != nil {
					t.Errorf("failed to create Endpoints: %v", err)
				}
			}

			key := types.NamespacedName{Name: "test-sts", Namespace: "default"}

			healthy, err := IsStatefulSetHealthy(context.Background(), client, key, key, tt.opts...)
			if healthy != tt.expected {
				t.Errorf("expected healthy to be %v, got %v", tt.expected, healthy)
			}
			if tt.expectedError != nil {
				if err == nil || err.Error() != tt.expectedError.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}
func TestStandbyHostFQDNs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = doltv1alpha.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name        string
		doltdb      *doltv1alpha.DoltDB
		pods        []corev1.Pod
		wantCount   int
		wantErr     bool
		errContains string
	}{
		{
			name: "returns FQDNs for healthy standbys",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dolt",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: 3,
				},
				Status: doltv1alpha.DoltDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dolt-0",
						Namespace: "default",
						Labels: map[string]string{
							"app.kubernetes.io/name":     "dolt",
							"app.kubernetes.io/instance": "dolt",
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dolt-1",
						Namespace: "default",
						Labels: map[string]string{
							"app.kubernetes.io/name":     "dolt",
							"app.kubernetes.io/instance": "dolt",
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dolt-2",
						Namespace: "default",
						Labels: map[string]string{
							"app.kubernetes.io/name":     "dolt",
							"app.kubernetes.io/instance": "dolt",
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				},
			},
			wantCount: 2,
		},
		{
			name: "no healthy standbys returns error",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dolt",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: 1,
				},
				Status: doltv1alpha.DoltDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dolt-0",
						Namespace: "default",
						Labels: map[string]string{
							"app.kubernetes.io/name":     "dolt",
							"app.kubernetes.io/instance": "dolt",
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "no healthy standbys",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.doltdb).Build()

			for _, p := range tt.pods {
				if err := client.Create(context.Background(), &p); err != nil {
					t.Fatalf("failed to create pod: %v", err)
				}
			}

			hosts, err := StandbyHostFQDNs(context.Background(), client, tt.doltdb)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !containsStr(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(hosts) != tt.wantCount {
				t.Errorf("expected %d hosts, got %d: %v", tt.wantCount, len(hosts), hosts)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestIsServiceHealthy(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		serviceKey    types.NamespacedName
		endpoints     *corev1.Endpoints
		expected      bool
		expectedError error
	}{
		{
			name: "Service with healthy endpoints",
			serviceKey: types.NamespacedName{
				Name:      "test-service",
				Namespace: "default",
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "1.1.1.1"},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Service with no subsets",
			serviceKey: types.NamespacedName{
				Name:      "test-service",
				Namespace: "default",
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
			},
			expected:      false,
			expectedError: fmt.Errorf("'test-service/default' subsets not ready"),
		},
		{
			name: "Service with no addresses",
			serviceKey: types.NamespacedName{
				Name:      "test-service",
				Namespace: "default",
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{},
					},
				},
			},
			expected:      false,
			expectedError: fmt.Errorf("'test-service/default' addresses not ready"),
		},
		{
			name: "Service not found",
			serviceKey: types.NamespacedName{
				Name:      "non-existent-service",
				Namespace: "default",
			},
			expected:      false,
			expectedError: errors.New("endpoints \"non-existent-service\" not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()

			if tt.endpoints != nil {
				if err := client.Create(context.Background(), tt.endpoints); err != nil {
					t.Errorf("failed to create Endpoints: %v", err)
				}
			}

			healthy, err := IsServiceHealthy(context.Background(), client, tt.serviceKey)
			if healthy != tt.expected {
				t.Errorf("expected healthy to be %v, got %v", tt.expected, healthy)
			}
			if tt.expectedError != nil {
				if err == nil || err.Error() != tt.expectedError.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}
