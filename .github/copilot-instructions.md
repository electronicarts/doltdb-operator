# Copilot Instructions for doltdb-operator

A Kubernetes operator for managing DoltDB clusters, built with [kubebuilder](https://book.kubebuilder.io/).
Module: `github.com/electronicarts/doltdb-operator`

## Commands

```bash
# Build
make build                    # Build manager binary
make docker-build IMG=<image> # Build container image

# Unit tests (excludes internal/controller/ which requires a cluster)
make test
make test TESTFLAGS="-run TestSpecificTest"  # Run a specific test

# Integration tests (requires kind cluster, runs in-cluster via Tilt)
make cluster cluster-ctx      # Create and switch to kind cluster
make tiltci                   # CI mode - runs tests and exits (preferred for AI)
make tiltdev                  # Interactive mode with Tilt UI

# Lint
make lint                     # Run golangci-lint + golines (check only)
make lint-fix                 # Auto-fix lint issues, format, and add copyright headers

# Code generation (MUST run after changing API types in api/v1alpha/)
make manifests                # Generate CRDs, RBAC, webhooks
make generate                 # Generate DeepCopy methods
```

## Architecture

### Custom Resources (CRDs)

Seven CRDs under API group `k8s.dolthub.com/v1alpha`:

| CRD | Description | Controller |
|-----|-------------|------------|
| **DoltDB** | Main cluster resource: StatefulSet, Services, ConfigMaps, replication | `internal/controller/doltdb_controller.go` |
| **Database** | Logical database within a DoltDB cluster | `internal/controller/database_controller.go` |
| **User** | Database user management (credentials via Secret refs) | `internal/controller/user_controller.go` |
| **Grant** | SQL privilege grants for users | `internal/controller/grant_controller.go` |
| **Snapshot** | Point-in-time VolumeSnapshots for backups | `internal/controller/snapshot_controller.go` |
| **Backup** | Dolt native backup management | `internal/controller/backup_controller.go` |
| **BackupSchedule** | Cron-based scheduled backups | `internal/controller/backupschedule_controller.go` |

### Package Structure

```
api/v1alpha/              CRD type definitions with kubebuilder markers
internal/controller/      Top-level reconciliation loops for each CRD
pkg/builder/              Kubernetes resource builders (StatefulSet, Service, ConfigMap, etc.)
pkg/controller/           Sub-reconcilers:
  ├── replication/          Primary election, failover, switchover, pod readiness
  ├── statefulset/          StatefulSet reconciliation and rolling updates
  ├── configmap/            DoltDB server config generation
  ├── service/              Service management (primary, reader, internal)
  ├── storage/              PVC management and resizing
  ├── status/               Status condition patching
  ├── rbac/                 ServiceAccount, Role, RoleBinding
  ├── database/             SQL resource reconciliation base (shared by Database, User, Grant)
  ├── backup/               Backup reconciliation
  ├── backupschedule/       BackupSchedule cron reconciliation
  └── volumesnapshot/       VolumeSnapshot creation
pkg/dolt/                 DoltDB-specific logic:
  ├── generate.go           Server config YAML generation
  ├── sql/                  SQL client for DoltDB (users, grants, databases, replication, backups)
  └── replication.go        Replication state types
pkg/refresolver/          Cross-resource reference resolution (DoltDB lookups, Secret refs)
pkg/conditions/           Status condition helpers (Ready, Complete, Updated, Backup)
pkg/health/               StatefulSet and pod health checks
pkg/builder/              Resource builders with labels, annotations, owner references
pkg/statefulset/          StatefulSet utility functions (pod indexing, naming)
pkg/pod/                  Pod utility functions
pkg/pvc/                  PVC utility functions
pkg/wait/                 Wait/retry utilities
pkg/metrics/              Prometheus metrics
pkg/patch/                Status patch helpers
pkg/watch/                Watch namespace configuration
charts/                   Helm charts:
  ├── doltdb-operator/      Operator deployment chart
  └── doltdb-operator-crds/ Standalone CRD chart
config/                   Kustomize manifests (CRDs, RBAC, manager, samples)
hack/                     Development scripts and test manifests
```

### Reconciliation Pattern

The main `DoltDBReconciler` delegates to specialized sub-reconcilers in phases:

```
DoltDBReconciler
  |-- RBACReconciler          (ServiceAccount, Role, RoleBinding)
  |-- ConfigMapReconciler     (DoltDB server config YAML per pod)
  |-- ServiceReconciler       (primary, reader, internal headless services)
  |-- StatefulSetReconciler   (pods, rolling updates with replication awareness)
  |-- StorageReconciler       (PVC management)
  |-- StatusReconciler        (condition updates, health aggregation)
  +-- ReplicationReconciler   (primary election, failover, switchover)
```

SQL resources (`Database`, `User`, `Grant`) use a shared `SqlReconciler` pattern in `pkg/controller/database/reconciler.go` that:
1. Waits for the parent DoltDB cluster to be healthy
2. Connects to DoltDB via SQL client (`pkg/dolt/sql/`)
3. Executes DDL statements
4. Updates status conditions

### Key Design Patterns

**Builder pattern** — Resource construction in `pkg/builder/`:
```go
b := builder.NewBuilder(scheme)
sts := b.BuildStatefulSet(doltdb, configMap)
svc := b.BuildService(doltdb, builder.ServiceTypePrimary)
```

**Reference resolution** — Cross-resource lookups in `pkg/refresolver/`:
```go
refResolver.DoltDB(ctx, obj)                    // Get DoltDB from annotation on child resource
refResolver.SecretKeyRef(ctx, namespace, ref)    // Resolve secret values for credentials
```

**Condition management** — Status conditions in `pkg/conditions/`:
```go
conditionReady.SetReady(ctx, doltdb)
conditionReady.SetFailed(ctx, doltdb, err)
conditions.SetPrimarySwitching(status, doltdb)
```

**Replication client set** — Per-pod SQL connections in `pkg/controller/replication/clientset.go`:
```go
clientSet := NewReplicationClientSet(doltdb, refResolver)
client, err := clientSet.ClientForIndex(ctx, podIndex)
dbState, err := client.GetDBState(ctx)
```

## Testing

### Unit Tests
- Run with `make test` (excludes `internal/controller/`)
- Standard Go tests in `pkg/` using `testify` assertions
- Table-driven tests are the preferred pattern
- Test files live alongside source (`*_test.go`)

### Integration Tests
- Run with `make tiltci` (CI) or `make tiltdev` (interactive)
- Located in `internal/controller/` using Ginkgo/Gomega
- Require a kind cluster with `DOLTDB_ENGINE_VERSION` env var set
- Test runner deployed as a Kubernetes Job via `Dockerfile.dev`
- Tests create real DoltDB clusters and verify reconciliation behavior

## Conventions

### Code Style
- **Go version**: 1.23+
- **Line length**: 140 characters max (enforced by `golines`)
- **Import grouping**: stdlib, then external, then internal (enforced by `goimports`)
- **Error handling**: Use `fmt.Errorf("context: %w", err)` for wrapping; `errors.Join` for multiple errors
- **Linter**: golangci-lint with config in `.golangci.yml`

### File Headers
All `.go` files MUST have the copyright header (auto-added by `make lint-fix`):
```go
// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.
```

### API Type Changes
When modifying types in `api/v1alpha/*_types.go`:
1. Add/update kubebuilder markers for validation, defaults, and descriptions
2. Run `make manifests generate` to regenerate CRDs and DeepCopy methods
3. Update Helm chart CRDs: copy from `config/crd/bases/` to `charts/doltdb-operator/generated/crd/` and `charts/doltdb-operator-crds/generated/crd/`
4. If RBAC changes: copy `config/rbac/role.yaml` to `charts/doltdb-operator/generated/rbac/role.yaml`

### Commit Messages
Use [conventional commits](https://www.conventionalcommits.org/):
```
feat: add backup retention policy
fix: correct replication failover timing
chore: update dependencies
docs: update CRD reference
refactor: extract SQL client interface
test: add switchover edge case tests
```

### Helm Charts
- **doltdb-operator**: Operator deployment (Deployment, RBAC, ServiceAccount)
- **doltdb-operator-crds**: Standalone CRD chart for independent lifecycle management
- Template helpers in `charts/doltdb-operator/templates/_helpers.tpl`
- CRDs and RBAC are generated — do not edit files in `charts/*/generated/` directly

### Network Encryption
The operator does NOT implement application-level TLS. It assumes network-level encryption via a service mesh (Istio mTLS, Linkerd, etc.). Health probes use `--no-tls` because they connect via loopback (`127.0.0.1`).

## Common Tasks

### Adding a New CRD
1. Define types in `api/v1alpha/<name>_types.go` with kubebuilder markers
2. Add keys helper in `api/v1alpha/<name>_keys.go`
3. Run `make manifests generate`
4. Create controller in `internal/controller/<name>_controller.go`
5. Register controller in `cmd/main.go`
6. Add RBAC markers to controller
7. Copy generated CRDs to Helm chart directories
8. Add tests

### Adding a Sub-reconciler to DoltDB
1. Create package in `pkg/controller/<name>/`
2. Implement reconciler with `Reconcile(ctx, doltdb) error` signature
3. Register in `DoltDBReconciler.reconcilePhases()` in `internal/controller/doltdb_controller.go`

### Debugging Replication
- Check `doltdb.Status.CurrentPrimaryPodIndex` for current primary
- Check conditions: `PrimarySwitching`, `PrimaryConfigured`, `Ready`
- Replication logic is in `pkg/controller/replication/`:
  - `controller.go` — main replication reconciler
  - `pod_readiness.go` — automatic failover on pod failure
  - `switchover.go` — graceful primary switchover
  - `clientset.go` — per-pod SQL connections for replication queries
