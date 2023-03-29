# Copyright 2023 TriggerMesh Inc.
# SPDX-License-Identifier: Apache-2.0

KREPO      = scoby
KREPO_DESC = Scoby Controller

BASE_DIR          ?= $(CURDIR)
OUTPUT_DIR        ?= $(BASE_DIR)/_output

# Rely on ko for building/publishing images and generating/deploying manifests
KO                ?= ko
KOFLAGS           ?=
IMAGE_TAG         ?= $(shell git rev-parse HEAD)

# Dynamically generate the list of commands based on the directory name cited in the cmd directory
COMMANDS          := $(notdir $(wildcard cmd/*))

BIN_OUTPUT_DIR    ?= $(OUTPUT_DIR)
DOCS_OUTPUT_DIR   ?= $(OUTPUT_DIR)
TEST_OUTPUT_DIR   ?= $(OUTPUT_DIR)
COVER_OUTPUT_DIR  ?= $(OUTPUT_DIR)
DIST_DIR          ?= $(OUTPUT_DIR)

# Go build variables
GO                ?= go
GOFMT             ?= gofmt
GOLINT            ?= golangci-lint run --timeout 5m
GOTOOL            ?= go tool

GOMODULE           = github.com/triggermesh/scoby

GOPKGS             = ./cmd/... ./pkg/apis/... ./pkg/reconciler/...
GOPKGS_SKIP_TESTS  =

# List of packages that expect the environment to have installed
# the dependencies for running tests:
#
# ...
#
GOPKGS_TESTS_WITH_DEPENDENCIES  =

# This environment variable should be set when dependencies have been installed
# at the running instance.
WITH_DEPENDENCIES          ?=

LDFLAGS            = -w -s
LDFLAGS_STATIC     = $(LDFLAGS) -extldflags=-static

TAG_REGEX         := ^v([0-9]{1,}\.){2}[0-9]{1,}$

HAS_GOLANGCI_LINT := $(shell command -v golangci-lint;)

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.25.0

.DEFAULT_GOAL := build

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: all
all: build

# Verify lint

install-golangci-lint:
ifndef HAS_GOLANGCI_LINT
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.45.2
endif


.PHONY: generate-code
generate-code: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object object:headerFile="hack/boilerplate.go.txt" paths="./pkg/apis/..."

.PHONY: generate-manifests
generate-manifests: controller-gen ## Generate manifests from code APIs.
	$(CONTROLLER_GEN) crd \
		 output:crd:artifacts:config=./config paths="./pkg/..."
	kubectl label --overwrite -f ./config/scoby.triggermesh.io_crdregistrations.yaml --local=true -o yaml triggermesh.io/crd-install=true > ./config/300-crdregistration.yaml; \
	rm ./config/scoby.triggermesh.io_crdregistrations.yaml

.PHONY: generate
generate: generate-code generate-manifests ## Generate assets from code.

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: generate fmt vet envtest ## Run tests.
	@mkdir -p $(COVER_OUTPUT_DIR)
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile $(COVER_OUTPUT_DIR)/cover.out

.PHONY: build
build: generate vet $(COMMANDS)  ## Build all artifacts

$(COMMANDS):
	go build -ldflags "$(LDFLAGS_STATIC)" -o $(BIN_OUTPUT_DIR)/$@ ./cmd/$@

## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.11.1
ENVTEST_K8S_VERSION = 1.25.0

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: lint
lint: install-golangci-lint ## Lint source files
	$(GOLINT) $(GOPKGS)

KO_IMAGES = $(foreach cmd,$(COMMANDS),$(cmd).image)
images: $(KO_IMAGES) ## Build container images
$(KO_IMAGES): %.image:
	$(KO) publish --push=false -B --tag-only -t $(IMAGE_TAG) ./cmd/$*

.PHONY: deploy
deploy: ## Deploy Scoby to default Kubernetes cluster
	$(KO) resolve -f $(BASE_DIR)/config > $(BASE_DIR)/scoby-$(IMAGE_TAG).yaml
	$(KO) apply -f $(BASE_DIR)/scoby-$(IMAGE_TAG).yaml
	@rm $(BASE_DIR)/scoby-$(IMAGE_TAG).yaml

.PHONY: undeploy
undeploy: ## Remove Scoby from default Kubernetes cluster
	$(KO) delete -f $(BASE_DIR)/config

.PHONY: release
release: ## Publish container images and generate release manifests
	@mkdir -p $(DIST_DIR)
	$(KO) resolve -f config/ -l 'triggermesh.io/crd-install' > $(DIST_DIR)/scoby-crds.yaml
	@cp config/namespace/100-namespace.yaml $(DIST_DIR)/scoby.yaml
ifeq ($(shell echo ${IMAGE_TAG} | egrep "${TAG_REGEX}"),${IMAGE_TAG})
	$(KO) resolve $(KOFLAGS) -B -t latest -f config/ -l '!triggermesh.io/crd-install' > /dev/null
endif
	$(KO) resolve $(KOFLAGS) -B -t $(IMAGE_TAG) --tag-only -f config/ -l '!triggermesh.io/crd-install' >> $(DIST_DIR)/scoby.yaml

.PHONY: gen-apidocs
gen-apidocs: ## Generate API docs
	GOPATH="" OUTPUT_DIR=$(DOCS_OUTPUT_DIR) ./hack/gen-api-reference-docs.sh

.PHONY: clean
clean: ## Clean build artifacts
	@for bin in $(COMMANDS) ; do \
		$(RM) -v $(BIN_OUTPUT_DIR)/$$bin; \
	done
	@$(RM) -v $(DIST_DIR)/scoby-crds.yaml $(DIST_DIR)/scoby.yaml
	@$(RM) -v $(COVER_OUTPUT_DIR)/cover.out


## TODO addlicense