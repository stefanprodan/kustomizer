# Kustomizer test, build, install makefile

all: test build

tidy:
	rm -f go.sum; go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

build:
	CGO_ENABLED=0 go build -o ./bin/kustomizer ./cmd/kustomizer

.PHONY: install
install:
	go install ./cmd/kustomizer

install-dev:
	CGO_ENABLED=0 go build -o /usr/local/bin ./cmd/kustomizer

install-plugin:
	CGO_ENABLED=0 go build -o /usr/local/bin/kubectl-kustomizer ./cmd/kustomizer

ENVTEST_ARCH ?= amd64
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
ENVTEST_KUBERNETES_VERSION=latest
install-envtest: setup-envtest
	$(SETUP_ENVTEST) use $(ENVTEST_KUBERNETES_VERSION) --arch=$(ENVTEST_ARCH) --bin-dir=$(ENVTEST_ASSETS_DIR)

KUBEBUILDER_ASSETS?="$(shell $(SETUP_ENVTEST) --arch=$(ENVTEST_ARCH) use -i $(ENVTEST_KUBERNETES_VERSION) --bin-dir=$(ENVTEST_ASSETS_DIR) -p path)"
test: tidy fmt vet install-envtest
	KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test ./cmd/... -v -parallel 4 -coverprofile cover.out

test-race: tidy fmt vet install-envtest
	KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test ./... -v -race -parallel 4 -coverprofile cover.out

test-bench:
	KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test ./... -v -bench=. -run=none

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

setup-envtest:
ifeq (, $(shell which setup-envtest))
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
SETUP_ENVTEST=$(GOBIN)/setup-envtest
else
SETUP_ENVTEST=$(shell which setup-envtest)
endif

.PHONY: release-docs
release-docs:
	git checkout main && git pull; \
	README=$$(cat README.md); \
	git checkout gh-pages && git pull; \
	echo "$$README" > README.md; \
	git add README.md; \
	git commit -m "update docs"; \
	git push origin gh-pages; \
	git checkout main

DEMO_IMAGE ?= ghcr.io/stefanprodan/kustomizer-demo-app
DEMO_TAG ?= 0.0.1
publish-demo:
	kustomizer push artifact -k ./examples/demo-app oci://$(DEMO_IMAGE):$(DEMO_TAG) --sign --cosign-key ~/.cosign/cosign.key
	kustomizer tag artifact oci://$(DEMO_IMAGE):$(DEMO_TAG) latest
	kustomizer inspect artifact oci://$(DEMO_IMAGE)  --verify --key ~/.cosign/cosign.pub

dockerfile:
	echo \
FROM gcr.io/distroless/static\\n\
COPY --chmod=755 kustomizer /\\n\
CMD ["/kustomizer"] > Dockerfile.distroless
