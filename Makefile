
# Image URL to use all building/pushing image targets
CONTROLLER_IMG ?= quay.io/csiaddons/k8s-controller
SIDECAR_IMG ?= quay.io/csiaddons/k8s-sidecar
BUNDLE_IMG ?= quay.io/csiaddons/k8s-bundle

# set TAG to a release for consumption in the bundle
TAG ?= latest

# version of the package inside the bundle, can be different from the TAG
BUNDLE_VERSION ?= 0.0.1

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.23

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
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./cmd/... ./controllers/... ./sidecar/..." output:crd:artifacts:config=config/crd/bases

.PHONY: bundle
bundle: kustomize operator-sdk
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(CONTROLLER_IMG):$(TAG)
	$(KUSTOMIZE) build config/default | $(OPERATOR_SDK) generate bundle --manifests --metadata --package=csi-addons --version=$(BUNDLE_VERSION)
	mkdir -p ./bundle/tests/scorecard && $(KUSTOMIZE) build config/scorecard --output=./bundle/tests/scorecard/config.yaml

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

.PHONY: check-all-committed
check-all-committed: ## Fail in case there are uncommitted changes
	test -z "$(shell git status --short)" || (echo "files were modified: " ; git status --short ; false)

.PHONY: bundle-validate
bundle-validate: IMAGE_BUILDER ?= $(shell which podman docker | head -n1 | xargs basename)
bundle-validate: operator-sdk
	$(OPERATOR_SDK) bundle validate --image-builder=$(IMAGE_BUILDER) ./bundle

##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/manager/main.go
	go build -o bin/csi-addons ./cmd/csi-addons

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/manager/main.go

.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	docker build -t ${CONTROLLER_IMG}:${TAG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${CONTROLLER_IMG}:${TAG}

.PHONY: docker-build-sidecar
docker-build-sidecar:
	docker build -f ./build/Containerfile.sidecar -t ${SIDECAR_IMG}:${TAG} .

.PHONY: docker-push-sidecar
docker-push-sidecar:
	docker push ${SIDECAR_IMG}:${TAG}

.PHONY: docker-build-bundle
docker-build-bundle: bundle
	docker build -f ./bundle.Dockerfile -t ${BUNDLE_IMG}:${TAG} .

.PHONY: docker-push-bundle
docker-push-bundle:
	docker push ${BUNDLE_IMG}:${TAG}

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
	cd config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLLER_IMG}:${TAG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

# controller-gen gets installed from the vendor/ directory.
CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
.PHONY: controller-gen
controller-gen:
	go build -o $(CONTROLLER_GEN) ./vendor/$(shell grep controller-gen tools.go | sed 's/.*_ "//;s/"//')

# kustomize gets installed from the vendor/ directory. The tools.go file is
# used to select the major version of kustomize.
KUSTOMIZE = $(shell pwd)/bin/kustomize
.PHONY: kustomize
kustomize:
	go build -o $(KUSTOMIZE) ./vendor/$(shell grep kustomize tools.go | sed 's/.*_ "//;s/"//')

# setup-envtest gets installed from the vendor/ directory.
ENVTEST = $(shell pwd)/bin/setup-envtest
.PHONY: envtest
envtest:
	go build -o $(ENVTEST) ./vendor/$(shell grep setup-envtest tools.go | sed 's/.*_ "//;s/"//')

# operator-sdk gets installed from the vendor/ directory.
OPERATOR_SDK = $(shell pwd)/bin/operator-sdk
.PHONY: operator-sdk
operator-sdk:
	go build -o $(OPERATOR_SDK) ./vendor/$(shell grep operator-sdk tools.go | sed 's/.*_ "//;s/"//')

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
