# DoltDB Operator

[![License](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](LICENSE.txt)

A Kubernetes operator for running and managing [DoltDB](https://www.dolthub.com/blog/what-is-dolt/) clusters. Declaratively manage your DoltDB cluster using Kubernetes CRDs rather than imperative commands.

## Features

- Provision DoltDB clusters with a single Custom Resource
- Automated primary failover with replication-aware candidate selection
- Single-instance and multi-replica modes with seamless scaling transitions
- Scheduled backups via VolumeSnapshots
- Zero-downtime rolling updates for data-plane upgrades
- Declarative SQL resource management: users, grants, and logical databases
- Install via Helm charts

## Quick Start

### Prerequisites

- Kubernetes 1.28+
- Helm 3.x
- A StorageClass that supports dynamic provisioning
- (Recommended) A service mesh with mTLS (e.g., Istio) for network encryption

### Install with Helm

```sh
# Add the CRDs chart (install separately for lifecycle management)
helm install doltdb-operator-crds oci://ghcr.io/electronicarts/doltdb-operator-crds

# Install the operator
helm install doltdb-operator oci://ghcr.io/electronicarts/doltdb-operator
```

### Deploy a DoltDB Cluster

```yaml
apiVersion: k8s.dolthub.com/v1alpha
kind: DoltDB
metadata:
  name: my-dolt
spec:
  engineVersion: "1.57.2"
  image: dolthub/dolt
  replicas: 2
  storage:
    size: 10Gi
  server:
    listener:
      host: 0.0.0.0
      port: 3306
      maxConnections: 2048
    cluster:
      remotesAPI:
        port: 50051
  globalConfig:
    commitAuthor:
      name: "dolt operator"
      email: "dolt@operator.local"
  replication:
    enabled: true
    primary:
      podIndex: 0
      automaticFailover: true
```

```sh
kubectl apply -f my-dolt.yaml
```

### Connect to DoltDB

```sh
# Port-forward the primary service
kubectl port-forward svc/my-dolt-primary 3306:3306

# Connect with any MySQL client
mysql -h 127.0.0.1 -P 3306 -u root
```

## Custom Resources

| CRD | Description |
|-----|-------------|
| `DoltDB` | Main cluster resource — manages StatefulSet, Services, ConfigMaps, and replication |
| `Database` | Logical database within a DoltDB cluster |
| `User` | Database user management (references credentials from Kubernetes Secrets) |
| `Grant` | SQL permission grants for users |
| `Snapshot` | Point-in-time volume snapshots for backups |
| `Backup` | Dolt native backup management |
| `BackupSchedule` | Cron-based scheduled backups |

## Architecture

The operator follows the standard Kubernetes controller pattern built with [kubebuilder](https://book.kubebuilder.io/). The main `DoltDBReconciler` delegates to specialized sub-reconcilers:

```
DoltDBReconciler
  |-- RBACReconciler        (ServiceAccount, Role, RoleBinding)
  |-- ConfigMapReconciler   (DoltDB server configuration)
  |-- ServiceReconciler     (primary, reader, internal headless services)
  |-- StatefulSetReconciler (pods, rolling updates)
  |-- StorageReconciler     (PVC management)
  |-- StatusReconciler      (condition updates)
  +-- ReplicationReconciler (primary election, failover, switchover)
```

SQL resources (`Database`, `User`, `Grant`) connect directly to DoltDB instances to execute DDL statements, with reconciliation tied to the parent `DoltDB` cluster health.

## Helm Chart Configuration

See [`charts/doltdb-operator/values.yaml`](charts/doltdb-operator/values.yaml) for all available values.

Key options:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Operator container image | — |
| `image.tag` | Operator image tag | — |
| `replicas` | Number of operator replicas | `1` |
| `installCrds` | Install CRDs with the operator chart | `false` |
| `installClusterRole` | Create ClusterRole and ClusterRoleBinding | `true` |
| `leaderElection.id` | Leader election lock name | `k8s.dolthub.com` |

### CRDs Chart

CRDs are distributed as a separate Helm chart (`doltdb-operator-crds`) for independent lifecycle management. This allows upgrading CRDs separately from the operator.

## Network Encryption

The operator does not implement application-level TLS for DoltDB connections. It is designed to run in environments with network-level encryption:

- **Istio** with strict mTLS (recommended)
- **Linkerd**, **Cilium**, or other service mesh / CNI with encryption

Health probes use `--no-tls` because they connect via loopback (`127.0.0.1`) within the same pod.

See [SECURITY.md](SECURITY.md) for more details.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing, and contribution guidelines.

**Quick commands:**

```sh
make build          # Build the operator binary
make test           # Run unit tests
make lint-fix       # Lint and auto-fix
make cluster        # Create a kind cluster
make tiltci         # Run integration tests
```

## License

[BSD 3-Clause License](LICENSE.txt) - Copyright (c) 2025 Electronic Arts Inc.

## Contributing

Before you can contribute, EA must have a Contributor License Agreement (CLA) on file that has been signed by each contributor.
You can sign here: [Go to CLA](https://electronicarts.na1.echosign.com/public/esignWidget?wid=CBFCIBAA3AAABLblqZhByHRvZqmltGtliuExmuV-WNzlaJGPhbSRg2ufuPsM3P0QmILZjLpkGslg24-UJtek*)

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.
