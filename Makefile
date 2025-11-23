.PHONY: build docker-build docker-run release test test-coverage clean help

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the walship binary
	go build -o walship ./cmd/walship

test: ## Run all tests
	go test -v ./...

test-coverage: ## Run tests with coverage report
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean: ## Clean build artifacts and test outputs
	rm -f walship
	rm -f coverage.out coverage.html
	rm -rf dist/

# Build Docker image locally
docker-build:
	docker build -t walship .

# Run Docker container (example with dummy env vars)
docker-run:
	docker run --rm \
		-e WALSHIP_REMOTE_URL=http://localhost:8080 \
		-e WALSHIP_AUTH_KEY=test \
		walship

# Create a new tag to trigger release (usage: make release v=v0.1.0)
release:
	git tag $(v)
	git push origin $(v)
