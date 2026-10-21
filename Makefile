# Constants.
VERSION_FILE=version.txt
BINARY_NAME=cds
REPOSITORY_DIR=config/repository
OUTPUT_DIR=target
BUILD_DIR=$(OUTPUT_DIR)/.build
DIST_DIR=$(OUTPUT_DIR)/dist
DISTRIBUTION_SRC=distribution

# Platform targets for the release distribution, as <os>/<arch> pairs. Only
# Linux is supported: those are the platforms CDS is released and maintained
# on. Every dependency is pure Go, so both cross-compile with CGO_ENABLED=0
# and no per-platform build host is needed.
PLATFORMS ?= linux/amd64 linux/arm64

# Product name as registered with the update service. It names the packed
# binary (<PACK>/bin/$(PRODUCT_NAME)), the pack directory, its zip and the
# product identity in <PACK>/updates/product.txt.
PRODUCT_NAME ?= wso2cds

# Variable constants.
VERSION=$(shell cat $(VERSION_FILE))
# ZIP_FILE_NAME=${BINARY_NAME_PREFIX}-$(VERSION)
PRODUCT_FOLDER=$(BINARY_NAME)-$(VERSION)
# Pack and product names carry no leading "v", so strip it.
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

# Build project and package it.
build: _build _package

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
#
# The race detector runs here because the process shares one connection pool
# across every request and every worker.
unit-test:
	go test -race ./internal/... ./dbscripts/...

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
#
# One source revision produces every supported pack, so the executables in a
# release — and in the updates published from it — are all built from the same
# code.
dist: clean-dist
	@for platform in $(PLATFORMS); do \
		$(MAKE) --no-print-directory _dist_one DIST_OS=$${platform%/*} DIST_ARCH=$${platform#*/} || exit 1; \
	done
	@$(MAKE) --no-print-directory _dist_metadata

# Stage and zip one platform pack. Not meant to be called directly.
#
# Pack layout:
#   bin/$(PRODUCT_NAME)              the lifecycle command and the runtime
#   repository/conf/                 deployment.yaml and compatibility.yaml
#   repository/database/             where the inbuilt database is created
#   repository/dbscripts/            DDL for operators running an external database
#   repository/resources/security/   where the operator installs the TLS material
#   logs/                            created empty so the runtime can write on first start
#   updates/product.txt              the update-service product identity of this pack
#   version.txt, LICENSE.txt, README.md
#
# bin/update_tool_setup.sh is not staged here. It is published by the Updates
# team rather than built from this source, so it is attached to the released
# packs instead - see $(DISTRIBUTION_SRC)/u2-onboarding/.
_dist_one:
	@set -e; \
	os=$(DIST_OS); arch=$(DIST_ARCH); \
	pack=$(PRODUCT_NAME)-$$os-$$arch-$(PACK_VERSION); \
	stage=$(DIST_DIR)/$$pack; \
	echo "Building $$pack..."; \
	rm -rf $$stage; \
	mkdir -p $$stage/bin $$stage/logs $$stage/updates \
		$$stage/repository/database $$stage/repository/dbscripts $$stage/repository/resources/security; \
	CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o $$stage/bin/$(PRODUCT_NAME) ./cmd/wso2cds; \
	cp -r $(REPOSITORY_DIR)/. $$stage/repository/; \
	$(DISTRIBUTION_SRC)/pack-config.sh $$stage/repository/conf/deployment.yaml; \
	cp dbscripts/*.sql $$stage/repository/dbscripts/; \
	cp $(DISTRIBUTION_SRC)/pack/security-README.md $$stage/repository/resources/security/README.md; \
	cp LICENSE.txt README.md $(VERSION_FILE) $$stage/; \
	printf '%s\n' "$$pack" > $$stage/updates/product.txt; \
	find $$stage -name '.DS_Store' -delete; \
	cd $(DIST_DIR) && COPYFILE_DISABLE=1 zip -rq $$pack.zip $$pack -x '**/.DS_Store' -x '**/__MACOSX/**'; \
	rm -rf $$pack; \
	shasum -a 256 $$pack.zip > $$pack.zip.sha256; \
	if command -v md5sum >/dev/null 2>&1; then md5sum $$pack.zip > $$pack.zip.md5; else md5 -r $$pack.zip > $$pack.zip.md5; fi; \
	echo "  -> $(DIST_DIR)/$$pack.zip"

# Publish the release compatibility metadata beside the packs, so installation
# tooling can resolve the right artifact for a target environment without
# unpacking one.
_dist_metadata:
	@PRODUCT_NAME=$(PRODUCT_NAME) PACK_VERSION=$(PACK_VERSION) PLATFORMS="$(PLATFORMS)" \
		$(DISTRIBUTION_SRC)/release-metadata.sh \
		$(REPOSITORY_DIR)/conf/compatibility.yaml $(DIST_DIR)

# Remove the staged distributions and their zips.
clean-dist:
	rm -rf $(DIST_DIR)

# Package the binary and repository directory into a zip file.
_package:
	mkdir -p $(OUTPUT_DIR)/$(PRODUCT_FOLDER) && \
	cp $(BUILD_DIR)/$(BINARY_NAME) $(OUTPUT_DIR)/$(PRODUCT_FOLDER)/ && \
	cp -r $(REPOSITORY_DIR) $(OUTPUT_DIR)/$(PRODUCT_FOLDER)/ && \
	cp $(VERSION_FILE) $(OUTPUT_DIR)/$(PRODUCT_FOLDER)/ && \
	cp -r dbscripts $(OUTPUT_DIR)/$(PRODUCT_FOLDER)/ && \
	cd $(OUTPUT_DIR) && zip -r $(PRODUCT_FOLDER).zip $(PRODUCT_FOLDER) && \
	rm -rf $(PRODUCT_FOLDER) && \
	rm -rf $(BUILD_DIR)

help:
	@echo "Makefile targets:"
	@echo "  all                        - Clean, build, and test the project."
	@echo "  clean                      - Remove build artifacts."
	@echo "  build                      - Build the Go project."
	@echo "  dist                       - Build the Linux release packs into $(DIST_DIR)."
	@echo "  clean-dist                 - Remove the staged distributions and their zips."
	@echo "  integration-test           - Run integration tests against PostgreSQL (use test=TestName to filter)."
	@echo "  integration-test-sqlite    - Run integration tests against the inbuilt database (use test=TestName to filter)."
	@echo "  mq-integration-test        - Run message queue integration tests (use test=TestName to filter)."
	@echo "  unit-test                  - Run unit tests with the race detector."
	@echo "  lint                       - Run golangci-lint."
	@echo "  help                       - Show this help message."

.PHONY: all clean build dist clean-dist _dist_one _dist_metadata lint help integration-test integration-test-sqlite mq-integration-test unit-test

.PHONY: go_install_tool golangci-lint

define go_install_tool
	cd /tmp && \
	GOBIN=$(TOOL_BIN) go install $(2)@$(3)
endef

golangci-lint: $(GOLANGCI_LINT)

$(GOLANGCI_LINT): $(TOOL_BIN)
	$(call go_install_tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))
