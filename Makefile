APP=ym-bot
BIN_DIR=bin

.PHONY: run build lint

run:
	@echo "Running $(APP)..."
	@go run ./cmd/bot

build:
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(APP) ./cmd/bot

lint:
	@go vet ./...

