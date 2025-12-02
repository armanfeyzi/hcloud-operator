# Local Development Guide

## Prerequisites

- Go 1.22+
- Docker
- a running Kubernetes cluster (e.g. `kind`, `minikube`)
- a Hetzner Cloud API token

## Setup Environment

1. Export your Hetzner Cloud token:
   ```bash
   export HCLOUD_TOKEN="your-api-token"
   ```

2. Point your `kubeconfig` to your local cluster:
   ```bash
   export KUBECONFIG=~/.kube/config
   ```

## Running the Operator Locally

To run the operator outside the cluster (as a local process) against your current `kubeconfig` context:

```bash
make install # Installs the CRDs into the cluster
make run     # Runs the operator
```

In a separate terminal, try applying a CRD:

```bash
kubectl apply -f config/samples/hcloudserver_v1alpha1.yaml
kubectl get hcloudservers
```

## Running Tests

Run the unit and envtest-based tests:

```bash
make test
```

## Generating Code

If you change the API types in `api/v1alpha1/hcloudserver_types.go`, you must regenerate the deepcopy methods and the CRD manifests:

```bash
make generate manifests
```
