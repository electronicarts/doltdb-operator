package builder

import (
	"fmt"
	"strings"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

const (
	DoltContainerName     = "dolt"
	DoltInitContainerName = "dolt-init"

	DoltMySQLPortName    = "dolt"
	DoltMetricsPortName  = "dolt-metrics"
	DoltProfilerPortName = "dolt-profiler"

	DoltDataVolume   = "dolt-data"
	DoltConfigVolume = "dolt-config"

	DoltDataMountPath   = "/db"
	DoltConfigMountPath = "/etc/doltdb"

	DefaultLivenessProbeInitialDelaySeconds  = 60
	DefaultReadinessProbeInitialDelaySeconds = 40
	DefaultProbePeriodSeconds                = 10
	DefaultProbeTimeoutSeconds               = 3
)

func doltVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      DoltConfigVolume,
			MountPath: DoltConfigMountPath,
		},
		{
			Name:      DoltDataVolume,
			MountPath: DoltDataMountPath,
		},
	}
}

func doltContainerCommand(doltdb *doltv1alpha.DoltDB) []string {
	cmd := []string{
		"tini",
		"--",
		"/usr/local/bin/dolt",
	}

	if doltdb.Spec.Server.Profiler.EnablePProf {
		cmd = append(cmd, "--prof", "mem", "--pprof-server")
	}

	cmd = append(cmd, "sql-server", "--config", "config.yaml")

	return cmd
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

func doltContainerPorts(doltdb *doltv1alpha.DoltDB) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{
			ContainerPort: doltdb.Spec.Server.Listener.Port,
			Name:          DoltMySQLPortName,
		},
	}

	if doltdb.Spec.Server.Metrics != nil && doltdb.Spec.Server.Metrics.Enabled {
		ports = append(ports, corev1.ContainerPort{
			ContainerPort: doltdb.Spec.Server.Metrics.Port,
			Name:          DoltMetricsPortName,
		})
	}

	if doltdb.Spec.Server.Profiler.EnablePProf {
		ports = append(ports, corev1.ContainerPort{
			ContainerPort: 6060,
			Name:          DoltProfilerPortName,
		})
	}

	return ports
}

func doltContainers(doltdb *doltv1alpha.DoltDB) []corev1.Container {
	containers := []corev1.Container{
		{
			Name:            DoltContainerName,
			Image:           fmt.Sprintf("%s:%s", doltdb.Spec.Image, doltdb.Spec.EngineVersion),
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         doltContainerCommand(doltdb),
			WorkingDir:      DoltDataMountPath,
			Env:             doltEnv(doltdb),
			Resources:       doltResourceRequirements(doltdb),
			Ports:           doltContainerPorts(doltdb),
			VolumeMounts:    doltVolumeMounts(),
			ReadinessProbe:  doltReadinessProbe(doltdb.Spec.Probes.ReadinessProbe, doltdb.Spec.Server.Listener),
			LivenessProbe:   doltLivenessProbe(doltdb.Spec.Probes.LivenessProbe, doltdb.Spec.Server.Listener),
		},
	}

	return containers
}

func doltInitContainers(doltdb *doltv1alpha.DoltDB) []corev1.Container {
	var commands []string

	doltdb.Spec.GlobalConfig.ApplyDefaults()

	commands = append(commands,
		fmt.Sprintf("dolt config --global --add user.name \"%s\"", doltdb.Spec.GlobalConfig.CommitAuthor.Name),
		fmt.Sprintf("dolt config --global --add user.email \"%s\"", doltdb.Spec.GlobalConfig.CommitAuthor.Email),
		fmt.Sprintf("dolt config --global --add metrics.disabled %t", ptr.Deref(doltdb.Spec.GlobalConfig.DisableClientUsageMetricsCollection, false)),
		"cp /etc/doltdb/${POD_NAME}.yaml config.yaml", `
if [ -n "$DOLT_PASSWORD" -a ! -f .doltcfg/privileges.db ]; then
	dolt sql -q "create user '$DOLT_USERNAME' identified by '$DOLT_PASSWORD'; grant all privileges on *.* to '$DOLT_USERNAME' with grant option;"
fi
`)

	command := []string{
		"/bin/sh",
		"-c",
		strings.Join(commands, "\n"),
	}

	containers := []corev1.Container{
		{
			Name:            DoltInitContainerName,
			Image:           fmt.Sprintf("%s:%s", doltdb.Spec.Image, doltdb.Spec.EngineVersion),
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         command,
			WorkingDir:      DoltDataMountPath,
			Env:             doltEnv(doltdb),
			VolumeMounts:    doltVolumeMounts(),
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

func doltReadinessProbe(probe *corev1.Probe, listener doltv1alpha.Listener) *corev1.Probe {
	if probe == nil {
		probe = &corev1.Probe{
			InitialDelaySeconds: DefaultReadinessProbeInitialDelaySeconds,
			PeriodSeconds:       DefaultProbePeriodSeconds,
		}
	}
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"/bin/sh",
					"-c",
					fmt.Sprintf(
						`dolt --host 127.0.0.1 -u "$DOLT_USERNAME" -p "$DOLT_PASSWORD" --port %d --no-tls sql -q 'select current_timestamp();'`,
						listener.Port,
					),
				},
			},
		},
		InitialDelaySeconds:           probe.InitialDelaySeconds,
		PeriodSeconds:                 probe.PeriodSeconds,
		TimeoutSeconds:                probe.TimeoutSeconds,
		SuccessThreshold:              probe.SuccessThreshold,
		FailureThreshold:              probe.FailureThreshold,
		TerminationGracePeriodSeconds: probe.TerminationGracePeriodSeconds,
	}
}

func doltLivenessProbe(probe *corev1.Probe, listener doltv1alpha.Listener) *corev1.Probe {
	if probe == nil {
		probe = &corev1.Probe{
			InitialDelaySeconds: DefaultLivenessProbeInitialDelaySeconds,
			PeriodSeconds:       DefaultProbePeriodSeconds,
			TimeoutSeconds:      DefaultProbeTimeoutSeconds,
		}
	}

	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"/bin/sh",
					"-c",
					fmt.Sprintf(
						`dolt --host 127.0.0.1 -u "$DOLT_USERNAME" -p "$DOLT_PASSWORD" --port %d --no-tls sql -q 'select current_timestamp();'`,
						listener.Port,
					),
				},
			},
		},
		InitialDelaySeconds:           probe.InitialDelaySeconds,
		PeriodSeconds:                 probe.PeriodSeconds,
		TimeoutSeconds:                probe.TimeoutSeconds,
		SuccessThreshold:              probe.SuccessThreshold,
		FailureThreshold:              probe.FailureThreshold,
		TerminationGracePeriodSeconds: probe.TerminationGracePeriodSeconds,
	}
}
