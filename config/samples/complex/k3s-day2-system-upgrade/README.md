# k3s Day-2: System Upgrade Controller (automated k3s upgrades)

This folder documents **[Rancher System Upgrade Controller](https://github.com/rancher/system-upgrade-controller)** (SUC) for **k3s**, matching the flow described in [k3s automated upgrades](https://docs.k3s.io/upgrades/automated).

SUC runs **inside the workload cluster**. It is independent of HKIC; HKIC continues to own **infrastructure** CRDs while SUC drives **k3s binary / data-plane** upgrades on existing nodes.

## Prerequisites

- Workload kubeconfig.
- **Server nodes upgraded before agents** — the sample agent `Plan` uses a `prepare` step that waits for the server `Plan` name you choose.

## 1) Install the controller

Follow upstream releases (URLs track `latest` in k3s docs):

```bash
kubectl apply -f https://github.com/rancher/system-upgrade-controller/releases/latest/download/crd.yaml \
  -f https://github.com/rancher/system-upgrade-controller/releases/latest/download/system-upgrade-controller.yaml
```

Wait until the controller is running:

```bash
kubectl -n system-upgrade rollout status deploy/system-upgrade-controller
```

## 2) Apply Plans

Example manifests (pinned **version** — edit to your target k3s release and respect [Kubernetes version skew](https://kubernetes.io/releases/version-skew-policy/)):

```bash
kubectl apply -f config/samples/complex/k3s-day2-system-upgrade/k3s-plan-server.yaml
kubectl apply -f config/samples/complex/k3s-day2-system-upgrade/k3s-plan-agent.yaml
```

- `k3s-plan-agent.yaml` references the server plan name **`k3s-server`** in `prepare` — keep names in sync if you rename resources.

### Channel-based upgrades (optional)

Instead of `spec.upgrade.version`, you can set `spec.upgrade.channel` (for example `https://update.k3s.io/v1-release/channels/stable`) per k3s docs. That continuously tracks the channel; use only if you explicitly want automatic upgrades.

## 3) Observe

```bash
kubectl -n system-upgrade get plans,jobs
kubectl get nodes -o wide
```

## 4) Cleanup

```bash
kubectl delete -f config/samples/complex/k3s-day2-system-upgrade/
kubectl delete -f https://github.com/rancher/system-upgrade-controller/releases/latest/download/system-upgrade-controller.yaml
kubectl delete -f https://github.com/rancher/system-upgrade-controller/releases/latest/download/crd.yaml
```

Deleting CRDs removes `Plan` objects; only do this when you intend to remove SUC entirely.
