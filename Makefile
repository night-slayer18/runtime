.PHONY: all build build-smoke test lint clean install version-list version-bump-all version-bump

APPS := grid prism pulse strata vault
# All modules in the workspace (apps + shared packages). The repo root is not a
# module, so `go test ./...` cannot be run from here; tests run per-module.
MODULES := $(shell find apps packages -maxdepth 2 -name go.mod -exec dirname {} \;)
GOFLAGS := -trimpath

all: build

build:
	@for app in $(APPS); do \
		echo "→ building runtime-$$app"; \
		(cd apps/$$app && go build $(GOFLAGS) -o ../../bin/runtime-$$app ./cmd/$$app) || exit 1; \
	done

# Build smoke test: assert every module in the workspace compiles.
build-smoke:
	@./scripts/build-smoke.sh

test:
	@for mod in $(MODULES); do \
		echo "→ testing $$mod"; \
		(cd $$mod && go test ./...) || exit 1; \
	done

lint:
	@for mod in $(MODULES); do \
		echo "→ linting $$mod"; \
		(cd $$mod && golangci-lint run ./...) || exit 1; \
	done

clean:
	@rm -rf bin/

install:
	@for app in $(APPS); do \
		echo "→ installing runtime-$$app"; \
		(cd apps/$$app && go install ./cmd/$$app) || exit 1; \
	done

# Build a single app: make app APP=prism
app:
	@cd apps/$(APP) && go build $(GOFLAGS) -o ../../bin/runtime-$(APP) ./cmd/$(APP)

# Version management (delegates to scripts/version.sh). Pass --tag/--push/etc.
# via ARGS, e.g. make version-bump-all BUMP=minor ARGS="--tag".
version-list:
	@./scripts/version.sh --list

# Coordinated/common release bump: make version-bump-all BUMP=minor
version-bump-all:
	@./scripts/version.sh --all $(BUMP) $(ARGS)

# Single-module bump: make version-bump MODULE=apps/grid BUMP=patch
version-bump:
	@./scripts/version.sh --module $(MODULE) $(BUMP) $(ARGS)
