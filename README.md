# dolt-operator

Run and operate DoltDB Cluster in a cloud native way. Declaratively manage your Dolt Cluster using Kubernetes CRDs rather than imperative commands.

## Features

* Easily provision DoltDB Cluster servers in Kubernetes
* Automated primary failover
* Scheduled backups via Snapshots
* Automated data-plane updates
* Declaratively manage SQL resources: users, grants, and logical databases
* Install via Helm or static manifests

## Custom Resources

| CRD | Description |
|-----|-------------|
| `DoltDB` | Main cluster resource managing StatefulSet, Services, ConfigMaps |
| `Database` | Logical database within a DoltDB cluster |
| `User` | Database user management |
| `Grant` | Permission grants for users |
| `Snapshot` | Volume snapshots for backups |
| `Backup` | On-demand backup to S3, DoltHub, or local storage |
| `BackupSchedule` | Cron-based scheduled backups |

## Roadmap

- **Backup-aware switchover** — prevent primary switchover while a backup is running to avoid unnecessary retries
- **Validating and mutating webhooks** — webhook certificate management for CRD admission control
- **Remote replication** — support for cross-cluster replication via Dolt remotes
- **Backup restore** — declarative restore from S3/DoltHub backups via a `BackupRestore` CRD

## Getting Started

### Prerequisites

- Go 1.23+
- Docker 17.03+
- kubectl 1.11.3+
- kind 0.30+
- Tilt 0.33+
- Access to a Kubernetes 1.11.3+ cluster

### Local Development

**Create kind cluster:**
```sh
make cluster cluster-ctx
```

**Run unit tests:**
```sh
make test
```

**Run integration tests (CI mode):**
```sh
make tiltci
```

**Run integration tests (interactive mode with Tilt UI):**
```sh
make tiltdev
```

**Lint and auto-fix:**
```sh
make lint-fix
```

## Deployment

### Build and Push Image

```sh
make docker-build docker-push IMG=<registry>/dolt-operator:tag
```

### Install CRDs and Deploy

```sh
make install
make deploy IMG=<registry>/dolt-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need cluster-admin privileges.

### Apply Sample Resources

```sh
kubectl apply -k config/samples/
```

## Uninstall

```sh
kubectl delete -k config/samples/
make uninstall
make undeploy
```

## Distribution

Build a consolidated installer YAML:

```sh
make build-installer IMG=<registry>/dolt-operator:tag
```

This generates `dist/install.yaml` which users can apply directly:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/dolt-operator/<tag>/dist/install.yaml
```

## Contributing

Before you can contribute, EA must have a Contributor License Agreement (CLA) on file that has been signed by each contributor.
You can sign here: [Go to CLA](https://electronicarts.na1.echosign.com/public/esignWidget?wid=CBFCIBAA3AAABLblqZhByHRvZqmltGtliuExmuV-WNzlaJGPhbSRg2ufuPsM3P0QmILZjLpkGslg24-UJtek*)
