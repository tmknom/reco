# Include: minimum
-include .makefiles/minimum/Makefile
.makefiles/minimum/Makefile:
	@git clone https://github.com/tmknom/makefiles.git .makefiles >/dev/null 2>&1

# Variables: Go
REPO_NAME := reco
MODULE := $(shell go list -m)
DIST_DIR := dist
# --version は、リリースで GoReleaser が埋め込む値と同じく、タグの先頭の v を除いた形で表示する
VERSION = $(patsubst v%,%,$(shell \git describe --tags --always --dirty))
LDFLAGS ?= "-X main.version=$(VERSION)"

# Targets: Go
.PHONY: all
all: fmt goimports tidy vet test cover build run ## format, tidy, vet, test, cover and build
	@echo "NEXT STEP: Run 'make vuln' to check known vulnerabilities."
	@echo "NEXT STEP: Run 'make lint' to run golangci-lint in a container."

.PHONY: build
build: ## build executable binary
	go build -ldflags=$(LDFLAGS) -o $(DIST_DIR)/$(REPO_NAME) .

.PHONY: install
install: ## install executable binary
	go install -ldflags=$(LDFLAGS) .

.PHONY: run
run: build ## run command
	$(DIST_DIR)/$(REPO_NAME) --help

.PHONY: fmt
fmt: ## format code
	go fmt ./...

.PHONY: goimports
goimports: ## update import lines
	go tool goimports -w -local $(MODULE) .

.PHONY: tidy
tidy: ## manage modules
	go mod tidy
	go mod verify

.PHONY: vet
vet: ## static analysis by vet
	go vet ./...

# Docker を使うため all には含めない
.PHONY: lint
lint: ## static analysis by golangci-lint in an isolated container
	LINT_MODULE=$(CURDIR) LINT_GOMODCACHE="$$(go env GOMODCACHE)" \
		docker compose -f scripts/golangci-lint/compose.yaml run --rm -T --interactive=false lint

.PHONY: vuln
vuln: ## check known vulnerabilities by govulncheck
	go tool govulncheck ./...

.PHONY: test
test: ## run test
	go test -count=1 ./...

.PHONY: cover
cover: ## measure test coverage excluding main.go and require 100% under internal/
	mkdir -p $(DIST_DIR)
	go test -count=1 -coverprofile=$(DIST_DIR)/coverage.out ./...
	scripts/coverage/check.sh $(DIST_DIR)/coverage.out

.PHONY: clean
clean: ## remove build artifacts
	rm -rf $(DIST_DIR)

# Targets: Release
# 共有 Makefile の release/* は workflow_dispatch でリリースを起動する前提のため、reco では使わない
.PHONY: release
release: ## choose the bump level interactively and push the next release tag
	@scripts/release/tag.sh

.PHONY: test/scripts
test/scripts: ## run tests of shell scripts
	scripts/release/tag_test.sh
