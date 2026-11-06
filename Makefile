.PHONY: build test generate verify install fmt vet tidy clean docs docs-schema docs-validate

PROVIDER_NAME := terraform-provider-bahriya
VERSION       := dev
GOOS          := $(shell go env GOOS)
GOARCH        := $(shell go env GOARCH)
INSTALL_DIR   := $(HOME)/.terraform.d/plugins/registry.terraform.io/bahriya/bahriya/$(VERSION)/$(GOOS)_$(GOARCH)

build:
	go build -o bin/$(PROVIDER_NAME) ./cmd/$(PROVIDER_NAME)

# ./... includes cmd/generator, whose test checks internal/resources is current.
test:
	go test ./...

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

SPEC_DIR ?= ../packages/bahriya-openapi/specs/v1

# The generator reads the embedded resource-schema.json, so copy it in first.
generate:
	cp $(SPEC_DIR)/../resource-schema.json internal/bmap/resource-schema.json
	go run ./cmd/generator -out internal/resources

verify: generate
	@if ! git diff --exit-code -- internal/resources internal/datasources internal/bmap/resource-schema.json 2>/dev/null; then \
		echo "ERROR: generated files are out of sync. Run 'make generate' and commit."; \
		exit 1; \
	fi

# docs renders docs/ from templates/ and examples/ with tfplugindocs (pinned as a
# go tool). The main package lives in cmd/, which tfplugindocs cannot build, so
# the schema is exported through a dev override and handed to it; tfplugindocs
# looks the provider up under its default registry key.
DOCS_TMP := $(shell mktemp -d 2>/dev/null || echo /tmp/tfdocs)
docs-schema: build
	printf 'provider_installation {\n  dev_overrides { "bahriya-cloud/bahriya" = "%s" }\n  direct {}\n}\n' "$(CURDIR)/bin" > $(DOCS_TMP)/rc
	mkdir -p $(DOCS_TMP)/ws
	printf 'terraform {\n  required_providers {\n    bahriya = { source = "bahriya-cloud/bahriya" }\n  }\n}\n' > $(DOCS_TMP)/ws/main.tf
	cd $(DOCS_TMP)/ws && TF_CLI_CONFIG_FILE=$(DOCS_TMP)/rc terraform providers schema -json > $(DOCS_TMP)/schema.json
	jq '.provider_schemas |= with_entries(.key = "registry.terraform.io/hashicorp/bahriya")' $(DOCS_TMP)/schema.json > bin/docs-schema.json

docs: docs-schema
	go tool tfplugindocs generate --provider-name bahriya --rendered-provider-name Bahriya --providers-schema bin/docs-schema.json

# docs-validate checks every example with `terraform validate` and the rendered
# docs with tfplugindocs.
docs-validate: docs-schema
	go test ./internal/provider -run 'TestDocsExamplesValidate|TestHCLFixturesValidate' -count=1
	go tool tfplugindocs validate --provider-name bahriya --providers-schema bin/docs-schema.json

install: build
	mkdir -p $(INSTALL_DIR)
	cp bin/$(PROVIDER_NAME) $(INSTALL_DIR)/

clean:
	rm -rf bin/
