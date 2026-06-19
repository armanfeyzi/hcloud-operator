# Sprint plan: GitOps platform (#12 Helm + #13 Argo CD)

This document scopes the **GitOps platform sprint** (Helm + Argo CD), now **complete**. It intentionally **does not** include `HCloudSSHKey` (deferred — Linear [ARM-72](https://linear.app/armanfeyzi/issue/ARM-72)); SSH keys remain reference-by-name on `HCloudServer` until a user asks for fully declarative key lifecycle.

**References:** [roadmap.md](roadmap.md) · sample stack `config/samples/complex/hcloud-stack/`

---

## Out of scope for this sprint

| Item | Rationale |
|---|---|
| `HCloudSSHKey` (#8) | Not blocking GitOps; bootstrap key in Hetzner/Terraform is ACK-normal |
| Observability (#11) | Follow-up sprint after install path exists |
| kro/Crossplane (#16) | Builds on stable Helm + Argo examples |
| New Hetzner CRDs | Milestone 1 complete |

---

## Sprint deliverables

### 1. Helm chart (#12)

**Directory:** `charts/hcloud-operator/` (standard layout)

| Chart piece | Source of truth today | Notes |
|---|---|---|
| CRDs | `config/crd/bases/*.yaml` | Ship as `crds/` templates or `crds/` directory; `helm install --skip-crds` documented if CRDs managed separately |
| Deployment | `config/manager/deployment.yaml` | Image via `values.image.repository` / `tag`; pull policy configurable |
| RBAC | `config/rbac/` | ServiceAccount, ClusterRole, ClusterRoleBinding |
| Namespace | `config/default/namespace.yaml` | Optional `values.namespaceOverride` |
| Secret contract | README + deployment env | `secret.name`, `secret.key` (default `token` → `HCLOUD_TOKEN`) — **not** committed; optional `secret.create` + `secret.token` for dev only |
| Values | — | `leaderElect`, `resources`, `replicaCount` (default 1), metrics/health ports |

**Acceptance criteria**

- `helm install` / `helm upgrade` against a test cluster installs operator + all CRDs
- Documented flow: create Secret (or use external-secrets) → install chart → `kubectl get pods -n hcloud-operator-system`
- Chart version aligned with app version in `Chart.yaml`; CI or Makefile target `helm lint` + optional `helm template` golden check
- README section: “Install with Helm” alongside existing release `install.yaml` and kustomize paths

**Suggested tasks (order)**

1. Scaffold chart with `helm create` baseline, strip unused templates
2. Copy/adapt manifests from `config/default` + CRD bases; parameterize image and namespace
3. Add `values.schema.json` for required fields when `secret.create=false`
4. `make helm-lint` (or document `helm lint charts/hcloud-operator`)
5. Publish chart in release workflow (OCI or `.tgz` on GitHub Releases) — can be a fast follow if sprint time is tight

**Estimate:** 2–3 days

---

### 2. Argo CD stack example (#13)

**Directory:** `examples/argo/` (or `config/argo/`)

**Pattern:** App-of-Apps or single Application with **sync waves** so refs resolve in order:

```text
wave -1: Namespace + operator (Helm chart from #12, or OCI/release)
wave  0: HCloudNetwork, HCloudFirewall, HCloudCertificate (if HTTPS LB)
wave  1: HCloudServer, HCloudPlacementGroup (optional)
wave  2: HCloudVolume, HCloudPrimaryIP / HCloudFloatingIP (optional)
wave  3: HCloudLoadBalancer (depends on server labels / certificateRefs)
```

**Base manifest:** extend [hcloud-stack sample](../config/samples/complex/hcloud-stack/hcloud_stack_v1alpha1.yaml) — network, server, volume, load balancer — with:

- Placeholder comments for `spec.sshKeys` (key must exist in Hetzner project)
- Optional `HCloudFirewall` + `HCloudCertificate` fragments in `examples/argo/manifests/` for a “production-shaped” variant

**Acceptance criteria**

- `examples/argo/README.md` with prerequisites: management cluster, Argo CD, `HCLOUD_TOKEN` secret, Hetzner SSH key name
- One `Application` (or `ApplicationSet` stub) that syncs the wave-ordered stack
- Document bootstrap: install CRDs/operator first (Helm Application wave -1), then infra Applications
- Link from main README under “GitOps / Argo CD”

**Suggested tasks (order)**

1. Split stack YAML by wave with `argocd.argoproj.io/sync-wave` annotations
2. Add `Application` for operator (Helm source) + `Application` for infra (directory or kustomize)
3. Optional: `AppProject` + resource whitelist for `infra.hkc.io/*`
4. Manual test checklist in README (sync, health, `kubectl get` all Ready)

**Estimate:** 1–2 days

---

## Suggested sprint timeline (1 week)

| Day | Focus |
|---|---|
| 1 | Helm chart scaffold + CRDs + deployment/RBAC templated |
| 2 | Helm values, secret contract, README + `helm lint` |
| 3 | Argo wave manifests + operator Application |
| 4 | Argo infra Application + docs; manual sync test |
| 5 | Buffer: release chart packaging, fix review feedback |

---

## Verification checklist (end of sprint)

- [x] `helm template charts/hcloud-operator` renders without error (`make helm-template`)
- [ ] `helm install` on kind/minikube (or dev cluster) → operator pod Ready
- [ ] Argo CD sync: operator app Healthy, stack app Synced, `HCloudServer` reaches `status.state=running` (with valid token + SSH key name)
- [x] Docs: roadmap Phase 4 items #12/#13 linked; chart and Argo paths in README

---

## Follow-on (next sprint, not this one)

- **#15 Multi-cluster / multi-token** — documentation for multiple Hetzner projects and management clusters
- **#16 kro/Crossplane** — out of repo scope; users compose HKIC CRDs in their own tooling

### Shipped since this sprint

- **#11 Observability** — shared base reconciler, Events, Prometheus metrics, `Synced`/`Ready` conditions ([docs/observability.md](observability.md))
- **#14 CI E2E** — opt-in real-Hetzner test (`e2e_real` build tag, `make test-e2e-real`, `.github/workflows/e2e-real.yaml`)
