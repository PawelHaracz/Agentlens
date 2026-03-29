# Contributing to AgentLens

Thank you for your interest in contributing! This guide covers how to get started.

## Development Setup

**Prerequisites:** Go 1.23+, Node.js 20+, SQLite

```bash
git clone https://github.com/PawelHaracz/agentlens
cd agentlens

# Build backend
go build ./...

# Run tests
go test ./...

# Build frontend
cd web && npm install && npm run build && cd ..

# Run locally
go run ./cmd/agentlens
```

The server starts on http://localhost:8080.

## Project Structure

```
cmd/agentlens/      # Main entrypoint
internal/
  api/              # HTTP handlers, router, middleware
  config/           # Configuration loading
  discovery/        # Agent discovery (K8s, static, A2A, MCP)
  health/           # Health checker
  model/            # Core data types
  server/           # HTTP server lifecycle
  store/            # SQLite persistence
web/                # React frontend (TypeScript + Vite + Tailwind)
deploy/helm/        # Helm chart
examples/           # Docker Compose + mock agents
docs/               # API documentation
```

## Making Changes

1. Fork the repository and create a feature branch.
2. Write tests for new functionality.
3. Run `go test ./...` to ensure tests pass.
4. Run `go vet ./...` and `go build ./...`.
5. If changing the frontend, run `cd web && npm run build`.
6. Open a pull request with a clear description of the change.

## Code Style

- Go: follow standard Go conventions (`gofmt`, `go vet`).
- TypeScript: strict mode, no unused variables.
- Keep functions small and focused.
- Comment public APIs.

## Reporting Issues

Please open a GitHub issue with:
- A clear description of the problem
- Steps to reproduce
- Expected vs actual behavior
- Go/Node version and OS

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
