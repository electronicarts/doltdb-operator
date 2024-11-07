package conditions

import (
	"errors"
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReady_PatcherRefResolver(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		obj      interface{}
		expected string
	}{
		{
			name:     "no error",
			err:      nil,
			obj:      &metav1.ObjectMeta{},
			expected: "",
		},
		{
			name:     "not found error",
			err:      apierrors.NewNotFound(schema.GroupResource{Group: "test", Resource: "resource"}, "name"),
			obj:      &metav1.ObjectMeta{},
			expected: "ObjectMeta not found",
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			obj:      &metav1.ObjectMeta{},
			expected: "Error getting ObjectMeta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready := NewReady()
			mockConditioner := &MockConditioner{}
			patcher := ready.PatcherRefResolver(tt.err, tt.obj)
			patcher(mockConditioner)

			if tt.expected == "" && len(mockConditioner.conditions) != 0 {
				t.Errorf("expected no conditions, got %v", mockConditioner.conditions)
			} else if tt.expected != "" {
				if len(mockConditioner.conditions) != 1 {
					t.Errorf("expected one condition, got %v", mockConditioner.conditions)
				} else if mockConditioner.conditions[0].Message != tt.expected {
					t.Errorf("expected message %q, got %q", tt.expected, mockConditioner.conditions[0].Message)
				}
			}
		})
	}
}

func TestComplete_PatcherRefResolver(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		obj      runtime.Object
		expected string
	}{
		{
			name:     "no error",
			err:      nil,
			obj:      &doltv1alpha.DoltDB{},
			expected: "",
		},
		{
			name:     "not found error",
			err:      apierrors.NewNotFound(doltv1alpha.GroupVersion.WithResource("doltdbs").GroupResource(), "name"),
			obj:      &doltv1alpha.DoltDB{},
			expected: "DoltDB not found",
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			obj:      &doltv1alpha.DoltDB{},
			expected: "Error getting DoltDB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewFakeClient()
			complete := NewComplete(client)
			mockConditioner := &MockConditioner{}
			patcher := complete.PatcherRefResolver(tt.err, tt.obj)
			patcher(mockConditioner)

			if tt.expected == "" && len(mockConditioner.conditions) != 0 {
				t.Errorf("expected no conditions, got %v", mockConditioner.conditions)
			} else if tt.expected != "" {
				if len(mockConditioner.conditions) != 1 {
					t.Errorf("expected one condition, got %v", mockConditioner.conditions)
				} else if mockConditioner.conditions[0].Message != tt.expected {
					t.Errorf("expected message %q, got %q", tt.expected, mockConditioner.conditions[0].Message)
				}
			}
		})
	}
}
