package volumesnapshot

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/mock"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newTestBuilder() *builder.Builder {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(doltv1alpha.AddToScheme(scheme))

	builder := builder.NewBuilder(scheme)

	return builder
}
func Test_buildOrPatchConfigMap(t *testing.T) {
	builder1 := newTestBuilder()
	objMeta := metav1.ObjectMeta{
		Namespace: "test",
		Labels: map[string]string{
			"app.kubernetes.io/name":   "doltdb",
			"pvc.k8s.dolthub.com/role": "dolt-data",
		},
		Annotations: map[string]string{
			"sidecar.istio.io/inject": "false",
		},
	}
	data := make(map[string]string)
	data[builder.ConfigmapKey] = string("abs: 123")
	mockClient := new(MockKubernetesClient)
	mockedConfigMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc-cm",
			Namespace: "test",
			Labels: map[string]string{
				"app.kubernetes.io/name":   "",
				"pvc.k8s.dolthub.com/role": "dolt-data",
			},
			Annotations: map[string]string{},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "k8s.dolthub.com/v1alpha",
					Kind:       "DoltDB",
					Name:       "",
				},
			},
		},
		Data: data,
	}
	mockClient.On("Get", mock.MatchedBy(func(ctx context.Context) bool {
		// Match the specific context.TODO() passed in Get
		return ctx == context.TODO()
	}), client.ObjectKey{Namespace: "test", Name: "test-pvc-cm"}, mock.Anything).
		Return(errors.NewNotFound(v1.Resource("configmaps"), "test-pvc-cm"))

	mockClient.On("Create", mock.MatchedBy(func(ctx context.Context) bool {
		// Match the specific context.TODO() passed in Get
		return ctx == context.TODO()
	}), mock.AnythingOfType("*v1.ConfigMap")).Return(nil)

	type args struct {
		ctx      context.Context
		req      *ReconcileRequest
		yamlData []byte
		pvcName  string
		err      error
		r        *Reconciler
	}

	tests := []struct {
		name    string
		args    args
		want    *corev1.ConfigMap
		wantErr bool
	}{
		{
			name: "test-pvc-cm",
			args: args{
				ctx: context.TODO(),
				req: &ReconcileRequest{
					Metadata: &metav1.ObjectMeta{
						Labels: map[string]string{
							"app.kubernetes.io/name":   "doltdb",
							"pvc.k8s.dolthub.com/role": "dolt-data",
						},
						Annotations: map[string]string{},
					},
					Owner: &doltv1alpha.DoltDB{
						ObjectMeta: objMeta,
					},
					SubOwner: &doltv1alpha.Snapshot{
						ObjectMeta: objMeta,
					},
				},
				yamlData: []byte("abs: 123"),
				pvcName:  "test-pvc",
				err:      nil,
				r:        &Reconciler{Builder: builder1, Client: mockClient},
			},
			want: &mockedConfigMap,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.args.r.buildOrPatchConfigMap(tt.args.ctx, tt.args.req, tt.args.yamlData, tt.args.pvcName)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildOrPatchConfigMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.Data, tt.want.Data) {
				t.Errorf("buildOrPatchConfigMap() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// MockKubernetesClient is a mock implementation of the KubernetesClient interface
type MockKubernetesClient struct {
	mock.Mock
}

func (m *MockKubernetesClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	args := m.Called(ctx, key, obj)
	return args.Error(0)
}

func (m *MockKubernetesClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	// TODO implement me
	panic("implement me")
}

func (m *MockKubernetesClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockKubernetesClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockKubernetesClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockKubernetesClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := m.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

func (m *MockKubernetesClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	// TODO implement me
	panic("implement me")
}

func (m *MockKubernetesClient) Status() client.SubResourceWriter {
	// TODO implement me
	panic("implement me")
}

func (m *MockKubernetesClient) SubResource(subResource string) client.SubResourceClient {
	// TODO implement me
	panic("implement me")
}

func (m *MockKubernetesClient) Scheme() *runtime.Scheme {
	// TODO implement me
	panic("implement me")
}

func (m *MockKubernetesClient) RESTMapper() meta.RESTMapper {
	// TODO implement me
	panic("implement me")
}

func (m *MockKubernetesClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockKubernetesClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockKubernetesClient) GetConfigMap(namespace, name string) (*v1.ConfigMap, error) {
	args := m.Called(namespace, name)
	return args.Get(0).(*v1.ConfigMap), args.Error(1)
}
