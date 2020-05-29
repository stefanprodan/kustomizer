VERSION?=$(shell grep 'VERSION' cmd/kustomizer/main.go | awk '{ print $$4 }' | tr -d '"')

all: test build

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

test: tidy fmt vet
	go test ./... -coverprofile cover.out

build:
	CGO_ENABLED=0 go build -o ./bin/kustomizer ./cmd/kustomizer

install:
	go install cmd/kustomizer

install-dev:
	CGO_ENABLED=0 go build -o /usr/local/bin ./cmd/kustomizer

install-plugin:
	CGO_ENABLED=0 go build -o /usr/local/bin/kubectl-kustomizer ./cmd/kustomizer

release:
	git tag "v$(VERSION)"
	git push origin "v$(VERSION)"
