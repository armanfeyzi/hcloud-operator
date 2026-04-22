# Hetzner Cloud API coverage matrix

This table maps **Hetzner Cloud API resource families** to **HKIC CRDs**. It is the primary planning doc for the ACK-style main track. See [roadmap.md](roadmap.md) and GitHub milestone **[HKIC: Hetzner Cloud CRD coverage](https://github.com/armanfeyzi/hcloud-operator/milestone/1)**.

**Legend:** **Done** = CRD + reconciler shipped · **Planned** = tracked issue · **Partial** = subset of API surface · **N/A** = intentionally out of scope (with rationale)

| Hetzner API area | HKIC CRD / behavior | Status | Notes / issue |
|---|---|---|---|
| **Servers** | `HCloudServer` | **Done** | Create, delete, adopt-by-name, status sync, `serverType` changes with optional `upgradeDisk`, `networkRef`, `userData`, labels, SSH key names at create |
| **Volumes** | `HCloudVolume` | **Done** | Create, delete, attach/detach, **size increase** via resize API |
| **Load Balancers** | `HCloudLoadBalancer` | **Done** | Create, delete, target sync, **services + health checks** |
| **Networks** | `HCloudNetwork` | **Done** | Private networks, Cloud subnets per zone, labels |
| **Firewalls** | `HCloudFirewall` | **Done** | Rules, labels, attach via `serverRefs` and/or Hetzner label selector |
| **Placement Groups** | `HCloudPlacementGroup` | **Done** | Spread/cluster groups; `HCloudServer.spec.placementGroupRef` at create time (#2, #3) |
| **Primary IPs** | `HCloudPrimaryIP` | **Planned** | Assignable IPv4/IPv6 (#4) |
| **Floating IPs** | `HCloudFloatingIP` | **Planned** (optional) | (#5) |
| **Certificates** | `HCloudCertificate` | **Planned** | Managed certs; LB TLS integration (#6) |
| **Volume snapshots** | `HCloudVolumeSnapshot` (TBD) | **Planned** | Spec model TBD (#7) |
| **SSH Keys** | `HCloudSSHKey` | **Planned** (low) | Today: pass key **names** on `HCloudServer.spec.sshKeys` only (#8) |
| **Images** | — | **N/A** | Referenced by name on server create; catalog is read-only in Hetzner |
| **Server types** | — | **N/A** | Referenced by name on server create |
| **Locations / datacenters** | — | **N/A** | Referenced by name on create |
| **Actions** (power on/off, rebuild, …) | Inline in `HCloudServer` | **Partial** | Power off/on and `change_type` used for resize; no standalone Action CRD |
| **ISOs** | — | **N/A** | Not targeted for v1alpha1 |
| **Storage Boxes** | — | **N/A** | Separate Hetzner product line |
| **Pricing / zones (catalog)** | — | **N/A** | Read-only metadata |

## Hardening backlog (existing CRDs)

| Area | Gap | Issue |
|---|---|---|
| All controllers | Kubernetes Events, Prometheus metrics, richer conditions | #11 |

## Optional composition (not API coverage)

Recipes that **use** the CRDs above but are not separate reconcilers:

| Outcome | Docs / samples | Notes |
|---|---|---|
| k3s single- or multi-node on Hetzner | `docs/k3s-on-hcloud.md`, `config/samples/complex/k3s-*` | `userData` + optional `contrib/k3s-optional/` |
| Day-2 add-ons (CCM, CSI, …) | `docs/k3s-day2.md` | Apply upstream Helm/manifests; not HKIC CRDs |
| hetzner-k3s `cluster.yaml` mapping | `docs/hetzner-k3s-cluster-yaml-mapping.md` | Reference only |

When adding a new CRD, update this table and [crds.md](crds.md).
