package dolt

import (
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	"gopkg.in/yaml.v2"
)

// GenerateConfigMapData generates the configuration data for the ConfigMap based on the number of replicas.
func GenerateConfigMapData(doltdb *doltv1alpha.DoltDB) (map[string]string, error) {
	var maxConnections int32

	if doltdb.Spec.MaxConnections != nil {
		maxConnections = *doltdb.Spec.MaxConnections
	} else {
		maxConnections = MaxConnections
	}

	data := make(map[string]string)
	for i := 0; i < int(doltdb.Spec.Replicas); i++ {
		config := Config{
			LogLevel: LogLevel,
			Cluster: Cluster{
				StandbyRemotes: generateStandbyRemotes(i, doltdb),
				BootstrapEpoch: 1,
				BootstrapRole:  getBootstrapRole(i),
				RemotesAPI: RemotesAPI{
					Port: RemotesAPIPort,
				},
			},
			Listener: Listener{
				Host:           "0.0.0.0",
				Port:           DatabasePort,
				MaxConnections: maxConnections,
			},
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
func generateStandbyRemotes(current int, doltdb *doltv1alpha.DoltDB) []StandbyRemote {
	var remotes []StandbyRemote
	for i := 0; i < int(doltdb.Spec.Replicas); i++ {
		if i != current {
			remotes = append(remotes, StandbyRemote{
				Name:              fmt.Sprintf("%s-%d", doltdb.Name, i),
				RemoteURLTemplate: fmt.Sprintf("http://%s:%d/{database}", statefulset.PodShortFQDNWithServiceAndNamespace(doltdb.ObjectMeta, i, doltdb.InternalServiceKey().Name), RemotesAPIPort),
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
