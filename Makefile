# Image URL to use all building/pushing image targets
IMAGE_REGISTRY ?= quay.io
REGISTRY_NAMESPACE ?= cephcsi
IMAGE_TAG ?= v0.3.1
IMAGE_NAME ?= ceph-csi-operator

# Allow customization of the name prefix and/or namespace
NAME_PREFIX ?= ceph-csi-operator-
NAMESPACE ?= $(NAME_PREFIX)system
# A comma separated list of namespaces for operator to cache objects from
WATCH_NAMESPACE ?= ""

IMG ?= $(IMAGE_REGISTRY)/$(REGISTRY_NAMESPACE)/$(IMAGE_NAME):$(IMAGE_TAG)

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.29.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Define the content of the temporary top-most kustomize overlay for the
# build-installer, build-multifile-installer and deploy targets
define BUILD_INSTALLER_OVERLAY
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: $(NAMESPACE)
namePrefix: $(NAME_PREFIX)
patches:
- patch: |-
    - op: add
      path: /spec/template/spec/containers/0/env/-
      value:
        name: CSI_SERVICE_ACCOUNT_PREFIX
        value: $(NAME_PREFIX)
    - op: add
      path: /spec/template/spec/containers/0/env/-
      value:
        name: WATCH_NAMESPACE
        value: $(WATCH_NAMESPACE)
  target:
    kind: Deployment
    name: controller-manager
images:
- name: controller
  newName: ${IMG}
endef
export BUILD_INSTALLER_OVERLAY


# Define the content of the temporary top-most kustomize overlay for the
# build-csi-rbac target
define BUILD_CSI_RBAC_OVERLAY
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: $(NAMESPACE)
namePrefix: $(NAME_PREFIX)
resources:
- ../config/csi-rbac
endef
export BUILD_CSI_RBAC_OVERLAY

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

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

.PHONY: mod.check
mod.check:#check go module dependencies
	@echo 'running "go mod vendor"'
	@go mod vendor
	@echo 'running "go mod verify"'
	@go mod verify
	@echo 'checking for modified files.'
	# fail in case there are uncommitted changes
	@ git diff --quiet || (echo "files were modified: " ; git status --porcelain ; false)

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	OPERATOR_NAMESPACE="$${OPERATOR_NAMESPACE:=$(NAMESPACE)}" KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e:
	go test ./test/e2e/ -v -ginkgo.v


MARKDOWNLINT_IMAGE = docker.io/davidanson/markdownlint-cli2:v0.17.1
.PHONY: markdownlint
markdownlint:
	@$(CONTAINER_TOOL) run --platform linux/amd64 -v .\:/workdir\:z  $(MARKDOWNLINT_IMAGE) markdownlint-cli2 "**.md" "#vendor/**"  --config .markdownlint.yaml

.PHONY: markdownlint-fix
markdownlint-fix:
	@$(CONTAINER_TOOL) run --platform linux/amd64 -v .\:/workdir\:z  $(MARKDOWNLINT_IMAGE) markdownlint-cli2 "**.md" "#vendor/**"  --config .markdownlint.yaml --fix


.PHONY:	golangci-lint-fix
golangci-lint-fix: $(GOLANGCI_LINT) ## Run the golangci-lint linter and perform fixes
	@$(GOLANGCI_LINT) --config=.golangci.yml -verbose run --fix


.PHONY: lint
lint: golangci-lint markdownlint ## Run various linters
.PHONY: lint-fix
lint-fix: golangci-lint-fix markdownlint-fix## run linters and perform fixes

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	OPERATOR_NAMESPACE="$${OPERATOR_NAMESPACE:=$(NAMESPACE)}" go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p build deploy/all-in-one
	cd build && echo "$$BUILD_INSTALLER_OVERLAY" > kustomization.yaml
	cd build && $(KUSTOMIZE) edit add resource ../config/default/
	$(KUSTOMIZE) build build > deploy/all-in-one/install.yaml
	rm -rf build

.PHONY: build-helm-installer
build-helm-installer: manifests generate kustomize helmify ## Generate helm charts for the operator.
	mkdir -p build deploy
	cd build && echo "$$BUILD_INSTALLER_OVERLAY" > kustomization.yaml
	cd build && $(KUSTOMIZE) edit add resource ../config/default/
	$(KUSTOMIZE) build build | $(HELMIFY) deploy/charts/ceph-csi-operator
	rm -rf build

.PHONY: build-multifile-installer
build-multifile-installer: build-csi-rbac manifests generate kustomize
	mkdir -p build deploy/multifile
	$(KUSTOMIZE) build config/crd > deploy/multifile/crd.yaml
	cd build && echo "$$BUILD_INSTALLER_OVERLAY" > kustomization.yaml
	cd build && $(KUSTOMIZE) edit add resource ../config/rbac ../config/manager
	$(KUSTOMIZE) build build > deploy/multifile/operator.yaml
	rm -rf build

.PHONY: build-csi-rbac
build-csi-rbac:
	mkdir -p build deploy/multifile
	cd build && echo "$$BUILD_CSI_RBAC_OVERLAY" > kustomization.yaml
	$(KUSTOMIZE) build build > deploy/multifile/csi-rbac.yaml
	rm -rf build

##@ Docs
.PHONY: generate-helm-docs
generate-helm-docs: helm-docs
	$(HELM_DOCS) -c deploy/charts/ceph-csi-operator \
		-t docs/helm-charts/operator-chart.gotmpl.md \
		-t docs/helm-charts/_templates.gotmpl \
		-o ../../../docs/helm-charts/operator-chart.md

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	mkdir -p build
	cd build && echo "$$BUILD_INSTALLER_OVERLAY" > kustomization.yaml
	cd build && $(KUSTOMIZE) edit add resource ../config/default/
	$(KUSTOMIZE) build build | $(KUBECTL) apply -f -
	rm -rf build

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
HELMIFY ?= $(LOCALBIN)/helmify-$(HELMIFY_VERSION)
HELM_DOCS ?= $(LOCALBIN)/helm-docs-$(HELM_DOCS_VERSION)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.17.2
ENVTEST_VERSION ?= release-0.17
GOLANGCI_LINT_VERSION ?= v1.63.4
HELMIFY_VERSION ?= v0.4.18
HELM_DOCS_VERSION ?= v1.14.2

.PHONY: helm-docs
helm-docs: $(HELM_DOCS) ## Download helm-docs locally if necessary.
$(HELM_DOCS): $(LOCALBIN)
	$(call go-install-tool,$(HELM_DOCS),github.com/norwoodj/helm-docs/cmd/helm-docs,$(HELM_DOCS_VERSION))

.PHONY: helmify
helmify: $(HELMIFY) ## Download helmify locally if necessary.
$(HELMIFY): $(LOCALBIN)
	$(call go-install-tool,$(HELMIFY),github.com/arttor/helmify/cmd/helmify,$(HELMIFY_VERSION))

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

.PHONY: check-all-committed
check-all-committed: ## Fail in case there are uncommitted changes
	test -z "$(shell git status --short)" || (echo "files were modified: " ; git status --short ; false)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Run the golangci-lint linter
	@$(GOLANGCI_LINT) --config=.golangci.yml -verbose run
$(GOLANGCI_LINT): $(LOCALBIN) ## Download golangci-lint locally if necessary.
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef
