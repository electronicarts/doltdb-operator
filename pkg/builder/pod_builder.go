package builder

import (
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func doltVolumes(doltdb *doltv1alpha.DoltDB) []corev1.Volume {
	configMapKeyRef := doltdb.DefaultConfigMapKeyRef().ToKubernetesType()

	return []corev1.Volume{
		{
			Name: DoltConfigVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: configMapKeyRef.LocalObjectReference,
				},
			},
		},
		{
			Name: "serviceaccount",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
								Path: "token",
							},
						},
						{
							ConfigMap: &corev1.ConfigMapProjection{
								Items: []corev1.KeyToPath{
									{
										Key:  "ca.crt",
										Path: "ca.crt",
									},
								},
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "kube-root-ca.crt",
								},
							},
						},
						{
							DownwardAPI: &corev1.DownwardAPIProjection{
								Items: []corev1.DownwardAPIVolumeFile{
									{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
										Path: "namespace",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func doltPodTemplate(metadata metav1.ObjectMeta, doltdb *doltv1alpha.DoltDB) corev1.PodTemplateSpec {
	labels := NewLabelsBuilder().
		WithDoltSelectorLabels(doltdb).
		WithVersion(doltdb.Spec.EngineVersion).
		Build()

	objMeta := NewMetadataBuilder(client.ObjectKeyFromObject(doltdb)).
		WithMetadata(&metadata).
		WithLabels(labels).
		Build()

	return corev1.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			ServiceAccountName:           ptr.Deref(doltdb.Spec.ServiceAccountName, doltdb.Name),
			Containers:                   doltContainers(doltdb),
			ImagePullSecrets:             doltdb.Spec.ImagePullSecrets,
			Volumes:                      doltVolumes(doltdb),
			SecurityContext:              &doltdb.Spec.PodSecurityContext,
			Affinity:                     doltdb.Spec.Affinity,
			NodeSelector:                 doltdb.Spec.NodeSelector,
			Tolerations:                  doltdb.Spec.Tolerations,
			InitContainers:               doltInitContainers(doltdb),
		},
	}
}
