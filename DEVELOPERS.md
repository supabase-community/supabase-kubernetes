# Developer Guide

This guide is for anyone who wants to build, run, test, or contribute to the **Supabase Kubernetes Operator**. For end-user deployment instructions, see the [README](./README.md). For the Helm chart, see [`charts/supabase/README.md`](./charts/supabase/README.md).

## Prerequisites

- [Go](https://go.dev/dl/) 1.25+ (matching `go.mod`)
- [Docker](https://docs.docker.com/get-docker/) (or another container runtime compatible with the Makefile `CONTAINER_TOOL` variable)
- [kubectl](https://kubernetes.io/docs/tasks/tools/) configured to talk to your cluster
- [make](https://www.gnu.org/software/make/)
- [Kind](https://kind.sigs.k8s.io/) (recommended for local clusters and e2e tests)

## Project Layout

This is a standard [Kubebuilder](https://book.kubebuilder.io/) project:

- `api/v1alpha1/` — CRD Go types and generated DeepCopy code
- `internal/controller/` — reconciliation logic
- `internal/database/`, `internal/project/`, `internal/function/`, etc. — domain-specific reconciler packages
- `config/` — Kubernetes manifests (CRDs, RBAC, manager deployment)
- `test/e2e/` — end-to-end tests

For the full set of conventions, see [`AGENTS.md`](./AGENTS.md).

## Quick Start (Local Development)

The fastest way to get the Operator running locally is with a Kind cluster:

```bash
make kind-up
make generate
make manifests
make install
make run
kubectl apply -k config/samples
```

When you are done, tear down the cluster:

```bash
make kind-down
```

## Common Workflows

### Build the manager binary

```bash
make build
```

### Run the controller locally

This starts the controller against the cluster configured in `~/.kube/config`:

```bash
make run
```

### Run unit tests

```bash
make test
```

### Lint your changes

```bash
make lint
```

To auto-fix issues when possible:

```bash
make lint-fix
```

### Regenerate manifests and code

Run these after editing `*_types.go` or Kubebuilder markers:

```bash
make manifests
make generate
```

### Build and push the Operator image

```bash
make docker-build IMG=example.com/supabase-operator:v0.0.1
make docker-push IMG=example.com/supabase-operator:v0.0.1
```

### Deploy to a cluster

```bash
make deploy IMG=example.com/supabase-operator:v0.0.1
```

To remove the deployed resources:

```bash
make undeploy
```

### Run end-to-end tests

The e2e tests run against an isolated Kind cluster managed by the test suite:

```bash
make test-e2e
```

## Before Submitting a Pull Request

- Run `make lint-fix` and `make test`
- Run `make manifests` and `make generate` if you changed API types or markers
- Run `make test-e2e` if your change affects reconciliation or deployment behavior
- Keep the project structure and conventions described in [`AGENTS.md`](./AGENTS.md)
