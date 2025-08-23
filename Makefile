.PHONY: help build test clean dev deploy setup

# Default target
help:
	@echo "Available commands:"
	@echo "  setup   - Setup the project (install dependencies, etc.)"
	@echo "  build   - Build the project"
	@echo "  test    - Run tests"
	@echo "  clean   - Clean build artifacts"
	@echo "  dev     - Start development server"
	@echo "  deploy  - Deploy to Cloudflare Workers"
	@echo "  logs    - View Cloudflare Workers logs"

# Setup the project
setup:
	@echo "Setting up the project..."
	go mod tidy
	@echo "Setup complete!"

# Build the project
build:
	@echo "Building the project..."
	go build -o bin/worker worker.go
	@echo "Build complete!"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...
	@echo "Tests complete!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf dist/
	@echo "Clean complete!"

# Start development server
dev:
	@echo "Starting development server..."
	wrangler dev

# Deploy to Cloudflare Workers
deploy:
	@echo "Deploying to Cloudflare Workers..."
	wrangler deploy

# View Cloudflare Workers logs
logs:
	@echo "Viewing Cloudflare Workers logs..."
	wrangler tail

# Create KV namespace
kv-create:
	@echo "Creating KV namespace..."
	wrangler kv:namespace create "CONFIG"
	wrangler kv:namespace create "CONFIG" --preview

# Set secrets
secrets:
	@echo "Setting up secrets..."
	@echo "Please run the following commands:"
	@echo "  wrangler secret put MISSKEY_API_KEY"
	@echo "  wrangler secret put MISSKEY_INSTANCE"
	@echo "  wrangler secret put POST_TEMPLATE"
	@echo "  wrangler secret put VISIBILITY"
	@echo "  wrangler secret put SWARM_WEBHOOK_SECRET"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run

# Run all checks
check: fmt lint test
	@echo "All checks passed!"
