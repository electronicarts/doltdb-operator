package builder

import (
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	ConfigmapKey                  = "dolt.yaml"
	ttlSecondsAfterFinished int32 = 300 // 5 minutes
)

// CronJobOpts holds the options for building a CronJob.
type CronJobOpts struct {
	Metadata      *metav1.ObjectMeta
	Key           types.NamespacedName
	ConfigMapName string
	Schedule      string
}

// BuildCronJob creates a cronJob based on the provided options and sets the owner reference.
// It returns the created cronJob or an error if the operation fails.
func (b *Builder) BuildCronJob(options CronJobOpts, doltdb *doltv1alpha.DoltDB, snapshot *doltv1alpha.Snapshot) (*batchv1.CronJob, error) {
	labels := NewLabelsBuilder().
		WithPartOf(doltdb.Name).
		WithManagedBy(snapshot.Name).
		Build()

	objMeta := NewMetadataBuilder(options.Key).
		WithMetadata(options.Metadata).
		WithLabels(labels).
		Build()
	//	Define the CronJobSpec
	cronJobSpec := batchv1.CronJobSpec{
		Schedule: options.Schedule, // Cron expression for every 5 minutes,
		JobTemplate: batchv1.JobTemplateSpec{
			Spec: batchv1.JobSpec{ // Use batchv1.JobSpec here
				TTLSecondsAfterFinished: ptr.To(ttlSecondsAfterFinished),
				Template: corev1.PodTemplateSpec{ // Define the PodTemplateSpec
					ObjectMeta: objMeta, // Set the metadata
					Spec: corev1.PodSpec{ // Define the PodSpec
						ServiceAccountName: doltdb.ServiceAccountKey().Name,
						ImagePullSecrets:   snapshot.Spec.ImagePullSecrets,
						Containers:         buildContainerSpec(options, *snapshot),
						Volumes:            buildVolume(options),
						RestartPolicy:      corev1.RestartPolicyOnFailure, // Use corev1.RestartPolicy
					},
				},
			},
		},
	}
	cronJob := &batchv1.CronJob{
		ObjectMeta: objMeta,
		Spec:       cronJobSpec,
	}
	if err := controllerutil.SetControllerReference(doltdb, cronJob, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to CronJob: %v", err)
	}
	return cronJob, nil
}

// buildVolume creates a volume for the cronJob.
func buildVolume(options CronJobOpts) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "yaml-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: options.ConfigMapName},
				},
			},
		},
	}
}

// buildContainerSpec creates a container for the cronJob.
func buildContainerSpec(options CronJobOpts, snapshot doltv1alpha.Snapshot) []corev1.Container {
	return []corev1.Container{ // Define the container inside the pod
		{
			Name:  options.Key.Name,
			Image: fmt.Sprintf("%s:%s", snapshot.Spec.Image, snapshot.Spec.Version),
			Command: []string{
				"/bin/sh",
				"-c",
				"export DATE=$(date +%s) && envsubst < /tmp/" + ConfigmapKey + " | kubectl apply -f -",
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "yaml-volume",
					MountPath: "/tmp",
					ReadOnly:  true,
				},
			},
		},
	}
}
