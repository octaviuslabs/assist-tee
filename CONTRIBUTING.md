# Contributing to Assist TEE

Thank you for your interest in contributing to Assist TEE! This document provides guidelines and information for contributors.

## Getting Started

### Prerequisites

- Go 1.21+
- Docker 24.0+ with Docker Compose
- gVisor runtime (runsc) for production testing (Linux only)
- PostgreSQL 16+ (or use Docker Compose)

### Development Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/your-org/assist-tee.git
   cd assist-tee
   ```

2. **Build the services**
   ```bash
   make build
   ```

3. **Start development environment**
   ```bash
   make run-dev  # Uses docker-compose.dev.yml (no gVisor)
   ```

4. **Run tests**
   ```bash
   cd services/api && go test ./...
   ```

## How to Contribute

### Reporting Bugs

- Check existing issues to avoid duplicates
- Use the bug report template
- Include steps to reproduce, expected behavior, and actual behavior
- Include your environment details (OS, Docker version, Go version)

### Suggesting Features

- Open an issue with the feature request template
- Describe the use case and why it would be valuable
- Be open to discussion about implementation approaches

### Pull Requests

1. **Fork the repository** and create a branch from `main`
2. **Make your changes** following the code style guidelines below
3. **Write tests** for new functionality
4. **Update documentation** if needed
5. **Run the test suite** to ensure nothing is broken
6. **Submit a pull request** with a clear description

### Code Style

#### Go Code

- Follow standard Go conventions and `gofmt`
- Use meaningful variable and function names
- Add comments for exported functions
- Keep functions focused and reasonably sized
- Handle errors explicitly

```go
// Good
func (e *DockerExecutor) SetupEnvironment(ctx context.Context, req *models.SetupRequest) (*models.Environment, error) {
    // ...
}

// Avoid
func setup(r *models.SetupRequest) *models.Environment {
    // ...
}
```

#### TypeScript Code (Runtime)

- Use TypeScript types where possible
- Prefer `const` over `let`
- Use async/await over raw promises

### Commit Messages

- Use clear, descriptive commit messages
- Start with a verb in present tense: "Add", "Fix", "Update", "Remove"
- Keep the first line under 72 characters
- Reference issues when applicable: "Fix #123"

```
Add streaming output for execution logs

- Use io.MultiWriter to log stdout/stderr line-by-line
- Include environment and execution IDs in log context
- Improves debugging visibility during execution
```

## Project Structure

```
assist-tee/
├── services/
│   ├── api/              # Go API service
│   │   ├── cmd/api/      # Entry point
│   │   └── internal/     # Internal packages
│   └── runtime/          # Deno runtime
├── docs/                 # Documentation
├── examples/             # Example user code
├── scripts/              # Testing and utility scripts
└── docker-compose.yml    # Production configuration
```

## Testing

### Running Tests

```bash
# Unit tests
cd services/api && go test ./...

# With coverage
cd services/api && go test -cover ./...

# Verbose output
cd services/api && go test -v ./internal/handlers/...
```

### Integration Tests

```bash
# Full flow test
./scripts/test-full-flow.sh

# Security tests (requires gVisor)
./scripts/test-all-security.sh
```

### Writing Tests

- Write unit tests for new handlers and functions
- Use the mock executor for handler tests
- Test both success and error cases

## Security

Security is critical for this project. If you discover a security vulnerability:

1. **Do NOT open a public issue**
2. Email the maintainers directly with details
3. Allow time for a fix before public disclosure

When contributing:

- Never commit secrets, credentials, or API keys
- Be cautious with user input handling
- Consider sandbox escape vectors in execution code
- Review gVisor and Docker security implications

## Documentation

- Update README.md for user-facing changes
- Update relevant docs/ files for detailed documentation
- Add inline comments for complex logic
- Include examples for new features

## Questions?

- Open a discussion for general questions
- Check existing issues and discussions first
- Be patient and respectful in all interactions

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
