# FileBox build system. Produces a single binary with the embedded frontend.

BACKEND_DIR  := backend
FRONTEND_DIR := frontend
BIN_DIR      := bin
BIN          := $(BIN_DIR)/filebox
STATIC_DIR   := $(BACKEND_DIR)/internal/web/static
LDFLAGS      := -s -w

.DEFAULT_GOAL := build
.PHONY: all deps frontend embed-frontend backend build run dev-backend swagger clean fmt

all: build

## deps: install Go dependencies only (no npm)
deps:
	cd $(BACKEND_DIR) && go mod download

## tools: install the swag CLI (needed by `make swagger`)
tools:
	go install github.com/swaggo/swag/cmd/swag@latest

## frontend: copy the static vanilla JS source into the Go embed directory
frontend:
	@mkdir -p $(STATIC_DIR)
	rm -rf $(STATIC_DIR)/*
	cp -r $(FRONTEND_DIR)/static/. $(STATIC_DIR)/

## embed-frontend: alias for frontend
embed-frontend: frontend

## backend: compile the Go binary (backend only, no frontend embed)
backend:
	@mkdir -p $(BIN_DIR)
	cd $(BACKEND_DIR) && CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o ../$(BIN) ./cmd/filebox

## build: build the single binary with the embedded frontend
build: frontend backend
	@echo "✓ Built $(BIN) (single binary, frontend embedded)"

## run: run the backend from the project root (loads backend/.env, storage/ at root)
run: build
	./$(BIN)

## dev-backend: run the Go backend via go run (faster iteration, no embed)
dev-backend:
	cd $(BACKEND_DIR) && go run ./cmd/filebox

## swagger: regenerate Swagger docs from handler annotations
swagger:
	cd $(BACKEND_DIR) && swag init -g cmd/filebox/main.go -o docs --parseInternal

## fmt: format Go sources
fmt:
	cd $(BACKEND_DIR) && go fmt ./...

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR)
	rm -rf $(STATIC_DIR)/*
	@touch $(STATIC_DIR)/.gitkeep
