# hcloud-operator Makefile
# ──────────────────────────────────────────────────────────────────────────────
# Variables
# ──────────────────────────────────────────────────────────────────────────────

# Container image for docker-build / deploy-img (override when deploying a release), e.g.:
#   ghcr.io/armanfeyzi/hcloud-operator:v0.7.0
IMG ?= localhost/hcloud-operator:dev

# Released install manifest (GitHub Releases publishes install.yaml per tag).
VERSION ?= v0.7.0
RELEASE_INSTALL_URL ?= https://github.com/armanfeyzi/hcloud-operator/releases/download/$(VERSION)/install.yaml

# Go settings
GOFLAGS     ?=
GOOS        ?= $(shell go env GOOS)
GOARCH      ?= $(shell go env GOARCH)

# Tool versions
CONTROLLER_TOOLS_VERSION ?= latest
ENVTEST_VERSION          ?= release-0.19
# Kubernetes control-plane version for envtest (etcd + kube-apiserver binaries).
# Must be defined so `setup-envtest use` downloads a deterministic version on a
# clean checkout; without it the test harness resolves to an empty asset path
# and envtest fails with "unable to start control plane".
ENVTEST_K8S_VERSION      ?= 1.31.0

# kro (composition layer) — pinned for RGD API compatibility
# Tool binary locations
LOCALBIN    ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST         ?= $(LOCALBIN)/setup-envtest

# ──────────────────────────────────────────────────────────────────────────────
# Default target
# ──────────────────────────────────────────────────────────────────────────────
.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ──────────────────────────────────────────────────────────────────────────────
# Code generation
# ──────────────────────────────────────────────────────────────────────────────
.PHONY: generate
generate: controller-gen ## Generate DeepCopy, DeepCopyObject implementations
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: manifests
manifests: controller-gen ## Generate CRD and RBAC manifests
	$(CONTROLLER_GEN) rbac:roleName=hcloud-operator-role \
		crd \
		webhook \
		paths="./..." \
		output:crd:artifacts:config=config/crd/bases \
		output:rbac:artifacts:config=config/rbac

# ──────────────────────────────────────────────────────────────────────────────
# Build
# ──────────────────────────────────────────────────────────────────────────────
.PHONY: fmt
fmt: ## Run go fmt
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: build
build: generate fmt vet ## Build the operator binary
	go build $(GOFLAGS) -o bin/manager ./cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run the operator locally (requires HCLOUD_TOKEN and kubeconfig)
	go run ./cmd/main.go

# ──────────────────────────────────────────────────────────────────────────────
# Tests
# ──────────────────────────────────────────────────────────────────────────────
.PHONY: setup-envtest
setup-envtest: envtest ## Download envtest control-plane binaries (etcd, kube-apiserver) for ENVTEST_K8S_VERSION
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path >/dev/null
	@echo "envtest assets: $$($(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)"

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run unit/integration tests
	KUBEBUILDER_ASSETS="$$($(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		go test ./... -coverprofile cover.out -v

.PHONY: test-e2e
test-e2e: manifests generate fmt vet setup-envtest ## Run end-to-end tests (requires real HCLOUD_TOKEN and cluster)
	KUBEBUILDER_ASSETS="$$($(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		go test ./test/e2e/... -v -timeout 30m

# ──────────────────────────────────────────────────────────────────────────────
# Docker
# ──────────────────────────────────────────────────────────────────────────────
.PHONY: docker-build
docker-build: ## Build the operator Docker image
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push the operator Docker image
	docker push $(IMG)

# ──────────────────────────────────────────────────────────────────────────────
# Cluster install / uninstall
# ──────────────────────────────────────────────────────────────────────────────
.PHONY: install
install: manifests ## Install CRDs into the cluster
	kubectl apply -f config/crd/bases/

.PHONY: uninstall
uninstall: manifests ## Remove CRDs from the cluster
	kubectl delete -f config/crd/bases/ --ignore-not-found

.PHONY: deploy
deploy: manifests ## Deploy using kustomize in config/default/ (uses deployment image: localhost/... — build/load locally or use deploy-img / deploy-release)
	kubectl apply -k config/default/

# Deploy a published GHCR image without editing repo files (substitutes image in rendered YAML).
.PHONY: deploy-img
deploy-img: manifests ## Deploy operator with IMG set (e.g. IMG=ghcr.io/armanfeyzi/hcloud-operator:v0.7.0)
	@test "$(IMG)" != "localhost/hcloud-operator:dev" || (echo "Set IMG to a pushed image, e.g. IMG=ghcr.io/<owner>/hcloud-operator:v0.7.0" >&2; false)
	kubectl kustomize config/default | sed 's|localhost/hcloud-operator:dev|$(IMG)|g' | kubectl apply -f -

.PHONY: deploy-release
deploy-release: ## Apply official install.yaml from GitHub Releases (set VERSION=v0.7.0)
	kubectl apply -f "$(RELEASE_INSTALL_URL)"

.PHONY: undeploy
undeploy: ## Tear down everything in config/default/ (includes CRDs — destructive for HKIC resources)
	kubectl delete -k config/default/ --ignore-not-found

# ──────────────────────────────────────────────────────────────────────────────
# Helm
# ──────────────────────────────────────────────────────────────────────────────
HELM_CHART ?= charts/hcloud-operator

.PHONY: helm-sync-crds
helm-sync-crds: ## Copy CRD bases into the Helm chart crds/ directory
	cp config/crd/bases/*.yaml $(HELM_CHART)/crds/

.PHONY: helm-lint
helm-lint: helm-sync-crds ## Lint the Helm chart
	helm lint $(HELM_CHART)

.PHONY: helm-template
helm-template: helm-sync-crds ## Render the Helm chart locally
	helm template hcloud-operator $(HELM_CHART) --namespace hcloud-operator-system

.PHONY: helm-package
helm-package: helm-lint ## Package chart as dist/hcloud-operator-<version>.tgz
	@mkdir -p dist
	helm package $(HELM_CHART) -d dist/

# ──────────────────────────────────────────────────────────────────────────────
# Tools
# ──────────────────────────────────────────────────────────────────────────────
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)
