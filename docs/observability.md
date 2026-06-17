# Observability

HKIC exposes Kubernetes **Events**, **conditions**, and **Prometheus metrics** for every `HCloud*` controller. All nine reconcilers share the same generic base loop (`internal/reconcile`), so behavior is consistent across kinds.

## Conditions

Each resource exposes two condition types:

| Type | Meaning | Owner |
|------|---------|--------|
| `Synced` | Last reconcile loop completed without error (control-plane health) | Base reconciler |
| `Ready` | External Hetzner resource is provisioned and usable (data-plane) | Domain logic |

On reconcile error, the base sets both `Synced=False` and `Ready=False` with reason `ReconcileError`.

```bash
kubectl get hcloudserver demo -o jsonpath='{range .status.conditions[*]}{.type}={.status} ({.reason}){"\n"}{end}'
```

## Kubernetes Events

The base reconciler emits warning events on reconcile/delete failures and a normal `Deleted` event after external cleanup.

```bash
kubectl get events --field-selector involvedObject.kind=HCloudServer
```

Per-controller event sources: `hcloud-server`, `hcloud-volume`, `hcloud-loadbalancer`, `hcloud-network`, `hcloud-firewall`, `hcloud-placementgroup`, `hcloud-primaryip`, `hcloud-floatingip`, `hcloud-certificate`.

## Prometheus metrics

The operator serves metrics on **`:8082/metrics`** (configurable via `--metrics-bind-address`). Custom collectors are registered on the controller-runtime registry.

### Reconcile metrics

| Metric | Labels | Description |
|--------|--------|-------------|
| `hcloud_reconcile_total` | `kind`, `result` | Reconcile count (`result` = `success` or `error`) |
| `hcloud_reconcile_duration_seconds` | `kind` | Reconcile latency histogram |

`kind` is the stable controller name, e.g. `HCloudServer`, `HCloudVolume`.

### Hetzner API metrics

| Metric | Labels | Description |
|--------|--------|-------------|
| `hcloud_api_requests_total` | `operation`, `result` | API call count per operation |
| `hcloud_api_request_duration_seconds` | `operation` | API call latency histogram |

Operations match `internal/hcloud.Interface` method names (`GetServer`, `CreateVolume`, …). The production client is wrapped with `hcloud.Instrument()` in `cmd/main.go`.

### Example queries

```promql
# Reconcile error rate by kind (5m)
sum by (kind) (rate(hcloud_reconcile_total{result="error"}[5m]))
  / sum by (kind) (rate(hcloud_reconcile_total[5m]))

# p95 Hetzner API latency for volume operations
histogram_quantile(0.95, sum by (le, operation) (
  rate(hcloud_api_request_duration_seconds_bucket{operation=~"GetVolume|CreateVolume|AttachVolume"}[5m])
))
```

## ServiceMonitor (Prometheus Operator)

When using the [Helm chart](../charts/hcloud-operator/), metrics are exposed on the pod at port `metrics` (default **8082**). Example `ServiceMonitor`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: hcloud-operator
  labels:
    release: prometheus  # match your Prometheus Operator release label
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: hcloud-operator
  endpoints:
    - port: metrics
      path: /metrics
      interval: 30s
```

Enable a metrics `Service` in chart values if your deployment does not already expose one (`service.metrics.enabled` when available).

## Leader election

Leader election defaults **on** (`--leader-elect=true`) so running more than one operator replica is safe. Lease RBAC is included in the Helm chart ClusterRole.

## Related docs

- [Helm chart](../charts/hcloud-operator/README.md)
- [Roadmap — observability track](roadmap.md)
