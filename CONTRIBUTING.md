# Contributing to HCloud Operator

Thank you for your interest in contributing! This project is maintained by a single developer, but community contributions (bug fixes, features, documentation) are highly appreciated. 

To keep things simple and avoid over-engineering, we follow a lightweight workflow.

## Development Environment Setup

1. **Fork and Clone:**
   Fork the repository and clone it locally:
   ```bash
   git clone https://github.com/<your-username>/hcloud-operator.git
   cd hcloud-operator
   ```

2. **Prerequisites:**
   - Go 1.25+
   - Docker or Podman
   - `kubectl` and `make`
   - A local Kubernetes cluster (like `kind`) for testing.

3. **Running Tests:**
   Before submitting code, ensure all tests pass and code is properly formatted:
   ```bash
   make fmt
   make vet
   make test
   ```

   **Optional — real Hetzner E2E** (creates and deletes a cheap `cx23` server; requires `HCLOUD_TOKEN`):
   ```bash
   export HCLOUD_TOKEN="your-hetzner-cloud-token"
   make test-e2e-real
   ```
   This target uses the `e2e_real` build tag and is **not** part of the default `make test` gate. CI runs it on `workflow_dispatch` or when the `HCLOUD_TOKEN` repository secret is configured (see `.github/workflows/e2e-real.yaml`).

4. **Testing Locally:**
   You can run the operator locally against your Kubernetes cluster (ensure you have the `HCLOUD_TOKEN` exported in your environment):
   ```bash
   make install
   export HCLOUD_TOKEN="your-hetzner-cloud-token"
   make run
   ```

## Pull Request Process

1. Create a branch for your feature or fix (e.g., `feat/add-volume-support` or `fix/reconciler-bug`).
2. Make your changes and write tests if applicable.
3. Ensure `make test`, `make fmt`, and `make vet` succeed.
4. Commit your changes. Please use conventional commit messages if possible (e.g., `feat: ...`, `fix: ...`).
5. Open a Pull Request against the `main` branch.
6. The CI pipeline will automatically run tests to verify your changes.

Once the PR is reviewed and CI passes, it will be merged into `main`.

Thank you for your help in making this project better!
