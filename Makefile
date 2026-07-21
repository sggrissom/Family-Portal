-include .env.mk

.PHONY: all build deploy test test-race local typecheck lint format check check-css
all: local

# ── deployment settings ────────────────────────────────────────────────────────
APP_NAME     := family
DEPLOY_HOST  := vps

# ── build settings ─────────────────────────────────────────────────────────────
BUILD_DIR    := build
BINARY_NAME  := family_site

GOOS         := linux
GOARCH       := amd64
CGO_ENABLED  := 1

build-frontend: check-css
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

build-face:
	@echo "Building family-face daemon (requires dlib on build machine)..."
	mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=1 \
	  go build -tags faceanalysis -ldflags="-s -w" \
	    -o $(BUILD_DIR)/family-face ./cmd/faceanalysis/

deploy: build
	deploy $(APP_NAME) $(DEPLOY_HOST) $(BUILD_DIR)/$(BINARY_NAME)

deploy-face: build-face
	deploy $(APP_NAME)-face $(DEPLOY_HOST) $(BUILD_DIR)/family-face internal

# Build on the server (where dlib is installed), scp back, then deploy.
FACE_SOURCE ?= ~/Family-Portal
deploy-face-remote:
	@echo "Building family-face on $(DEPLOY_HOST)..."
	ssh $(DEPLOY_HOST) "cd $(FACE_SOURCE) && \
	  CGO_ENABLED=1 go build -tags faceanalysis -ldflags='-s -w' \
	    -o /tmp/family-face ./cmd/faceanalysis/"
	mkdir -p $(BUILD_DIR)
	scp $(DEPLOY_HOST):/tmp/family-face $(BUILD_DIR)/family-face
	deploy $(APP_NAME)-face $(DEPLOY_HOST) $(BUILD_DIR)/family-face internal

test:
	go test ./backend/ -v

# boltdb v1.3.1 uses pointer conversions rejected by Go's checkptr instrumentation.
# Keep the race detector enabled while disabling only that incompatible check.
test-race:
	go test -race -gcflags=all=-d=checkptr=0 ./backend/

typecheck: check-css
	@echo "Checking TypeScript types..."
	npx tsc --noEmit

local:
	go run family/local

# ── code quality ───────────────────────────────────────────────────────────────

check-css:
	@echo "Validating CSS blocks..."
	node scripts/check-css-blocks.mjs

lint: check-css
	@echo "Running Go linters..."
	go vet -tags release $(shell go list -tags release ./... | grep -v '/cmd/')
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "The following Go files need formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@echo "Running TypeScript linter..."
	npx prettier --check "frontend/**/*.{ts,tsx,json}" --ignore-path .prettierignore

format:
	@echo "Formatting Go code..."
	go fmt ./...
	@echo "Formatting TypeScript code..."
	npx prettier --write "frontend/**/*.{ts,tsx,json}" --ignore-path .prettierignore

check: test typecheck lint
	@echo "✅ All quality checks passed!"
