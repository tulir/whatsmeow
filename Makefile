# Use default shell BASH.
SHELL_PATH := /bin/bash
SHELL := /usr/bin/env bash

# ==============================================================================
# Define dependencies

NAME            := whatsmeow
GOBIN           := $$HOME/go/bin
STATICCHECK     := $(GOBIN)/staticcheck
GOVULNCHECK     := $(GOBIN)/govulncheck
PROTO_DIR       := proto

# ==============================================================================
# Defining all make targets

.DEFAULT_GOAL := all

.PHONY: all
all: update fmt lint vulncheck protos tidy test

.PHONY: fmt
fmt:
	@echo "-- Formatting Go files --"
	gofmt -w -s . && \
	goimports -local go.mau.fi/whatsmeow -w .

.PHONY: lint
lint:
	@echo "-- Lint check for Go files --"
	go mod tidy
	# golangci-lint run
	# CGO_ENABLED=0 go vet ./...

.PHONY: staticcheck
staticcheck:
	$(STATICCHECK) -checks=all ./...

.PHONY: vulncheck
vulncheck:
	$(GOVULNCHECK) ./...

.PHONY: update
update:
	go get -u ./...

.PHONY: tidy
tidy:
	@echo "-- Tidying Go modules --"
	go mod tidy

.PHONY: clean
clean: clean-protos
	@echo " -- Cleaning Go files --"
	@go clean -cache -testcache -modcache
	@rm $(GO_DIR)/*.zip

.PHONY: clean-protos
clean-protos:
	@echo "-- Cleaning generated Go files --"
	@find $(PROTO_DIR) -name "*.pb.*" -type f -exec rm -f {} +

.PHONY: protos
protos: clean-protos
	@echo "-- Using PROTO_DIR: $(PROTO_DIR) --"
	@cd $(PROTO_DIR) && protoc --go_out=. --go-grpc_out=. --go_opt=paths=source_relative */*.proto

.PHONY: build
build:
	@echo "-- Building binaries --"
	go build -v ./...

.PHONY: bench
bench: build
	@echo "-- Running benchmark tests --"
	CGO_ENABLED=1 go test -race -count=1 ./... && \
	CGO_ENABLED=0 go test -count=1 -v ./...

.PHONY:
test: build
	@echo "-- Running tests --"
	CGO_ENABLED=1 go test ./...
