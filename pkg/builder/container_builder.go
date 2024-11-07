package builder

import (
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

const (
	DoltContainerName     = "dolt"
	DoltInitContainerName = "dolt-init"

	DoltMySQLPortName = "dolt"
	DoltMySQLPort     = 3306
	DoltRemoteAPIPort = 50051

	DoltDataVolume   = "dolt-data"
	DoltConfigVolume = "dolt-config"

	DoltDataMountPath   = "/db"
	DoltConfigMountPath = "/etc/doltdb"
)

func doltVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      DoltDataVolume,
			MountPath: DoltDataMountPath,
		},
		{
			Name:      DoltConfigVolume,
			MountPath: DoltConfigMountPath,
		},
	}
}

func doltContainerCommand() []string {
	return []string{
		"/usr/local/bin/dolt",
		"sql-server",
		"--config",
		"config.yaml",
		"--data-dir",
		".",
	}
}

func doltEnv(doltdb *doltv1alpha.DoltDB) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "DOLT_ROOT_PATH",
			Value: DoltDataMountPath,
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "DOLT_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: ptr.To(doltdb.RootUserSecretKeyRef().ToKubernetesType()),
			},
		},
		{
			Name: "DOLT_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: ptr.To(doltdb.RootPasswordSecretKeyRef().ToKubernetesType()),
			},
		},
	}
}

func doltContainers(doltdb *doltv1alpha.DoltDB) []corev1.Container {
	containers := []corev1.Container{
		{
			Name:            DoltContainerName,
			Image:           fmt.Sprintf("%s:%s", doltdb.Spec.Image, doltdb.Spec.EngineVersion),
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         doltContainerCommand(),
			WorkingDir:      DoltDataMountPath,
			Env:             doltEnv(doltdb),
			Resources:       doltResourceRequirements(doltdb),
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: DoltMySQLPort,
					Name:          DoltContainerName,
				},
			},
			VolumeMounts: doltVolumeMounts(),
		},
	}

	return containers
}

func doltInitContainers(doltdb *doltv1alpha.DoltDB) []corev1.Container {
	containers := []corev1.Container{
		{
			Name:            DoltInitContainerName,
			Image:           fmt.Sprintf("%s:%s", doltdb.Spec.Image, doltdb.Spec.EngineVersion),
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command: []string{
				"/bin/sh",
				"-c",
				`
				dolt config --global --add user.name "dolt kubernetes deployment"
				dolt config --global --add user.email "dolt@kubernetes.deployment"
				cp /etc/doltdb/${POD_NAME}.yaml config.yaml
				if [ -n "$DOLT_PASSWORD" -a ! -f .doltcfg/privileges.db ]; then
					dolt sql -q "create user '$DOLT_USERNAME' identified by '$DOLT_PASSWORD'; grant all privileges on *.* to '$DOLT_USERNAME' with grant option;"
				fi`,
			},
			WorkingDir:   DoltDataMountPath,
			Env:          doltEnv(doltdb),
			VolumeMounts: doltVolumeMounts(),
		},
	}

	return containers
}

func doltResourceRequirements(doltdb *doltv1alpha.DoltDB) corev1.ResourceRequirements {
	if doltdb.Spec.Resources != nil {
		return *doltdb.Spec.Resources
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("4000m"),
			"memory": resource.MustParse("2Gi"),
		},
		Limits: corev1.ResourceList{
			"memory": resource.MustParse("8Gi"),
		},
	}
}
