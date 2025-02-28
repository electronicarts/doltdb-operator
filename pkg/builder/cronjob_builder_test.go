package builder

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuildCronJob(t *testing.T) {
	builder := newTestBuilder()
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

	tests := []struct {
		name        string
		options     CronJobOpts
		doltdb      *doltv1alpha.DoltDB
		snapshot    *doltv1alpha.Snapshot
		wantCronJob *batchv1.CronJob
	}{
		{
			name: "doltdb-snapshot",
			options: CronJobOpts{
				Metadata: &metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":   "doltdb",
						"pvc.k8s.dolthub.com/role": "dolt-data",
					},
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
				},
				Key: types.NamespacedName{
					Name:      "doltdb-snapshot",
					Namespace: "test",
				},
				ConfigMapName: "dolt-config",
				Schedule:      "* * * * *",
			},
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
			},
			snapshot: &doltv1alpha.Snapshot{
				ObjectMeta: objMeta,
			},
			wantCronJob: &batchv1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb-snapshot",
					Namespace: "test",
					Labels: map[string]string{
						"app.kubernetes.io/name":   "",
						"pvc.k8s.dolthub.com/role": "dolt-data",
					},
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
				},
				Spec: batchv1.CronJobSpec{
					Schedule: "* * * * *",
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "doltdb-snapshot",
									Namespace: "test",
									Labels: map[string]string{
										"app.kubernetes.io/name":   "doltdb",
										"pvc.k8s.dolthub.com/role": "dolt-data",
									},
									Annotations: map[string]string{
										"sidecar.istio.io/inject": "false",
									},
								},
								Spec: corev1.PodSpec{
									ServiceAccountName: "doltdb-snapshot",
									Containers: []corev1.Container{
										{
											Name:  "doltdb-snapshot",
											Image: "bitnami/kubectl:latest",
											Command: []string{
												"/bin/sh",
												"-c",
												"export DATE=$(date +%s) && envsubst < /tmp/dolt.yaml | kubectl apply -f -",
											},
											VolumeMounts: []corev1.VolumeMount{
												{
													Name:      "yaml-volume",
													MountPath: "/tmp",
													ReadOnly:  true,
												},
											},
										},
									},
									Volumes: []corev1.Volume{
										{
											Name: "yaml-volume",
											VolumeSource: corev1.VolumeSource{
												ConfigMap: &corev1.ConfigMapVolumeSource{
													LocalObjectReference: corev1.LocalObjectReference{Name: "doltdb-snapshot"},
												},
											},
										},
									},
									RestartPolicy: corev1.RestartPolicyOnFailure,
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cronjob, err := builder.BuildCronJob(tt.options, tt.doltdb, tt.snapshot)
			if err != nil {
				t.Fatalf("unexpected error building Volumesnapshot: %v", err)
			}

			assert.Equal(
				t,
				cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command,
				tt.wantCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command,
				"The pod command should be equal",
			)
			if !reflect.DeepEqual(cronjob.Spec, tt.wantCronJob.Spec) {
				if !reflect.DeepEqual(cronjob.Spec.Schedule, tt.wantCronJob.Spec.Schedule) {
					t.Errorf("CronJob.Spec = %v, want %v", cronjob.Spec, tt.wantCronJob.Spec)
				}
			}
		})
	}
}
