# Design: Milestone 2 — Production Hardening (A)

Status: Approved (design)
Date: 2026-06-07
Scope: Production trust for the existing ACK-style Hetzner controllers — observability, opt-in real E2E, and a shared base reconciler. Developer-experience work for complex stacks (kro recipes, k3s samples) is tracked separately as "B" and gets its own design/spec after A lands.

## Background and goals

HKIC already satisfies the core ACK-for-Hetzner vision: 9 CRDs, one reconciler per concern, finalizers, status conditions, idempotent adoption, Helm + Argo install paths. Milestone 1 (API coverage) is complete.

The gap to "production-grade ACK for Hetzner" is operational maturity, not vision or API surface:

- No Kubernetes Events — `kubectl describe` does not tell the operational story.
- No custom Prometheus metrics — limited SRE visibility beyond controller-runtime defaults.
- Reconcile boilerplate (fetch, finalizer, deletion, status-retry) is duplicated across 9 controllers, making consistent observability expensive to add and easy to drift.
- Leader election is off by default — unsafe to scale replicas.
- No real-Hetzner E2E — regression risk against live API behavior.
- Minor sample/doc drift (LB `spec.services` is implemented but a sample still says to use the console).

Goal of A: make the existing primitives trustworthy in production GitOps without changing their behavior. Behavior parity with today is a hard requirement.

## Non-goals

- No new Hetzner resource CRDs.
- No full Crossplane-style Observe/Create/Update/Delete decomposition (considered and rejected for now — see Approach 2 below).
- No Grafana dashboards shipped (metric names documented so users can build their own / wire a ServiceMonitor).
- No kro / k3s DX work (that is B).

## Design decisions (from brainstorming)

- E2E: real Hetzner E2E, opt-in only (runs on `workflow_dispatch` or when `HCLOUD_TOKEN` secret present). Roadmap #14.
- Observability depth: full — Events + richer conditions + custom Prometheus metrics. Roadmap #11.
- Implementation approach: a shared base reconciler that handles cross-cutting concerns once (chosen over per-controller duplication and over a full Crossplane-style rewrite).
- Leader election: default ON.

## Architecture

### 1. Base reconciler (`internal/reconcile`)

A generic driver owns the reconcile loop skeleton once; each controller supplies only its domain logic.

```go
// Managed is implemented by all HCloud* root types.
type Managed interface {
    client.Object
    GetConditions() *[]metav1.Condition
}

// Resource is the per-kind domain contract.
type Resource[T Managed] interface {
    NewObject() T                                              // e.g. &HCloudServer{}
    FinalizerName() string
    Kind() string                                             // events/metrics label
    Reconcile(ctx context.Context, obj T) (ctrl.Result, error) // converge external state
    Delete(ctx context.Context, obj T) error                   // external cleanup before finalizer removal
}

type BaseReconciler[T Managed] struct {
    client.Client
    Recorder record.EventRecorder
    Resource Resource[T]
}
```

`BaseReconciler.Reconcile` handles, for every kind:

1. Fetch object; on not-found, return cleanly.
2. Start reconcile timer (metrics).
3. Deletion: if `DeletionTimestamp` set and finalizer present → `Resource.Delete` → remove finalizer → `Normal Deleted` event.
4. Ensure finalizer (add + requeue if missing).
5. Delegate to `Resource.Reconcile`.
6. On error: set `Synced=False` + `Ready=False` (reason `ReconcileError`), emit `Warning ReconcileError`, record `result="error"` metric, persist status with retry-on-conflict, return error.
7. On success: set `Synced=True`, persist status with retry-on-conflict, record `result="success"` metric.

The retry-on-conflict status update is promoted from the existing `updateServerStatusWithRetry` into the base and made generic over `T`.

Per-controller `SetupWithManager` stays in each controller (the `Watches`/`Owns` wiring differs: Volume/LB/FloatingIP/PrimaryIP watch `HCloudServer`, LB also watches `HCloudCertificate`). The base only provides `Reconcile`.

Wiring pattern (per controller): the existing `HCloud<Kind>Reconciler` implements `Resource[*HCloud<Kind>]`; `SetupWithManager` constructs a `BaseReconciler[*HCloud<Kind>]{Client, Recorder, Resource: r}` and registers it with `For(...).<Watches...>.Complete(base)`.

Migration order: migrate `HCloudServer` first as the reviewed reference template, confirm envtest parity, then migrate the remaining 8 controllers one per change.

### 2. Richer conditions

Adopt the two-condition convention used by Crossplane/ACK:

- `Synced` — the last reconcile loop completed without error (control-plane health). Owned by the base.
- `Ready` — the external Hetzner resource is provisioned and usable (data-plane). Owned by domain logic, as today.

Each status type gains a trivial `GetConditions() *[]metav1.Condition` returning `&obj.Status.Conditions` (all 9 already have `Conditions []metav1.Condition`). Existing per-phase reasons (Creating, Attaching, Resizing, NetworkMigrated, CertificatePending, …) are preserved on `Ready`.

### 3. Kubernetes Events

Recorder from `mgr.GetEventRecorderFor("hcloud-<kind>")`, injected into the base and available to domain logic.

- Base (generic): `Normal Deleted`, `Warning ReconcileError`, `Warning DeleteFailed`.
- Domain logic (per controller, via injected recorder): `Normal Created`, `Normal Adopted`, `Normal Resizing`, `Normal Attached`/`Detached`, `Normal NetworkMigrated`, etc., mirroring existing condition reasons.

### 4. Custom Prometheus metrics (`internal/metrics`)

Registered to controller-runtime's metrics registry (`sigs.k8s.io/controller-runtime/pkg/metrics`.Registry), served on the existing `:8082/metrics`:

- `hcloud_reconcile_total{kind,result}` — counter (`result` ∈ success|error), recorded by base.
- `hcloud_reconcile_duration_seconds{kind}` — histogram, recorded by base.
- `hcloud_api_requests_total{operation,result}` — counter, recorded by a thin wrapper around the hcloud client.
- `hcloud_api_request_duration_seconds{operation}` — histogram, recorded by the client wrapper.

The hcloud client wrapper records per-operation latency/result without changing call sites' behavior (same return values).

### 5. Leader election

Flip `--leader-elect` flag default to `true` in `cmd/main.go`. Expose `leaderElection.enabled` (default `true`) in the Helm chart and ensure lease RBAC is present (controller-runtime requires `coordination.k8s.io` leases). Document that >1 replica is now safe.

### 6. Opt-in real Hetzner E2E (#14)

- Default required gate is unchanged: envtest + fake client.
- Real E2E gated behind a build tag (e.g. `//go:build e2e_real`) and `HCLOUD_TOKEN` presence.
- New CI job `e2e-real` in `.github/workflows/`, triggered only by `workflow_dispatch` or when the `HCLOUD_TOKEN` repo secret exists. It:
  - uses a unique name prefix (e.g. `hkic-e2e-<run_id>`) and a cheap server type/location,
  - creates a minimal `HCloudServer` (+ optional `HCloudVolume`), waits for `Ready`,
  - deletes and asserts Hetzner-side cleanup,
  - always runs cleanup in `defer`/teardown to avoid orphaned cost.

### 7. Docs polish

- Fix `config/samples/complex/hcloud-stack/hcloud_stack_v1alpha1.yaml` header comment (LB services are implemented via `spec.services`) and add a sample `services:` block.
- Add `docs/observability.md`: event catalog, metric reference, and a `ServiceMonitor` snippet.
- Update `docs/roadmap.md` / `docs/hcloud-api-coverage.md` as #11/#14 progress lands.

## Testing strategy

- Base reconciler: unit tests with a fake client + a stub `Resource[T]` implementation covering: not-found, finalizer add/requeue, deletion → `Delete` → finalizer removal, `Reconcile` error → `Synced=False`/`Ready=False` + `Warning` event + error metric, status update retry-on-conflict.
- Events: assert with `record.NewFakeRecorder`.
- Metrics: assert with `testutil.ToFloat64` / `testutil.CollectAndCount`.
- Parity gate: all existing envtest controller suites must pass unchanged after each controller migration.

## Risks and prerequisites

- Task 0 — envtest is currently failing locally ("unable to start control plane"), i.e. missing kubebuilder/`setup-envtest` assets. Environmental, not a code bug, but the parity gate is meaningless until it runs. Fix (install/setup envtest assets, wire `make test` to `setup-envtest`) before migrating controllers.
- Migrating 9 controllers is the main risk surface. Mitigation: Server first as a reviewed reference, then one controller per change, parity gate after each.
- Generic base must not change requeue timing/semantics for any controller — preserve existing requeue intervals and custom requeue-after values.

## Out of scope / follow-up (B)

After A: a separate design/spec for developer experience on complex stacks — kro `ResourceGraphDefinition` recipes (e.g. `HetznerWebStack`, `HetznerK3sCluster`) composing existing CRDs, and tightened k3s samples. Tracked as roadmap #16.
