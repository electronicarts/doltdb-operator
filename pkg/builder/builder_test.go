package builder

import (
	"reflect"
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func newTestBuilder() *Builder {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(doltv1alpha.AddToScheme(scheme))

	builder := NewBuilder(scheme)

	return builder
}

func assertObjectMeta(t *testing.T, objMeta *metav1.ObjectMeta, wantLabels, wantAnnotations map[string]string) {
	if objMeta == nil {
		t.Fatal("expecting object metadata to not be nil")
	}
	if !reflect.DeepEqual(wantLabels, objMeta.Labels) {
		t.Errorf("unexpected labels, want: %v  got: %v", wantLabels, objMeta.Labels)
	}
	if !reflect.DeepEqual(wantAnnotations, objMeta.Annotations) {
		t.Errorf("unexpected annotations, want: %v  got: %v", wantAnnotations, objMeta.Annotations)
	}
}

func assertMeta(t *testing.T, meta *doltv1alpha.DoltDB, wantLabels, wantAnnotations map[string]string) {
	if meta == nil {
		t.Fatal("expecting metadata to not be nil")
	}
	if !reflect.DeepEqual(wantLabels, meta.Labels) {
		t.Errorf("unexpected labels, want: %v  got: %v", wantLabels, meta.Labels)
	}
	if !reflect.DeepEqual(wantAnnotations, meta.Annotations) {
		t.Errorf("unexpected annotations, want: %v  got: %v", wantAnnotations, meta.Annotations)
	}
}
