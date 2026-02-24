// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package dolt

import (
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	"gopkg.in/yaml.v2"
)

func getIntValueOrDefault(value, defaultValue int32) int32 {
	if value == 0 {
		return defaultValue
	}
	return value
}

// GenerateConfigMapData generates the configuration data for the ConfigMap based on the number of replicas.
func GenerateConfigMapData(doltdb *doltv1alpha.DoltDB) (map[string]string, error) {
	remotesAPIPort := getIntValueOrDefault(doltdb.Spec.Server.Cluster.RemotesAPI.Port, RemotesAPIPort)

	data := make(map[string]string)
	for i := 0; i < int(doltdb.Spec.Replicas); i++ {
		config := Config{
			Behavior: Behavior{
				AutoGCBehavior: AutoGCBehavior{
					Enable: doltdb.Spec.Server.Behavior.AutoGCBehavior.Enable,
				},
			},
			LogLevel: doltdb.Spec.Server.LogLevel,
			Listener: Listener{
				Host:           doltdb.Spec.Server.Listener.Host,
				Port:           doltdb.Spec.Server.Listener.Port,
				MaxConnections: doltdb.Spec.Server.Listener.MaxConnections,
			},
		}

		if doltdb.Replication().Enabled {
			config.Cluster = &Cluster{
				StandbyRemotes: generateStandbyRemotes(i, doltdb, remotesAPIPort),
				BootstrapEpoch: 1,
				BootstrapRole:  getBootstrapRole(i),
				RemotesAPI: RemotesAPI{
					Port: remotesAPIPort,
				},
			}
		}

		if doltdb.Spec.Server.Metrics != nil && doltdb.Spec.Server.Metrics.Enabled {
			config.Metrics = Metrics{
				Labels: doltdb.Spec.Server.Metrics.Labels,
				Host:   doltdb.Spec.Server.Metrics.Host,
				Port:   doltdb.Spec.Server.Metrics.Port,
			}
		}

		yamlData, err := yaml.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("error marshaling DoltDB config to YAML: %v", err)
		}
		data[fmt.Sprintf("%s-%d.yaml", doltdb.Name, i)] = string(yamlData)
	}
	return data, nil
}

// generateStandbyRemotes generates the standby remotes section of the configuration.
func generateStandbyRemotes(current int, doltdb *doltv1alpha.DoltDB, remotesAPIPort int32) []StandbyRemote {
	var remotes []StandbyRemote
	for i := 0; i < int(doltdb.Spec.Replicas); i++ {
		if i != current {
			remotes = append(remotes, StandbyRemote{
				Name: fmt.Sprintf("%s-%d", doltdb.Name, i),
				RemoteURLTemplate: fmt.Sprintf(
					"http://%s:%d/{database}",
					statefulset.PodShortFQDNWithServiceAndNamespace(doltdb.ObjectMeta, i, doltdb.InternalServiceKey().Name),
					remotesAPIPort,
				),
			})
		}
	}
	return remotes
}

// getBootstrapRole returns the bootstrap role based on the index.
func getBootstrapRole(index int) string {
	if index == 0 {
		return PrimaryRoleValue.String()
	}
	return StandbyRoleValue.String()
}
