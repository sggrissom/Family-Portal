-include .env.mk

.PHONY: all build deploy stop_service start_service copy_files test local typecheck lint format setup-hooks check
all: local

# ── deployment settings ────────────────────────────────────────────────────────
NAS_USER     ?= user
NAS_IP       ?= 192.168.1.100
NAS_DEST     ?= /tmp/deploy
NAS_SERVICE  ?= family_portal_service

# ── build settings ─────────────────────────────────────────────────────────────
BUILD_DIR    := build
BINARY_NAME  := family_site

GOOS         := linux
GOARCH       := amd64
CGO_ENABLED  := 0

build-frontend:
	@echo "Building frontend..."
	go run -tags frontend release/frontend.go

build-go:
	@echo "Building $(BINARY_NAME)..."
	# make sure build dir exists
	mkdir -p $(BUILD_DIR)
	# (b) compile release.go into a self‑contained Linux binary
	cd release && GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
	  go build -tags release -ldflags="-s -w" \
	    -o ../$(BUILD_DIR)/$(BINARY_NAME) release.go

build: build-frontend build-go

stop_service:
	@echo "Stopping $(NAS_SERVICE) on NAS..."
	ssh $(NAS_USER)@$(NAS_IP) "./stop_service.sh"

copy_files:
	@echo "Uploading binary..."
	scp -O $(BUILD_DIR)/$(BINARY_NAME) $(NAS_USER)@$(NAS_IP):$(NAS_DEST)

start_service:
	@echo "Starting $(NAS_SERVICE) on NAS..."
	ssh $(NAS_USER)@$(NAS_IP) "./start_service.sh"

deploy: build stop_service copy_files start_service
	@echo "✅ Deployment complete."

test:
	go test ./backend/ -v

typecheck:
	@echo "Checking TypeScript types..."
	npx tsc --noEmit

local:
	go run family/local

# ── code quality ───────────────────────────────────────────────────────────────

lint:
	@echo "Running Go linters..."
	go vet -tags release ./...
	go fmt ./...
	@echo "Running TypeScript linter..."
	npx prettier --check "frontend/**/*.{ts,tsx,json}" --ignore-path .prettierignore

format:
	@echo "Formatting Go code..."
	go fmt ./...
	@echo "Formatting TypeScript code..."
	npx prettier --write "frontend/**/*.{ts,tsx,json}" --ignore-path .prettierignore

setup-hooks:
	@echo "Setting up pre-commit hooks..."
	@command -v pre-commit >/dev/null 2>&1 || { echo "Installing pre-commit..."; pip install pre-commit; }
	pre-commit install
	@echo "Pre-commit hooks installed. Run 'make check' to validate."

check: test typecheck lint
	@echo "✅ All quality checks passed!"
