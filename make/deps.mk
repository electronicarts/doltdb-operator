##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= $(LOCALBIN)/kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
GOLINES = $(LOCALBIN)/golines
KIND ?= kind
TILT ?= $(LOCALBIN)/tilt

## Tool Versions
KUSTOMIZE_VERSION ?= v5.4.3
CONTROLLER_TOOLS_VERSION ?= v0.16.1
ENVTEST_VERSION ?= release-0.19
GOLANGCI_LINT_VERSION ?= v1.63.4
GOLINES_VERSION ?= latest
KIND_VERSION ?= v0.30.0

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: golines
golangci-lint: $(GOLINES) ## Download golines locally if necessary.
$(GOLINES): $(LOCALBIN)
	$(call go-install-tool,$(GOLINES),github.com/segmentio/golines,$(GOLINES_VERSION))

.PHONY: kubectl
kubectl: ## Download kubectl locally if necessary.
ifeq (,$(wildcard $(KUBECTL)))
ifeq (,$(shell which kubectl 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(KUBECTL)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(KUBECTL) https://dl.k8s.io/release/v1.31.0/bin/linux/$${ARCH}/kubectl ;\
	chmod +x $(KUBECTL) ;\
	}
else
KUBECTL = $(shell which kubectl)
endif
endif

.PHONY: tilt
tilt:
	curl -fsSL https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.sh | bash

.PHONY: kind
kind:
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -Lo ./kind https://kind.sigs.k8s.io/dl/$(KIND_VERSION)/kind-$${OS}-$${ARCH}
	chmod +x ./kind
	mv ./kind /usr/local/bin/kind

.PHONY: ctlpl
ctlpl: $(CTLPTL)
	$(call go-install-tool,$(CTLPTL),github.com/tilt-dev/ctlptl/cmd/ctlptl,latest)

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
echo "$(1)-$(3) $(1)"
ln -sf $(1)-$(3) $(1)
endef


.PHONY: install-tilt-ci
install-tilt-ci: TILT_VERSION ?= 0.33.21
install-tilt-ci:
	curl -fsSL https://github.com/tilt-dev/tilt/releases/download/v$(TILT_VERSION)/tilt.$(TILT_VERSION).linux-alpine.x86_64.tar.gz | tar -xzv tilt && \
	mv tilt /usr/local/bin/tilt

.PHONY: install-kustomize-ci
install-kustomize-ci: KUSTOMIZE_VERSION ?= v5.5.0
install-kustomize-ci:
	curl -fsSL https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/$(KUSTOMIZE_VERSION)/kustomize_$(KUSTOMIZE_VERSION)_linux_amd64.tar.gz | tar -xzv kustomize && \
	mv kustomize /usr/local/bin/kustomize

.PHONY: install-go-ci
install-go-ci: GO_VERSION ?= 1.23.3
install-go-ci:
	curl -LO https://go.dev/dl/go$(GO_VERSION).linux-amd64.tar.gz && tar -C /usr/local -xvzf go$(GO_VERSION).linux-amd64.tar.gz