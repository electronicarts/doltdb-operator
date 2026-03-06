# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in the DoltDB Operator, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email **opensource@ea.com** with:

- A description of the vulnerability
- Steps to reproduce
- Potential impact
- Any suggested fix (if applicable)

We will acknowledge receipt within 48 hours and aim to provide a fix or mitigation plan within 7 business days.

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |

## Security Considerations

### Network Encryption

The DoltDB Operator does not implement application-level TLS for database connections. It is designed to run in environments with network-level encryption such as:

- **Istio** with strict mTLS (recommended)
- **Linkerd** with mTLS
- **Cilium** with WireGuard encryption
- Any service mesh or CNI providing transparent encryption

Health check probes use `--no-tls` because they connect via `127.0.0.1` (loopback within the same pod), which does not traverse the network.

If you run the operator without a service mesh, consider configuring network policies to restrict traffic between the operator and DoltDB pods.

### RBAC

The operator requires cluster-scoped permissions (ClusterRole) to manage CRDs and watch resources across namespaces. Review the generated RBAC in `charts/doltdb-operator/generated/rbac/role.yaml` to understand the permissions required.

### Secrets

Database credentials are stored in Kubernetes Secrets and referenced via `secretKeyRef` in CRD specs. Ensure your cluster has encryption at rest enabled for etcd to protect these values.

### CRD Input Validation

CRD fields are validated using kubebuilder markers (OpenAPI v3 schema validation). For additional defense-in-depth, consider deploying validating admission webhooks.
