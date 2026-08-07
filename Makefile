BINARY_NAME := orgen
MODULE := github.com/schmitthub/openrouter-generate
ORGEN_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
ORGEN_REVISION ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GO ?= go
# Append to (not clobber) any inherited GOFLAGS: worktree containers set
# GOFLAGS=-buildvcs=false because Go cannot stamp linked worktrees there
# (the .git-file walk lands on the mounted main .git and exits 128).
# (make re-exports the merged value to recipes only when GOFLAGS was already
# in the environment; host builds without GOFLAGS are unchanged.)
GOFLAGS := -trimpath $(GOFLAGS)
# Dev builds leave build.Date empty; release goreleaser stamps it via
# {{.CommitDate}} in .goreleaser.yaml.
LDFLAGS := -s -w \
	-X '$(MODULE)/internal/build.Version=$(ORGEN_VERSION)' \
	-X '$(MODULE)/internal/build.Revision=$(ORGEN_REVISION)'
BIN_DIR := bin
DIST_DIR := dist

# Test runner configuration
# Use gotestsum if available for human-friendly output, fall back to go test
GOTESTSUM := $(shell command -v gotestsum 2>/dev/null)
ifdef GOTESTSUM
	# gotestsum with human-friendly format: icons, colors, package names
	TEST_CMD = gotestsum --format testdox --
	TEST_CMD_VERBOSE = gotestsum --format standard-verbose --
else
	TEST_CMD = $(GO) test
	TEST_CMD_VERBOSE = $(GO) test -v
endif

# build the orgen binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

.PHONY: test
test:
	$(TEST_CMD) ./...

.PHONY: test-verbose
test-verbose:
	$(TEST_CMD_VERBOSE) ./...

# CI mode: race detector, no cache, coverage profile (see .github/workflows/test.yml)
.PHONY: test-ci
test-ci:
	$(GO) test -race -count=1 -coverprofile=coverage.out ./...

.PHONY: cover
cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

.PHONY: lint
lint:
	golangci-lint run --config .golangci.yml

.PHONY: fmt
fmt:
	golangci-lint fmt --config .golangci.yml

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)/$(BINARY_NAME) coverage.out

# Run all pre-commit hooks against the entire repo (mirrors CI gates)
.PHONY: pre-commit
pre-commit:
	pre-commit run --all-files

# Local dry-run of the release pipeline: builds every target + archives into
# dist/ without tagging, signing, or publishing. Empties dist/ by hand instead
# of `--clean`: dist/ is a tmpfs mount point in clawker containers and
# goreleaser's os.RemoveAll of the dir itself fails with EBUSY. CI uses
# `--clean` (release-build.yml) where dist/ is a plain directory.
.PHONY: release-check
release-check:
	@mkdir -p $(DIST_DIR) && find $(DIST_DIR) -mindepth 1 -delete
	goreleaser release --snapshot --skip=sign,sbom

# Create and push an annotated tag to trigger the release workflow
# (.github/workflows/release.yml). Guards mirror the workflow's validate job:
# semver tag, clean tree, on main, main up to date with origin, and HEAD's
# Main CI run already green (the workflow re-checks all of this server-side).
# Usage: make release VERSION=v0.1.0 MESSAGE="description of release"
.PHONY: release
release:
	@if [ -z "$(VERSION)" ]; then echo "Usage: make release VERSION=v0.1.0 MESSAGE=\"...\""; exit 1; fi
	@if [ -z "$(MESSAGE)" ]; then echo "MESSAGE is required"; exit 1; fi
	@if ! echo "$(VERSION)" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9._-]+)?$$'; then echo "Invalid semver: $(VERSION)"; exit 1; fi
	@if [ -n "$$(git status --porcelain)" ]; then echo "Working tree dirty — commit or stash first"; exit 1; fi
	@if [ "$$(git branch --show-current)" != "main" ]; then echo "Not on main branch"; exit 1; fi
	@git fetch origin main
	@if [ "$$(git rev-parse HEAD)" != "$$(git rev-parse origin/main)" ]; then echo "Local main is not in sync with origin/main — push or pull first"; exit 1; fi
	@if command -v gh >/dev/null 2>&1; then \
		if ! gh run list --workflow main.yml --commit "$$(git rev-parse HEAD)" --status success --json databaseId --jq 'length' | grep -qv '^0$$'; then \
			echo "No successful Main CI run for HEAD — wait for CI before tagging"; exit 1; \
		fi; \
	else \
		echo "warning: gh not installed, skipping CI status check (release workflow enforces it)"; \
	fi
	git tag -a $(VERSION) -m "$(MESSAGE)"
	git push origin $(VERSION)
	@echo ""
	@echo "Tagged and pushed $(VERSION) — watch: gh run watch"
