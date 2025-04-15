# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/open-component-model/ocm-controller
TAG ?= v0.11.0
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
GITHUBORG                                      ?= open-component-model
OCMREPO                                        ?= ghcr.io/$(GITHUBORG)/ocm-controller
ENVTEST_K8S_VERSION = 1.24.1
OCM_CLI_VERSION ?= 0.15.0
REPO_ROOT                                      := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
PLATFORM_OS                                    := $(shell go env GOOS)
PLATFORM_ARCH                                  := $(shell go env GOARCH)
GOTESTSUM ?= $(LOCALBIN)/gotestsum
MKCERT ?= $(LOCALBIN)/mkcert
UNAME ?= $(shell uname|tr '[:upper:]' '[:lower:]')
GEN := $(REPO_ROOT)/gen

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

GO_WASM_BUILD = tinygo build -target=wasi -panic=trap -scheduler=none -no-debug -o

.PHONY: all
all: build

CREDS ?=
OCM ?= $(LOCALBIN)/ocm $(CREDS)

.PHONY: ocm
ocm: $(OCM)
$(OCM): $(LOCALBIN)
	curl -L "https://github.com/open-component-model/ocm/releases/download/v$(OCM_CLI_VERSION)/ocm-$(OCM_CLI_VERSION)-$(PLATFORM_OS)-$(PLATFORM_ARCH).tar.gz" | tar -xz -C $(LOCALBIN)

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
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
	$(CONTROLLER_GEN) rbac:roleName=ocm-controller-manager-role crd webhook paths="./api/..." paths="./controllers/..." output:crd:artifacts:config=deploy/crds output:rbac:artifacts:config=deploy/templates

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object paths="./api/..." paths="./controllers/..."

tidy:  ## Run go mod tidy
	rm -f go.sum; go mod tidy

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint.
	$(GOLANGCI_LINT) run --timeout 10m

##@ Testing

# Note: Requires tinygo (0.31.2) and wasm-opt
.PHONY: test
test: manifests generate fmt vet build-wasm-testdata envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

.PHONY: e2e
e2e: prime-test-cluster test-summary-tool ## Runs e2e tests
	$(GOTESTSUM) --format testname -- -count=1 -tags=e2e ./e2e

.PHONY: e2e-verbose
e2e-verbose: prime-test-cluster test-summary-tool ## Runs e2e tests in verbose
	$(GOTESTSUM) --format standard-verbose -- -count=1 -tags=e2e ./e2e

.PHONY: prime-test-cluster
prime-test-cluster: mkcert
	./hack/prime_test_cluster.sh

.PHONY: build-wasm-testdata
build-wasm-testdata:
	${GO_WASM_BUILD} ./internal/wasm/hostfuncs/resource/testdata/get_resource_bytes.wasm  ./internal/wasm/hostfuncs/resource/testdata/get_resource_bytes.go
	${GO_WASM_BUILD} ./internal/wasm/hostfuncs/resource/testdata/get_resource_labels.wasm  ./internal/wasm/hostfuncs/resource/testdata/get_resource_labels.go
	${GO_WASM_BUILD} ./internal/wasm/hostfuncs/resource/testdata/get_resource_url.wasm  ./internal/wasm/hostfuncs/resource/testdata/get_resource_url.go
##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -t ${IMG}:${TAG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}:${TAG}

##@ Component

.PHONY: plain-push
plain-push: ocm
	$(MAKE) -C component ctf
	$(OCM) transfer ctf -f $(GEN)/component/ctf $(OCMREPO)

##@ Documentation

api-docs: gen-crd-api-reference-docs  ## Generate API reference documentation
	$(GEN_CRD_API_REFERENCE_DOCS) -api-dir=./api/v1alpha1 -config=./hack/api-docs/config.json -template-dir=./hack/api-docs/template -out-file=./docs/api/v1alpha1/ocm.md

##@ Build Dependencies

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
ENVTEST ?= $(LOCALBIN)/setup-envtest
GEN_CRD_API_REFERENCE_DOCS ?= $(LOCALBIN)/gen-crd-api-reference-docs

## Tool Versions
KUSTOMIZE_VERSION ?= v3.8.7
CONTROLLER_TOOLS_VERSION ?= v0.14.0
GEN_API_REF_DOCS_VERSION ?= e327d0730470cbd61b06300f81c5fcf91c23c113
MKCERT_VERSION ?= v1.4.4
GOLANGCI_LINT_VERSION ?= v2.1.1

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN)

.PHONY: mkcert
mkcert: $(MKCERT)
$(MKCERT): $(LOCALBIN)
	curl -L "https://github.com/FiloSottile/mkcert/releases/download/$(MKCERT_VERSION)/mkcert-$(MKCERT_VERSION)-$(UNAME)-amd64" -o $(LOCALBIN)/mkcert
	chmod +x $(LOCALBIN)/mkcert

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT): $(LOCALBIN)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s $(GOLANGCI_LINT_VERSION)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

# Find or download gen-crd-api-reference-docs
.PHONY: gen-crd-api-reference-docs
gen-crd-api-reference-docs: $(GEN_CRD_API_REFERENCE_DOCS)
$(GEN_CRD_API_REFERENCE_DOCS): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/ahmetb/gen-crd-api-reference-docs@$(GEN_API_REF_DOCS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: test-summary-tool
test-summary-tool: ## Download gotestsum locally if necessary.
	GOBIN=$(LOCALBIN) go install gotest.tools/gotestsum@latest

.PHONY: generate-license
generate-license:
	for f in $(shell find . -name "*.go" -o -name "*.sh"); do \
		reuse addheader -r \
			--copyright="SAP SE or an SAP affiliate company and Open Component Model contributors." \
			--license="Apache-2.0" \
			$$f \
			--skip-unrecognised; \
	done
