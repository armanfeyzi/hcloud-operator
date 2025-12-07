# hcloud-operator Makefile
# ──────────────────────────────────────────────────────────────────────────────
# Variables
# ──────────────────────────────────────────────────────────────────────────────

# Image name — override with: make docker-build IMG=myregistry/hcloud-operator:v0.1.0
IMG ?= hcloud-operator:dev

# Go settings
GOFLAGS     ?=
GOOS        ?= $(shell go env GOOS)
GOARCH      ?= $(shell go env GOARCH)

# Tool versions
CONTROLLER_TOOLS_VERSION ?= latest
ENVTEST_VERSION          ?= release-0.19

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
.PHONY: test
test: manifests generate fmt vet envtest ## Run unit/integration tests
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		go test ./... -coverprofile cover.out -v

.PHONY: test-e2e
test-e2e: manifests generate fmt vet envtest ## Run end-to-end tests (requires real HCLOUD_TOKEN and cluster)
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
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
deploy: manifests ## Deploy the operator to the cluster (requires IMG to be set and pushed)
	kubectl apply -k config/default/

.PHONY: undeploy
undeploy: ## Remove the operator from the cluster
	kubectl delete -k config/default/ --ignore-not-found

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
