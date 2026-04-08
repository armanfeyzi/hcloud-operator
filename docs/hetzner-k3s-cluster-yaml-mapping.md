# Mapping `hetzner-k3s` `cluster.yaml` to HKIC

This document maps a representative `hetzner-k3s` `cluster.yaml` into HKIC resources.

Purpose:

- show how to express the same intent with HKIC CRDs
- highlight fields that are already covered
- call out current gaps and practical workarounds

HKIC remains composable: there is no mandatory all-in-one cluster CRD. You combine `HCloudNetwork`, `HCloudServer`, `HCloudFirewall`, `HCloudLoadBalancer`, and Day-2 manifests.

## Representative `cluster.yaml` (conceptual)

This is a condensed example of the kind of inputs users commonly set in `hetzner-k3s`:

```yaml
cluster_name: demo
k3s_version: v1.30.2+k3s1
public_ssh_key_path: ~/.ssh/id_ed25519.pub
private_network_subnet: 10.220.0.0/16
location: fsn1
network_zone: eu-central

masters_pool:
  instance_type: cx23
  instance_count: 1

worker_node_pools:
  - name: general
    instance_type: cx23
    instance_count: 2

firewall:
  enabled: true
  api_source_ranges:
    - 203.0.113.0/24

load_balancer:
  enabled: true
  type: lb11

cni: flannel
disable_traefik: true

cloud_controller_manager:
  enabled: true
csi_driver:
  enabled: true
```

## HKIC resource composition for the same intent

1. `HCloudNetwork` for `private_network_subnet` and `network_zone`.
2. `HCloudServer` for each master/worker node (type, location, image, SSH keys, cloud-init bootstrap).
3. `HCloudFirewall` for API/SSH restrictions and server attachment.
4. `HCloudLoadBalancer` for Hetzner LB lifecycle and server target selection.
5. Day-2 manifests for CCM, CSI, optional Cluster Autoscaler, and System Upgrade Controller (`docs/k3s-day2.md`).

## Field-by-field mapping

| `hetzner-k3s` concept | HKIC mapping | Status | Notes |
|---|---|---|---|
| `cluster_name` | Labels on all HKIC resources (`app.kubernetes.io/name`, `cluster`) | Covered | Use a consistent label set across all CRs. |
| `location` | `HCloudServer.spec.location` | Covered | Set per server/pool object. |
| `network_zone` | `HCloudNetwork.spec.networkZones[]` | Covered | Usually `eu-central` for EU regions. |
| `private_network_subnet` | `HCloudNetwork.spec.ipRange` | Covered | CIDR is immutable after network creation. |
| `masters_pool.instance_type` | `HCloudServer.spec.serverType` on master objects | Covered | Mutable with controller-driven type reconciliation. |
| `masters_pool.instance_count` | Number of master `HCloudServer` objects | Partial | Manual/object-count based today; no pool CRD yet. |
| `worker_node_pools[*].instance_type` | `HCloudServer.spec.serverType` on worker objects | Covered | Label workers by pool name for grouping. |
| `worker_node_pools[*].instance_count` | Number of worker `HCloudServer` objects | Partial | Manual/object-count based today; no autoscaling pool CRD yet. |
| SSH key input (`public_ssh_key_path`) | `HCloudServer.spec.sshKeys` | Covered | HKIC expects key names/IDs already present in Hetzner. |
| `k3s_version` | Cloud-init (`spec.userData`) install channel/version env | Partial | Supported via `userData`, but no dedicated typed field yet. |
| `disable_traefik` | Cloud-init args in `spec.userData` (`--disable traefik`) | Covered | Already used in k3s samples. |
| `cni` | Cloud-init args in `spec.userData` | Partial | Works, but currently template-level and user-managed. |
| firewall enabled/rules | `HCloudFirewall.spec.rules` + `applyTo` | Covered | Supports `serverRefs` and Hetzner label selector. |
| API source ranges | `HCloudFirewall` ingress rule source CIDRs | Covered | Model as TCP 6443 ingress restrictions. |
| load balancer enabled/type | `HCloudLoadBalancer.spec.loadBalancerType` | Covered | Targets synced from server labels via selector. |
| CCM enabled | Day-2 manifests (`docs/k3s-day2.md`) | Covered | Installed separately from infra CRDs. |
| CSI enabled | Day-2 manifests (`docs/k3s-day2.md`) | Covered | Installed separately from infra CRDs. |
| Cluster Autoscaler | `config/samples/complex/k3s-day2-cluster-autoscaler/README.md` | Partial | Upstream Hetzner provider creates servers via API; coordinate with `HCloudServer` GitOps (see README). |
| System Upgrade Controller | `config/samples/complex/k3s-day2-system-upgrade/README.md` | Covered | SUC + k3s `Plan` samples; in-cluster upgrades independent of HKIC. |

## Practical translation pattern

For a one-master/two-worker shape:

- Create one `HCloudNetwork`.
- Create three `HCloudServer` resources:
  - one labeled `role=k3s-server`
  - two labeled `role=k3s-agent`
- Attach all servers to the same `networkRef`.
- Apply `HCloudFirewall` rules for SSH/API and attach via labels or `serverRefs`.
- Run bootstrap/join helpers:
  - `contrib/k3s-optional/configure-k3s-join-agents.sh k3s-join-server`
  - `contrib/k3s-optional/verify-k3s-join-cluster.sh k3s-join-server`
- Install Day-2 components from `docs/k3s-day2.md`.

See sample:

- `config/samples/complex/k3s-multi-node-join/k3s_multi_node_join_v1alpha1.yaml`

## Gaps and extensions

Current gaps versus a single `cluster.yaml` UX:

1. No first-class node pool CRD (`count/min/max`) yet.
2. No dedicated typed API for k3s version/CNI; currently cloud-init driven.
3. Cluster Autoscaler integration with **HKIC-owned** node pools is operational documentation only; GitOps-native scale-out remains “add/remove `HCloudServer` workers”.

Planned direction (roadmap-aligned):

- optional higher-level composition (Helm or kro) that renders the lower-level HKIC CRDs
- parity checklist/E2E that validates equivalent outcomes (nodes, networking, LB, storage)
- fuller Day-2 package beyond CCM+CSI
