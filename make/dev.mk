##@ Development

CLUSTER ?= dolt
KIND_CONFIG ?= hack/manifests/kind/kind.yaml

.PHONY: cluster
cluster:
	$(KIND) create cluster --name $(CLUSTER) --config $(KIND_CONFIG)

.PHONY: cluster-delete
cluster-delete:
	$(KIND) delete cluster --name $(CLUSTER)

.PHONY: cluster-ctx
cluster-ctx: ## Sets cluster context.
	$(KUBECTL) config use-context kind-$(CLUSTER)

.PHONY: test-int
test-int: ## Run tests.
	go test ./internal/controller/... -v -ginkgo.v --coverprofile=cover.out --timeout 5m 

.PHONY: test
test: ## Run tests.
	go test $$(go list ./...| grep -v /internal/controller) -race -coverprofile cover.out

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

tiltdev: cluster-ctx
	tilt up

tiltci: cluster cluster-ctx
	tilt ci
