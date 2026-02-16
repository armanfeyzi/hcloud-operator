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
make install # Installs every CRD in config/crd/bases/ (servers, volumes, load balancers)
make run     # Runs the operator
```

If `kubectl apply` reports **no matches for kind "HCloudVolume"** or **"HCloudLoadBalancer"**, the cluster is missing newer CRDs. Run `make install` again (or `kubectl apply -f config/crd/bases/`), then re-apply your manifests.

In a separate terminal, try applying the demo stack (server, attached volume, load balancer):

```bash
kubectl apply -f config/samples/hcloud_stack_v1alpha1.yaml
kubectl get hcs,hcv,hclb
```

To try a **single-node k3s** workload cluster (private network + `userData` bootstrap), see [k3s on Hetzner with HKIC](k3s-on-hcloud.md) and `config/samples/k3s_single_node_private_net_v1alpha1.yaml`.

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
