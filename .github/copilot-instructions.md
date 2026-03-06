# Copilot Instructions for doltdb-operator

A Kubernetes operator for managing DoltDB clusters, built with kubebuilder.

## Build, Test, and Lint Commands

```bash
# Build
make build                    # Build manager binary
make docker-build IMG=<image> # Build container image

# Unit tests (excludes internal/controller/)
make test
make test TESTFLAGS="-run TestDoltServicePorts"  # Run specific test

# Integration tests (requires kind cluster, runs in-cluster via Tilt)
make cluster cluster-ctx      # Create and switch to kind cluster
make tiltci                   # CI mode - runs tests and exits (preferred for AI)
make tiltdev                  # Interactive mode - opens Tilt UI for humans

# Lint
make lint                     # Run golangci-lint + golines
make lint-fix                 # Auto-fix lint issues and add copyright headers

# Code generation (run after changing API types)
make manifests                # Generate CRDs, RBAC, webhooks
make generate                 # Generate DeepCopy methods
```

## Architecture

### Custom Resources (CRDs)

Five CRDs under `k8s.dolthub.com/v1alpha`:

- **DoltDB** - Main cluster resource managing StatefulSet, Services, ConfigMaps
- **Database** - Logical database within a DoltDB cluster
- **User** - Database user management
- **Grant** - Permission grants for users
- **Snapshot** - Volume snapshots for backups

### Package Structure

- `api/v1alpha/` - CRD type definitions with kubebuilder markers
- `internal/controller/` - Reconciliation loops for each CRD
- `pkg/controller/` - Sub-reconcilers (statefulset, service, replication, storage, etc.)
- `pkg/builder/` - Kubernetes resource builders (StatefulSet, Service, ConfigMap, etc.)
- `pkg/dolt/` - DoltDB-specific configuration generation
- `pkg/refresolver/` - Cross-resource reference resolution

### Reconciliation Pattern

The main `DoltDBReconciler` delegates to specialized sub-reconcilers in phases:

```
DoltDBReconciler
  ├── RBACReconciler      (ServiceAccount, Role, RoleBinding)
  ├── ConfigMapReconciler (DoltDB config)
  ├── ServiceReconciler   (primary, reader, internal services)
  ├── StatefulSetReconciler
  ├── StorageReconciler   (PVC management)
  ├── StatusReconciler
  └── ReplicationReconciler (primary election, failover)
```

### Testing

- **Unit tests** (`make test`): Standard Go tests in `pkg/` using testify
- **Integration tests** (`make tiltdev` or `make tiltci`): Ginkgo/Gomega tests in `internal/controller/` that run inside a kind cluster via Tilt. The test runner is deployed as a Job using `Dockerfile.dev`
- `DOLTDB_ENGINE_VERSION` env var is required for integration tests

## Key Conventions

### API Type Definitions

- All types in `api/v1alpha/` use kubebuilder markers for validation, defaults, and CRD generation
- Changes to `*_types.go` require running `make manifests generate`

### Builder Pattern

Resource construction uses builders in `pkg/builder/`:

```go
builder.NewBuilder(scheme)
builder.BuildStatefulSet(doltdb, configMap)
builder.BuildService(doltdb, serviceType)
```

### Reference Resolution

Cross-resource lookups use `pkg/refresolver.RefResolver`:

```go
refResolver.DoltDB(ctx, obj)              // Get DoltDB from annotation
refResolver.SecretKeyRef(ctx, namespace, ref) // Resolve secret values
```

### Condition Management

Status conditions use helpers in `pkg/conditions/`:

```go
conditionReady.SetReady(ctx, doltdb)
conditionReady.SetFailed(ctx, doltdb, err)
```

### File Headers

All `.go` files must have the EA copyright header (auto-added by `make lint-fix`):

```go
// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.
```

### Line Length

Maximum 140 characters enforced by golines. Run `make lint-fix` to auto-format.
