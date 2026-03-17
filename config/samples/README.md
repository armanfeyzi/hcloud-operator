# Sample Manifests

This directory is organized by scenario complexity:

- `simple/`: single-purpose components (one main CR type per sample)
- `complex/`: composed stacks and k3s-oriented scenarios

Each sample has its own subdirectory for easier discovery and future expansion.

## Prerequisites

- CRDs installed (`make install` or `kubectl apply -f config/crd/bases/`)
- Operator running with `HCLOUD_TOKEN`
- `kubectl` pointed to the management cluster

## Start Here (Recommended Order)

1. Learn each primitive in `simple/`
2. Move to composed stacks in `complex/`
3. Use k3s-oriented scenarios once primitive behavior is clear

### 1) Simple component samples

- Server:
  - `kubectl apply -f config/samples/simple/hcloud-server/hcloud_server_simple_v1alpha1.yaml`
- Volume:
  - `kubectl apply -f config/samples/simple/hcloud-volume/hcloud_volume_simple_v1alpha1.yaml`
- Network:
  - `kubectl apply -f config/samples/simple/hcloud-network/hcloud_network_v1alpha1.yaml`
- Firewall:
  - `kubectl apply -f config/samples/simple/hcloud-firewall/hcloud_firewall_v1alpha1.yaml`

Quick checks:

- `kubectl get hcs,hcv,hcn,hcfw`
- `kubectl get hcs -o yaml | rg "serverID|state|publicIPv4"`
- `kubectl get hcv -o yaml | rg "volumeID|attachedServerID|linuxDevice"`
- `kubectl get hcn -o yaml | rg "networkID|subnetZones"`

### 2) Complex composed samples

- Demo stack (network + server + volume + LB):
  - `kubectl apply -f config/samples/complex/hcloud-stack/hcloud_stack_v1alpha1.yaml`
- Connected services (2 servers + shared LB selector pattern):
  - `kubectl apply -f config/samples/complex/hcloud-connected-services/hcloud_connected_services_v1alpha1.yaml`
- Production-leaning blueprint:
  - `kubectl apply -f config/samples/complex/k3s-production-blueprint/k3s_production_blueprint_v1alpha1.yaml`

Quick checks:

- `kubectl get hcs,hcv,hcn,hclb,hcfw`
- `kubectl get hclb -o yaml | rg "loadBalancerID|attachedServerIDs|publicIPv4"`
- `kubectl get hcfw -o yaml | rg "firewallID|Ready"`

### 3) k3s-oriented scenarios

- Single node, public-only (smoke test):
  - `kubectl apply -f config/samples/complex/k3s-single-node-public-only/k3s_single_node_public_only_v1alpha1.yaml`
- Single node, private network (recommended baseline):
  - `kubectl apply -f config/samples/complex/k3s-single-node-private-net/k3s_single_node_private_net_v1alpha1.yaml`
- Multi-node join template (server + agents):
  - `kubectl apply -f config/samples/complex/k3s-multi-node-join/k3s_multi_node_join_v1alpha1.yaml`
  - then: `hack/configure-k3s-join-agents.sh k3s-join-server` (auto-discovers agents by labels)
- Day-2 baseline (CCM + CSI):
  - `config/samples/complex/k3s-day2-ccm/README.md`
  - `config/samples/complex/k3s-day2-csi/README.md`

See `docs/k3s-on-hcloud.md` for full flow and kubeconfig usage.

## Cleanup

Delete the specific sample file you applied, for example:

- `kubectl delete -f config/samples/complex/hcloud-stack/hcloud_stack_v1alpha1.yaml`

## Simple

- `simple/hcloud-server/`
- `simple/hcloud-volume/`
- `simple/hcloud-network/`
- `simple/hcloud-firewall/`

## Complex

- `complex/hcloud-stack/`
- `complex/hcloud-connected-services/`
- `complex/k3s-production-blueprint/`
- `complex/k3s-single-node-public-only/`
- `complex/k3s-single-node-private-net/`
- `complex/k3s-multi-node-join/`
- `complex/k3s-day2-ccm/`
- `complex/k3s-day2-csi/`
