.PHONY: build test generate verify install fmt vet tidy clean

PROVIDER_NAME := terraform-provider-bahriya
VERSION       := dev
GOOS          := $(shell go env GOOS)
GOARCH        := $(shell go env GOARCH)
INSTALL_DIR   := $(HOME)/.terraform.d/plugins/registry.terraform.io/bahriya/bahriya/$(VERSION)/$(GOOS)_$(GOARCH)

build:
	go build -o bin/$(PROVIDER_NAME) ./cmd/$(PROVIDER_NAME)

test:
	go test ./internal/...

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

# The 0.4.x line is generated from bahriya/openapi commit 7dd8aaf, the spec
# v0.4.0 was built from (regenerating from it reproduces v0.4.0 exactly). A
# later spec changes unrelated attributes (maxcpu, project users, maxversions,
# …), so pass SPEC_DIR pointing at an export of that commit:
#   git -C ../packages/bahriya-openapi archive 7dd8aaf specs | tar -x -C /tmp/openapi-7dd8aaf
#   make generate SPEC_DIR=/tmp/openapi-7dd8aaf/specs/v1
SPEC_DIR ?= ../packages/bahriya-openapi/specs/v1

generate:
	go run ./cmd/generator -spec $(SPEC_DIR) -out internal/resources

verify: generate
	@if ! git diff --exit-code -- internal/resources internal/datasources 2>/dev/null; then \
		echo "ERROR: generated files are out of sync. Run 'make generate' and commit."; \
		exit 1; \
	fi

install: build
	mkdir -p $(INSTALL_DIR)
	cp bin/$(PROVIDER_NAME) $(INSTALL_DIR)/

clean:
	rm -rf bin/
