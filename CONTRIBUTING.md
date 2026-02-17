# Contributing to Go-SNMPSIM

Thank you for your interest in contributing to Go-SNMPSIM! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Coding Standards](#coding-standards)

## Code of Conduct

This project adheres to professional standards of conduct:

- Be respectful and constructive
- Welcome newcomers and help them learn
- Focus on what is best for the community
- Show empathy towards other community members

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/go-snmpsim.git
   cd go-snmpsim
   ```
3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/debashish/go-snmpsim.git
   ```

## Development Setup

### Prerequisites

- Go 1.21 or higher
- Make (optional, for convenience)
- Docker & Docker Compose (for container testing)

### Install Dependencies

```bash
go mod download
go mod tidy
```

### Build the Project

```bash
# Using Make
make build

# Or directly with Go
go build -o snmpsim ./cmd/snmpsim
```

### Run Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run with verbose output
go test -v ./...
```

## Making Changes

### Branch Naming

Create a descriptive branch for your work:

- `feature/add-snmpv3-support` - New features
- `fix/memory-leak-agent` - Bug fixes
- `docs/update-readme` - Documentation updates
- `refactor/optimize-lookup` - Code refactoring
- `test/add-engine-tests` - Test additions

### Commit Messages

Follow conventional commit format:

```
type(scope): short description

Longer explanation if needed, wrapped at 72 characters.

Fixes #123
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**
```
feat(agent): add SNMP v3 USM authentication support

Implements User-based Security Model for SNMP v3 with
authentication and privacy protocols.

Fixes #42
```

## Testing

### Unit Tests

Add unit tests for new functionality:

```go
// internal/store/database_test.go
func TestOIDDatabase_Insert(t *testing.T) {
    db := NewOIDDatabase()
    db.Insert("1.3.6.1.2.1.1.1.0", &OIDValue{
        Type: gosnmp.OctetString,
        Value: "Test",
    })
    
    val := db.Get("1.3.6.1.2.1.1.1.0")
    if val == nil {
        t.Fatal("Expected value, got nil")
    }
}
```

### Integration Tests

Test the full simulator:

```bash
# Start simulator
./snmpsim -port-start=25000 -port-end=25001 -devices=1 &
PID=$!

# Run SNMP queries
snmpget -v2c -c public localhost:25000 1.3.6.1.2.1.1.1.0

# Cleanup
kill $PID
```

### Benchmarks

Add benchmarks for performance-critical code:

```go
func BenchmarkOIDDatabase_Get(b *testing.B) {
    db := setupTestDatabase()
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        db.Get("1.3.6.1.2.1.1.1.0")
    }
}
```

## Submitting Changes

### Before Submitting

1. **Update documentation** if you changed functionality
2. **Add tests** for new features or bug fixes
3. **Run tests** locally: `go test ./...`
4. **Format code**: `go fmt ./...`
5. **Run linter**: `go vet ./...` or `golangci-lint run`
6. **Update CHANGELOG** if applicable

### Pull Request Process

1. **Push your branch** to your fork:
   ```bash
   git push origin feature/your-feature
   ```

2. **Create Pull Request** on GitHub with:
   - Clear title describing the change
   - Description of what changed and why
   - Reference to related issues (e.g., "Fixes #123")
   - Screenshots/logs if relevant

3. **Respond to feedback** from reviewers

4. **Update your PR** if requested:
   ```bash
   git add .
   git commit -m "Address review feedback"
   git push origin feature/your-feature
   ```

### PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix (non-breaking change fixing an issue)
- [ ] New feature (non-breaking change adding functionality)
- [ ] Breaking change (fix or feature causing existing functionality to break)
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing performed

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review performed
- [ ] Comments added for complex code
- [ ] Documentation updated
- [ ] No new warnings generated
- [ ] Tests added that prove fix/feature works
- [ ] Dependent changes merged

## Related Issues
Fixes #(issue number)
```

##Coding Standards

### Go Style

Follow standard Go conventions:

- Use `gofmt` for formatting
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

### Code Organization

```
internal/
├── engine/    # Network layer - UDP listeners, dispatching
├── agent/     # Device logic - Virtual agent implementation
└── store/     # Data layer - OID storage, indexing, loading
```

### Naming Conventions

- **Packages**: lowercase, single word (e.g., `engine`, `store`)
- **Exported types**: PascalCase (e.g., `VirtualAgent`, `OIDDatabase`)
- **Private types**: camelCase (e.g., `oidValue`, `deviceCache`)
- **Interfaces**: `-er` suffix when appropriate (e.g., `Handler`, `Parser`)

### Documentation

- Add godoc comments for all exported types and functions:
  ```go
  // VirtualAgent represents a simulated SNMP device that responds
  // to SNMP queries on a specific UDP port.
  type VirtualAgent struct {
      // ...
  }
  ```

- Include examples for complex functionality:
  ```go
  // Example usage:
  //   agent := NewVirtualAgent(1, 20000, "Device-1", oidDB)
  //   response := agent.HandlePacket(snmpRequest)
  ```

### Error Handling

- Return errors, don't panic (except in unrecoverable situations)
- Wrap errors with context: `fmt.Errorf("failed to load OID: %w", err)`
- Log errors appropriately using the `log` package

### Performance

- Use `sync.Pool` for frequently allocated objects
- Minimize allocations in hot paths
- Profile before optimizing: `go test -bench=. -cpuprofile=cpu.prof`

## Project Structure

```
go-snmpsim/
├── cmd/snmpsim/        # Main entry point
├── internal/           # Internal packages
│   ├── engine/        # Network layer
│   ├── agent/         # Agent logic
│   └── store/         # Data management
├── docs/              # Documentation
├── examples/          # Example configurations & data
├── scripts/           # Utility scripts
├── build/             # Build artifacts (gitignored)
├── go.mod             # Go module definition
├── Makefile           # Build automation
├── README.md          # Project overview
├── LICENSE            # MIT License
└── CONTRIBUTING.md    # This file
```

## Questions?

- Open an issue for bug reports or feature requests
- Start a discussion for questions or ideas
- Check existing issues and PRs before creating new ones

## Thank You!

Your contributions make this project better for everyone. Thank you for taking the time to contribute!
