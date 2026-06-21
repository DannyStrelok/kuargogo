# Detect operating system
ifeq ($(OS),Windows_NT)
    # On Windows cmd.exe
    LOOKUP_LINTER = $(shell where golangci-lint 2>NUL)
    LOOKUP_GCC = $(shell where gcc 2>NUL)
    LOOKUP_TRIVY = $(shell where trivy 2>NUL)
else
    # On POSIX shell
    LOOKUP_LINTER = $(shell command -v golangci-lint 2>/dev/null)
    LOOKUP_GCC = $(shell command -v gcc 2>/dev/null)
    LOOKUP_TRIVY = $(shell command -v trivy 2>/dev/null)
endif

.PHONY: audit lint test security build

audit: lint test security

lint:
ifneq ($(LOOKUP_LINTER),)
	@echo "🔍 Running linter..."
	golangci-lint run ./...
else
	@echo "⚠️  golangci-lint not found. Skipping."
endif

test:
	@echo "🧪 Running tests..."
ifneq ($(LOOKUP_GCC),)
	@echo "   (Race detector enabled via GCC)"
	go test -race -v ./...
else
	@echo "⚠️  GCC not found. Skipping race detector (-race)."
	go test -v ./...
endif

security:
ifneq ($(LOOKUP_TRIVY),)
	@echo "🛡️  Running security scan..."
	trivy fs . --severity CRITICAL,HIGH --ignore-unfixed --pkg-types os,library
else
	@echo "⚠️  trivy not found. Skipping."
endif

build:
	@echo "🔨 Building kuargogo CLI..."
	go build -o kgg.exe ./cmd/kgg
