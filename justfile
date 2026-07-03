# Task runner for Itinerarium. Run `just` to list recipes.
# Requires: go, node/npm, golangci-lint, and (optionally) docker.

set windows-shell := ["cmd.exe", "/c"]

# Show available recipes
default:
    @just --list

# --- API (Go, from api/) ---

# Start the API server on :8080
api:
    cd api && go run . serve

# Format Go code
api-fmt:
    cd api && gofmt -w .

# Compile all Go packages
api-build:
    cd api && go build ./...

# Run go vet
api-vet:
    cd api && go vet ./...

# Run golangci-lint (same config as CI)
api-lint:
    cd api && golangci-lint run ./...

# Run all Go tests
api-test:
    cd api && go test ./...

# Everything the API CI job checks
api-verify: api-build api-vet api-lint api-test

# --- Web (SvelteKit, from web/) ---

# Start the frontend dev server on :5173 (/api proxied to :8080)
web:
    cd web && npm run dev

# Install frontend dependencies
web-install:
    cd web && npm install

# Format frontend code
web-fmt:
    cd web && npm run format

# Run prettier + eslint checks
web-lint:
    cd web && npm run lint

# Run svelte-check (type checking)
web-check:
    cd web && npm run check

# Run frontend tests
web-test:
    cd web && npm run test

# Production build
web-build:
    cd web && npm run build

# Everything the web CI job checks
web-verify: web-lint web-check web-test web-build

# --- Whole project ---

# Format everything
fmt: api-fmt web-fmt

# Run all tests
test: api-test web-test

# Run every check CI runs — do this before finishing any feature
verify: api-verify web-verify

# Full stack via Docker Compose (API :8080, web :3000)
up:
    docker compose up --build
