# Constants.
VERSION_FILE=version.txt
BINARY_NAME=cds
REPOSITORY_DIR=config/repository
OUTPUT_DIR=target
BUILD_DIR=$(OUTPUT_DIR)/.build
DIST_DIR=$(OUTPUT_DIR)/dist
DISTRIBUTION_SRC=distribution

# Platform targets for the release distribution, as <os>/<arch> pairs. Every
# dependency is pure Go, so these all cross-compile with CGO_ENABLED=0.
PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Product name, as registered with the WSO2 update service. Used for the packed
# binary (<PACK>/bin/$(PRODUCT_NAME)), the pack directory and its zip.
PRODUCT_NAME ?= wso2cds

# Variable constants.
VERSION=$(shell cat $(VERSION_FILE))
# Pack names carry no leading "v", so strip it.
PACK_VERSION=$(VERSION:v%=%)

# Tools
PROJECT_DIR := $(realpath $(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
PROJECT_BIN_DIR := $(PROJECT_DIR)/bin
TOOL_BIN ?= $(PROJECT_BIN_DIR)/tools
GOLANGCI_LINT ?= $(TOOL_BIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.64.8

$(TOOL_BIN):
	mkdir -p $(TOOL_BIN)

# Default target.
all: clean lint build integration-test

# Clean up build artifacts.
clean:
	rm -rf $(OUTPUT_DIR)

# Compile the binary for the host platform. Packaging is `dist`, which produces
# the release packs; this target exists for a fast compile check and for the
# container image build, which copies $(BUILD_DIR)/$(BINARY_NAME).
build: _build

lint: golangci-lint
	cd . && $(GOLANGCI_LINT) run ./...

# Run the integration suite against PostgreSQL, which needs Docker.
integration-test:
ifdef test
	TESTCONTAINERS_RYUK_DISABLED=true go test -v ./test/integration -run $(test)
else
	TESTCONTAINERS_RYUK_DISABLED=true go test -v ./test/integration
endif

# Run the same integration suite against the inbuilt database, which needs no Docker.
integration-test-sqlite:
ifdef test
	CDS_TEST_DB=sqlite go test -v ./test/integration -run $(test)
else
	CDS_TEST_DB=sqlite go test -v ./test/integration
endif

# Run the unit tests.
unit-test:
	go test ./internal/... ./dbscripts/...

mq-integration-test:
ifdef test
	TESTCONTAINERS_RYUK_DISABLED=true go test -v ./test/activemq_integration/... -run $(test)
else
	TESTCONTAINERS_RYUK_DISABLED=true go test -v ./test/activemq_integration/...
endif

# Build the Go project.
_build:
	mkdir -p $(BUILD_DIR) && \
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

# Build the platform-specific release distributions.
dist: clean-dist
	@for platform in $(PLATFORMS); do \
		$(MAKE) --no-print-directory _dist_one DIST_OS=$${platform%/*} DIST_ARCH=$${platform#*/} || exit 1; \
	done

# Stage and zip a single platform pack. Not meant to be called directly.
#
# Pack layout:
#   bin/$(PRODUCT_NAME)[.exe]      the server binary
#   repository/conf/               deployment.yaml and friends
#   repository/database/           where the inbuilt database is created, see DefaultSQLitePath
#   repository/dbscripts/          DDL for operators running an external database
#   logs/                          created empty so the server can write on first start
#   LICENSE.txt, README.md
_dist_one:
	@set -e; \
	os=$(DIST_OS); arch=$(DIST_ARCH); \
	bin=$(PRODUCT_NAME); \
	if [ "$$os" = "windows" ]; then bin=$(PRODUCT_NAME).exe; fi; \
	pack=$(PRODUCT_NAME)-$$os-$$arch-$(PACK_VERSION); \
	stage=$(DIST_DIR)/$$pack; \
	echo "Building $$pack..."; \
	rm -rf $$stage; \
	mkdir -p $$stage/bin $$stage/logs $$stage/repository/database $$stage/repository/dbscripts; \
	CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o $$stage/bin/$$bin ./cmd/server; \
	cp -r $(REPOSITORY_DIR)/. $$stage/repository/; \
	cp dbscripts/*.sql $$stage/repository/dbscripts/; \
	cp LICENSE.txt README.md $$stage/; \
	find $$stage -name '.DS_Store' -delete; \
	cd $(DIST_DIR) && COPYFILE_DISABLE=1 zip -rq $$pack.zip $$pack -x '**/.DS_Store' -x '**/__MACOSX/**' && rm -rf $$pack; \
	shasum -a 256 $$pack.zip > $$pack.zip.sha256; \
	if command -v md5sum >/dev/null 2>&1; then md5sum $$pack.zip > $$pack.zip.md5; else md5 -r $$pack.zip > $$pack.zip.md5; fi; \
	echo "  -> $(DIST_DIR)/$$pack.zip"

# Remove the staged distributions and their zips.
clean-dist:
	rm -rf $(DIST_DIR)

help:
	@echo "Makefile targets:"
	@echo "  all                        - Clean, build, and test the project."
	@echo "  clean                      - Remove build artifacts."
	@echo "  build                      - Compile the binary for the host platform."
	@echo "  dist                       - Build the platform-specific release zips into $(DIST_DIR)."
	@echo "  clean-dist                 - Remove the staged distributions and their zips."
	@echo "  integration-test           - Run integration tests against PostgreSQL (use test=TestName to filter)."
	@echo "  integration-test-sqlite    - Run integration tests against the inbuilt database (use test=TestName to filter)."
	@echo "  mq-integration-test        - Run message queue integration tests (use test=TestName to filter)."
	@echo "  unit-test                  - Run unit tests."
	@echo "  lint                       - Run golangci-lint."
	@echo "  help                       - Show this help message."

.PHONY: all clean build dist clean-dist _dist_one lint help integration-test integration-test-sqlite mq-integration-test unit-test

.PHONY: go_install_tool golangci-lint

define go_install_tool
	cd /tmp && \
	GOBIN=$(TOOL_BIN) go install $(2)@$(3)
endef

golangci-lint: $(GOLANGCI_LINT)

$(GOLANGCI_LINT): $(TOOL_BIN)
	$(call go_install_tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))
