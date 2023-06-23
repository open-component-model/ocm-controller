# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/open-component-model/ocm-controller
# Registry server image URL to use all building/pushing image targets
REG_IMG ?= ghcr.io/open-component-model/ocm-registry
TAG ?= latest
REG_TAG ?= latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.24.1

GOTESTSUM ?= $(LOCALBIN)/gotestsum
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

.PHONY: all
all: build

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
	$(CONTROLLER_GEN) rbac:roleName=ocm-controller-manager-role crd webhook paths="./api/..." paths="./controllers/..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..." paths="./controllers/..."

tidy:  ## Run go mod tidy
	rm -f go.sum; go mod tidy


.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

.PHONY: e2e
e2e: test-summary-tool ## Runs e2e tests
	$(GOTESTSUM) --format testname -- -count=1 -tags=e2e ./e2e

.PHONY: e2e-verbose
e2e-verbose: test-summary-tool ## Runs e2e tests in verbose
	$(GOTESTSUM) --format standard-verbose -- -count=1 -tags=e2e ./e2e

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

.PHONY: registry-server
registry-server: cd pkg/oci/registry
	go build -o bin/registry-server main.go

.PHONY: docker-registry-server
docker-registry-server:
	docker buildx build \
	-t ${REG_IMG}:${REG_TAG} \
	-f registry-server.dockerfile \
	.

.PHONY: docker-registry-server-push
docker-registry-server-push: docker-registry-server ## Push docker image with the manager.
	docker push ${REG_IMG}:${REG_TAG}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/default && $(KUSTOMIZE) edit set image open-component-model/ocm-controller=$(IMG):$(TAG) \
	&& $(KUSTOMIZE) edit add patch --path ./patches/init-container.yaml \
	&& $(KUSTOMIZE) edit set image open-component-model/ocm-registry=$(REG_IMG):$(REG_TAG)
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: dev-deploy
dev-deploy: kustomize ## Deploy controller dev image in the configured Kubernetes cluster in ~/.kube/config
	mkdir -p config/dev && cp -R config/default/* config/dev
	cd config/dev && $(KUSTOMIZE) edit set image open-component-model/ocm-controller=$(IMG):$(TAG) \
	&& $(KUSTOMIZE) edit add patch --path ./patches/init-container.yaml \
	&& $(KUSTOMIZE) edit set image open-component-model/ocm-registry=$(REG_IMG):$(REG_TAG)
	$(KUSTOMIZE) build config/dev | kubectl apply -f -
	rm -rf config/dev

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies



## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v3.8.7
CONTROLLER_TOOLS_VERSION ?= v0.9.0

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: test-summary-tool
test-summary-tool: ## Download gotestsum locally if necessary.
	GOBIN=$(LOCALBIN) go install gotest.tools/gotestsum@${TAG}

.PHONY: generate-license
generate-license:
	for f in $(shell find . -name "*.go" -o -name "*.sh"); do \
		reuse addheader -r \
			--copyright="SAP SE or an SAP affiliate company and Open Component Model contributors." \
			--license="Apache-2.0" \
			$$f \
			--skip-unrecognised; \
	done
