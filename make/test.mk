# CLUSTER ?= dolt
# KIND_CONFIG ?= hack/manifests/kind/kind.yaml

# .PHONY: cluster
# cluster:
# 	$(KIND) create cluster --name $(CLUSTER) --config $(KIND_CONFIG)

# .PHONY: cluster-delete
# cluster-delete:
# 	$(KIND) delete cluster --name $(CLUSTER)

# .PHONY: cluster-ctx
# cluster-ctx: ## Sets cluster context.
# 	$(KUBECTL) config use-context kind-$(CLUSTER)

# .PHONY: host-doltdb
# host-doltdb: ## Add doltdb hosts to /etc/hosts.
# 	@./hack/add_host.sh 201 doltdb-0.dolt-internal.default.svc.cluster.local
# 	@./hack/add_host.sh 202 doltdb-1.dolt-internal.default.svc.cluster.local
# 	@./hack/add_host.sh 203 doltdb-2.dolt-internal.default.svc.cluster.local
# 	@./hack/add_host.sh 204 doltdb-3.dolt-internal.default.svc.cluster.local
# 	@./hack/add_host.sh 205 foo-app.default.svc.cluster.local
# 	@./hack/add_host.sh 206 dolt.default.svc.cluster.local
# 	@./hack/add_host.sh 207 dolt-ro.default.svc.cluster.local

# .PHONY: host
# host: host-doltdb  ## Configure hosts for local development.

# .PHONY: net
# net: host ## Configure networking for local development.

# .PHONY: cidr
# cidr: ## Get CIDR used by KIND.
# 	@./hack/display_cidr.sh

# .PHONY: install-crds
# install-crds: cluster-ctx manifests kustomize ## Install CRDs.
# 	$(KUSTOMIZE) build config/crd | kubectl apply --server-side=true --force-conflicts -f -

# .PHONY: uninstall-crds
# uninstall-crds: cluster-ctx manifests kustomize ## Uninstall CRDs.
# 	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

# .PHONY: secret
# secret: cluster-ctx  ## Create secret for DoltDB.
# 	kubectl apply -f hack/manifests/doltdb-secret.yaml

# .PHONY: serviceaccount
# serviceaccount: cluster-ctx  ## Create long-lived ServiceAccount token for development.
# 	$(KUBECTL) apply -f ./hack/manifests/storageclass.yaml

# .PHONY: serviceaccount-token
# serviceaccount-get: cluster-ctx ## Get ServiceAccount token for development.
# 	$(KUBECTL) get secret dolt-operator -o jsonpath="{.data.token}" | base64 -d

# .PHONY: storageclass
# storageclass: cluster-ctx  ## Create StorageClass that allows volume expansion.
# 	$(KUBECTL) apply -f ./hack/manifests/storageclass.yaml

# METALLB_VERSION ?= "0.14.8"
# .PHONY: install-metallb
# install-metallb: cluster-ctx ## Install metallb helm chart.
# 	@METALLB_VERSION=$(METALLB_VERSION) ./hack/install_metallb.sh

# .PHONY: install-test
# install-test: cluster-ctx install-crds secret serviceaccount storageclass ## Install everything you need for local development.

# .PHONY: test-int
# test-int: install-test net manifests generate fmt vet envtest  ## Run integration tests.
# 	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./internal/controller/... | grep -v /e2e) -race -coverprofile cover.out  --timeout 1m
