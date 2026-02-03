APP       := ym-bot
BIN_DIR   := bin
MAIN_PKG  := ./cmd/bot
GO        := go

.PHONY: all build run deps tidy test lint clean build-linux help

all: deps build

# Download module dependencies.
deps:
	$(GO) mod download

# Tidy go.mod and go.sum.
tidy:
	$(GO) mod tidy

# Build binary into $(BIN_DIR).
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(APP) $(MAIN_PKG)

# Run the bot (for local development).
run:
	$(GO) run $(MAIN_PKG)

# Run tests.
test:
	$(GO) test -race -count=1 ./...

# Run tests with coverage.
test-cover:
	$(GO) test -race -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Vet and (if installed) run golangci-lint.
lint:
	$(GO) vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || true

# Remove build artifacts.
clean:
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html

# Linux binary for Docker / production (no CGO).
build-linux:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o $(BIN_DIR)/$(APP)-linux-amd64 $(MAIN_PKG)

help:
	@echo "Targets:"
	@echo "  all         - deps + build (default)"
	@echo "  build       - build binary to $(BIN_DIR)/$(APP)"
	@echo "  run         - run bot locally"
	@echo "  deps        - go mod download"
	@echo "  tidy        - go mod tidy"
	@echo "  test        - run tests"
	@echo "  test-cover  - run tests with coverage report"
	@echo "  lint        - go vet and optionally golangci-lint"
	@echo "  clean       - remove $(BIN_DIR) and coverage files"
	@echo "  build-linux - build Linux amd64 binary for Docker"
	@echo "  help        - this help"
