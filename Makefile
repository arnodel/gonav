.PHONY: help backend frontend build clean

help: ## Show this help message
	@echo "Available commands:"
	@echo ""
	@echo "  make backend   - Start the Go backend server (port 8080)"
	@echo "  make frontend  - Start the frontend development server (port 1234)"
	@echo "  make build     - Build the Go binary"
	@echo "  make clean     - Clean built artifacts"
	@echo ""
	@echo "For development:"
	@echo "  Terminal 1: make backend"
	@echo "  Terminal 2: make frontend"
	@echo ""

backend: ## Start the Go backend server
	@echo "Starting Go backend server on port 8080..."
	go run main.go

frontend: ## Start the frontend development server
	@echo "Starting frontend development server on port 1234..."
	cd frontend && npm run dev

build: ## Build the Go binary
	@echo "Building Go binary..."
	go build -o gonav .

clean: ## Clean built artifacts
	@echo "Cleaning built artifacts..."
	rm -f gonav
	cd frontend && rm -rf dist .parcel-cache