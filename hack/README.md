# `hack/` — codegen only

This directory holds **repository build helpers** (for example Go license boilerplate for `controller-gen`).

Optional **k3s / SSH cookbook scripts** live under **`contrib/k3s-optional/`** — see [contrib/k3s-optional/README.md](../contrib/k3s-optional/README.md).

For backwards compatibility, **`hack/configure-k3s-join-agents.sh`**, **`hack/verify-k3s-join-cluster.sh`**, and **`hack/export-k3s-kubeconfig-to-secret.sh`** are thin wrappers that `exec` the corresponding scripts under `contrib/k3s-optional/`. Prefer calling the `contrib/` paths in new docs and automation.
