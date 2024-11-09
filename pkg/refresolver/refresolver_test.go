package refresolver

import (
	"context"
	"errors"
	"fmt"
	"testing"

	doltv1alpha1 "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDoltDBFromAnnotation(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = doltv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		objMeta       metav1.ObjectMeta
		expectedDolt  *doltv1alpha1.DoltDB
		expectedError error
	}{
		{
			name: "annotation not found",
			objMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
				Namespace:   "default",
			},
			expectedError: ErrDoltClusterAnnotationNotFound,
		},
		{
			name: "dolt cluster not found",
			objMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					dolt.Annotation: "dolt-cluster",
				},
				Namespace: "default",
			},
			expectedError: apierrors.NewNotFound(doltv1alpha1.GroupVersion.WithResource("doltdbs").GroupResource(), "dolt-cluster"),
		},
		{
			name: "dolt cluster found",
			objMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					dolt.Annotation: "dolt-cluster",
				},
				Namespace: "default",
			},
			expectedDolt: &doltv1alpha1.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dolt-cluster",
					Namespace: "default",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			if tt.expectedDolt != nil {
				client.Create(context.Background(), tt.expectedDolt)
			}

			r := New(client)
			doltdb, err := r.DoltDBFromAnnotation(context.Background(), tt.objMeta)
			if tt.expectedError != nil {
				if err == nil || err.Error() != tt.expectedError.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if doltdb == nil || doltdb.Name != tt.expectedDolt.Name {
					t.Errorf("expected dolt cluster %v, got %v", tt.expectedDolt, doltdb)
				}
			}
		})
	}
}

func TestDoltDB(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = doltv1alpha1.AddToScheme(scheme)

	tests := []struct {
		name          string
		ref           *doltv1alpha1.DoltClusterRef
		namespace     string
		expectedDolt  *doltv1alpha1.DoltDB
		expectedError error
	}{
		{
			name: "dolt cluster not found",
			ref: &doltv1alpha1.DoltClusterRef{
				ObjectReference: doltv1alpha1.ObjectReference{
					Name: "dolt-cluster",
				},
			},
			namespace:     "default",
			expectedError: apierrors.NewNotFound(doltv1alpha1.GroupVersion.WithResource("doltdbs").GroupResource(), "dolt-cluster"),
		},
		{
			name: "dolt cluster found",
			ref: &doltv1alpha1.DoltClusterRef{
				ObjectReference: doltv1alpha1.ObjectReference{
					Name: "dolt-cluster",
				},
			},
			namespace: "default",
			expectedDolt: &doltv1alpha1.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dolt-cluster",
					Namespace: "default",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			if tt.expectedDolt != nil {
				client.Create(context.Background(), tt.expectedDolt)
			}

			r := New(client)
			doltdb, err := r.DoltDB(context.Background(), tt.ref, tt.namespace)
			if tt.expectedError != nil {
				if err == nil || err.Error() != tt.expectedError.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if doltdb == nil || doltdb.Name != tt.expectedDolt.Name {
					t.Errorf("expected dolt cluster %v, got %v", tt.expectedDolt, doltdb)
				}
			}
		})
	}
}

func TestSecretKeyRef(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		selector      doltv1alpha1.SecretKeySelector
		namespace     string
		secret        *corev1.Secret
		expectedValue string
		expectedError error
	}{
		{
			name: "secret not found",
			selector: doltv1alpha1.SecretKeySelector{
				LocalObjectReference: doltv1alpha1.LocalObjectReference{
					Name: "my-secret",
				},
				Key: "my-key",
			},
			namespace:     "default",
			expectedError: fmt.Errorf("error getting Secret: %v", apierrors.NewNotFound(corev1.Resource("secrets"), "my-secret")),
		},
		{
			name: "secret key not found",
			selector: doltv1alpha1.SecretKeySelector{
				LocalObjectReference: doltv1alpha1.LocalObjectReference{
					Name: "my-secret",
				},
				Key: "my-key",
			},
			namespace: "default",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{},
			},
			expectedError: errors.New("secret key \"my-key\" not found"),
		},
		{
			name: "secret key found",
			selector: doltv1alpha1.SecretKeySelector{
				LocalObjectReference: doltv1alpha1.LocalObjectReference{
					Name: "my-secret",
				},
				Key: "my-key",
			},
			namespace: "default",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"my-key": []byte("my-value"),
				},
			},
			expectedValue: "my-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			if tt.secret != nil {
				err := client.Create(context.Background(), tt.secret)
				if err != nil {
					t.Errorf("error not expected, got %v", err)
				}
			}

			r := New(client)
			value, err := r.SecretKeyRef(context.Background(), tt.selector, tt.namespace)
			if tt.expectedError != nil {
				if err == nil || err.Error() != tt.expectedError.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if value != tt.expectedValue {
					t.Errorf("expected value %v, got %v", tt.expectedValue, value)
				}
			}
		})
	}
}

func TestConfigMapKeyRef(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		selector      *doltv1alpha1.ConfigMapKeySelector
		namespace     string
		configMap     *corev1.ConfigMap
		expectedValue string
		expectedError error
	}{
		{
			name: "configmap not found",
			selector: &doltv1alpha1.ConfigMapKeySelector{
				LocalObjectReference: doltv1alpha1.LocalObjectReference{
					Name: "my-cm",
				},
				Key: "my-key",
			},
			namespace:     "default",
			expectedError: fmt.Errorf("error getting ConfigMap: %v", apierrors.NewNotFound(corev1.Resource("configmaps"), "my-cm")),
		},
		{
			name: "configmap key not found",
			selector: &doltv1alpha1.ConfigMapKeySelector{
				LocalObjectReference: doltv1alpha1.LocalObjectReference{
					Name: "my-configmap",
				},
				Key: "my-key",
			},
			namespace: "default",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-configmap",
					Namespace: "default",
				},
				Data: map[string]string{},
			},
			expectedError: errors.New("ConfigMap key \"my-key\" not found"),
		},
		{
			name: "configmap key found",
			selector: &doltv1alpha1.ConfigMapKeySelector{
				LocalObjectReference: doltv1alpha1.LocalObjectReference{
					Name: "my-configmap",
				},
				Key: "my-key",
			},
			namespace: "default",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-configmap",
					Namespace: "default",
				},
				Data: map[string]string{
					"my-key": "my-value",
				},
			},
			expectedValue: "my-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			if tt.configMap != nil {
				err := client.Create(context.Background(), tt.configMap)
				if err != nil {
					t.Errorf("error not expected, got %v", err)
				}
			}

			r := New(client)
			value, err := r.ConfigMapKeyRef(context.Background(), tt.selector, tt.namespace)
			if tt.expectedError != nil {
				if err == nil || err.Error() != tt.expectedError.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if value != tt.expectedValue {
					t.Errorf("expected value %v, got %v", tt.expectedValue, value)
				}
			}
		})
	}
}
