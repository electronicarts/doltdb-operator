# Format and lint the Helm charts before submitting a merge request.
reviewable: generate-docs update-dependencies lint check-policy

# Generate README.md documentation for all Helm charts.
generate-docs:
	@echo '== BEGIN GENERATE DOCS =='
	@../hack/helm/generate_docs.sh
	@echo '== END GENERATE DOCS =='

# Update all Chart.lock files and archives for local chart dependencies.
update-dependencies:
	@echo '== BEGIN UPDATE DEPENDENCIES =='
	@../hack/helm/update_dependencies.sh
	@echo '== END UPDATE DEPENDENCIES =='

# Verify that README.md documentation was generated for all Helm charts.
verify-docs:
	@echo '== BEGIN VERIFY DOCS =='
	@../hack/helm/hack/verify_docs.sh
	@echo '== END VERIFY DOCS =='

# Lint all Helm charts.
lint: update-dependencies
	@echo '== BEGIN LINT =='
	@../hack/helm/lint.sh
	@echo '== END LINT =='

# Validate all Helm charts by applying custom OPA policies to the templated Kubernetes YAML.
check-policy: update-dependencies
	@echo '== BEGIN CHECK POLICY =='
	@../hack/helm/check_policy.sh
	@echo '== END CHECK POLICY =='

# Validate all Helm charts by applying custom OPA policies to the templated Kubernetes YAML.
# Strict mode treats warnings as errors.
check-policy-strict: update-dependencies
	@echo '== BEGIN CHECK POLICY STRICT =='
	@CONFTEST_ARGS=--fail-on-warn ../hack/helm/check_policy.sh
	@echo '== END CHECK POLICY STRICT =='
