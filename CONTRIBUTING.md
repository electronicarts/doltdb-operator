# Contributing to doltdb-operator

Thank you for your interest in contributing to the DoltDB Operator! This document provides guidelines and instructions for contributing.

## Contributor License Agreement (CLA)

Before you can contribute, Electronic Arts must have a Contributor License Agreement (CLA) on file that has been signed by each contributor.
You can sign here: [Go to CLA](https://electronicarts.na1.echosign.com/public/esignWidget?wid=CBFCIBAA3AAABLblqZhByHRvZqmltGtliuExmuV-WNzlaJGPhbSRg2ufuPsM3P0QmILZjLpkGslg24-UJtek*)

## Development Setup

### Prerequisites

- Go 1.23+
- Docker 17.03+
- kubectl v1.28+
- [kind](https://kind.sigs.k8s.io/) v0.30+
- [Tilt](https://tilt.dev/) v0.33+ (for integration tests)

### Getting Started

1. Fork and clone the repository
2. Install dependencies:
   ```sh
   make build
   ```
3. Run unit tests:
   ```sh
   make test
   ```
4. Create a kind cluster for integration testing:
   ```sh
   make cluster cluster-ctx
   ```

### Running Tests

**Unit tests** (no cluster required):
```sh
make test
make test TESTFLAGS="-run TestSpecificTest"
```

**Integration tests** (requires kind cluster):
```sh
make tiltci    # CI mode - runs tests and exits
make tiltdev   # Interactive mode with Tilt UI
```

**Linting:**
```sh
make lint       # Check for issues
make lint-fix   # Auto-fix issues and add copyright headers
```

### Code Generation

After changing API types in `api/v1alpha/`:
```sh
make manifests   # Generate CRDs, RBAC, webhooks
make generate    # Generate DeepCopy methods
```

## How to Contribute

### Reporting Bugs

Open a GitHub issue with:
- A clear title and description
- Steps to reproduce
- Expected vs actual behavior
- Operator version, Kubernetes version, and DoltDB engine version

### Suggesting Features

Open a GitHub issue with:
- A clear description of the feature
- The use case it addresses
- Any proposed implementation approach

### Submitting Changes

1. Create a feature branch from `main`:
   ```sh
   git checkout -b feat/my-feature main
   ```
2. Make your changes following the conventions below
3. Add or update tests as needed
4. Run the full validation suite:
   ```sh
   make lint-fix
   make test
   ```
5. Commit using [conventional commits](https://www.conventionalcommits.org/):
   ```
   feat: add backup retention policy
   fix: correct replication failover timing
   docs: update CRD reference documentation
   ```
6. Push and open a pull request against `main`

## Conventions

### Code Style

- Follow idiomatic Go patterns
- Maximum line length: 140 characters (enforced by `golines`)
- All `.go` files must include the copyright header (auto-added by `make lint-fix`):
  ```go
  // Copyright (c) 2025 Electronic Arts Inc. All rights reserved.
  ```

### API Types

- All types in `api/v1alpha/` use kubebuilder markers for validation, defaults, and CRD generation
- Changes to `*_types.go` require running `make manifests generate`

### Commit Messages

Use [conventional commits](https://www.conventionalcommits.org/):
- `feat:` for new features
- `fix:` for bug fixes
- `chore:` for maintenance tasks
- `docs:` for documentation changes
- `refactor:` for code refactoring
- `test:` for test changes

### Pull Requests

- Keep PRs focused on a single concern
- Include a description of what changed and why
- Add test coverage for behavioral changes
- Ensure CI passes before requesting review

## Project Structure

```
api/v1alpha/         CRD type definitions with kubebuilder markers
internal/controller/ Reconciliation loops for each CRD
pkg/builder/         Kubernetes resource builders (StatefulSet, Service, etc.)
pkg/controller/      Sub-reconcilers (replication, statefulset, storage, etc.)
pkg/dolt/            DoltDB-specific configuration generation
pkg/dolt/sql/        SQL client for managing DoltDB instances
pkg/refresolver/     Cross-resource reference resolution
charts/              Helm charts for deployment
config/              Kustomize manifests (CRDs, RBAC, manager)
hack/                Development scripts and test manifests
```

## License

By contributing, you agree that your contributions will be licensed under the [BSD 3-Clause License](LICENSE.txt).
