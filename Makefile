# pia-wg-config Makefile

# Variables
BINARY_NAME=pia-wg-config
BINARY_PATH=./$(BINARY_NAME)
MAIN_PACKAGE=.
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golint
GOVET=$(GOCMD) vet

# Build targets
.PHONY: all build clean test coverage lint fmt vet check install uninstall deps tidy help

# Default target
all: check build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH) $(MAIN_PACKAGE)
	@echo "✓ Build complete: $(BINARY_PATH)"

# Build for multiple platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)

build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)

build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	@echo "✓ Clean complete"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .
	@echo "✓ Code formatted"

# Lint code
lint:
	@echo "Linting code..."
	@if command -v golint >/dev/null 2>&1; then \
		golint ./...; \
	else \
		echo "golint not installed. Install with: go install golang.org/x/lint/golint@latest"; \
	fi

# Vet code
vet:
	@echo "Vetting code..."
	$(GOVET) ./...
	@echo "✓ Code vetted"

# Run all checks
check: fmt vet lint test
	@echo "✓ All checks passed"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) verify
	@echo "✓ Dependencies installed"

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy
	@echo "✓ Dependencies tidied"

# Install the binary to GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(LDFLAGS) $(MAIN_PACKAGE)
	@echo "✓ $(BINARY_NAME) installed"

# Uninstall the binary
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	rm -f $(GOPATH)/bin/$(BINARY_NAME)
	@echo "✓ $(BINARY_NAME) uninstalled"

# Development helpers
dev-setup: deps
	@echo "Setting up development environment..."
	@if ! command -v golint >/dev/null 2>&1; then \
		echo "Installing golint..."; \
		$(GOCMD) install golang.org/x/lint/golint@latest; \
	fi
	@echo "✓ Development environment ready"

# Quick test run with a region list
test-regions: build
	@echo "Testing regions command..."
	./$(BINARY_NAME) regions

# Test build with example (requires valid credentials)
test-build: build
	@echo "To test config generation, run:"
	@echo "  ./$(BINARY_NAME) -r uk_london YOUR_USERNAME YOUR_PASSWORD"

# Release preparation
release-check: check build-all
	@echo "✓ Release artifacts ready"
	@ls -la $(BINARY_NAME)-*

# Show version
version:
	@echo "Version: $(VERSION)"

# Show help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  build-all    - Build for all platforms"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"
	@echo "  coverage     - Run tests with coverage"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code"
	@echo "  vet          - Vet code"
	@echo "  check        - Run all checks (fmt, vet, lint, test)"
	@echo "  deps         - Install dependencies"
	@echo "  tidy         - Tidy dependencies"
	@echo "  install      - Install binary to GOPATH/bin"
	@echo "  uninstall    - Remove binary from GOPATH/bin"
	@echo "  dev-setup    - Set up development environment"
	@echo "  test-regions - Test the regions command"
	@echo "  release-check- Prepare and check release artifacts"
	@echo "  version      - Show version"
	@echo "  help         - Show this help"
