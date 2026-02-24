# Development Tools Guide

This guide explains how to use the development tools integrated into the service-platform project: **benchstat**, **modgraphviz**, **revive**, and **mockery**.

## 🚀 Quick Start

All tools can be accessed through:
1. **Makefile targets** - Direct CLI usage
2. **Bubble Tea TUI** - Interactive menu (run `make cli`)

---

## 📊 Benchstat - Benchmark Comparison

Compare Go benchmark results between versions to detect performance regressions.

### Installation
```bash
make install-benchstat
```

### Usage

#### Run all benchmarks (default location)
```bash
make benchstat
```

#### Run benchmarks for specific package
```bash
make benchstat BENCH_PKG='./tests/unit/fun_string_test.go'
```

#### Compare benchmark results
```bash
# Generate baseline
go test -bench=. -benchmem ./tests/unit/ > baseline.txt

# Make changes to code...

# Generate new results
go test -bench=. -benchmem ./tests/unit/ > new.txt

# Compare
benchstat baseline.txt new.txt
```

### Available Benchmarks
- `tests/unit/fun_string_bench_test.go` - String operation benchmarks
  - `BenchmarkStringConcat` - String concatenation performance
  - `BenchmarkStringRepeat` - String repetition performance
  - `BenchmarkStringToLower` - Case conversion performance

### Example Output
```
name                           old time/op    new time/op    delta
BenchmarkStringConcat-8          3.25ns ± 1%   3.28ns ± 2%    ~     (p=0.424)
BenchmarkStringRepeat-8          9.76ns ± 3%   9.81ns ± 2%    ~     (p=0.595)
BenchmarkStringToLower-8        28.5ns ± 4%   28.2ns ± 2%    ~     (p=0.392)
```

---

## 📈 Modgraphviz - Module Dependency Visualization

Generate a visual graph of Go module dependencies. Supports both full codebase and package-specific analysis.

### Installation
```bash
make modgraphviz
```

### Usage

#### Full module dependency graph (default)
```bash
make modgraphviz
# Output: docs/graphs/module-graph.svg (full codebase - large)
```

#### Package-specific dependency graph
```bash
make modgraphviz PKG='github.com/gin-gonic'
make modgraphviz PKG='google.golang.org/grpc'
make modgraphviz PKG='go.mongodb.org'
# Output: docs/graphs/module-graph-pkg.svg (smaller, focused)
```

### Requirements
- `modgraphviz` (installed automatically)
- `dot` command (from Graphviz package)
  ```bash
  # Install Graphviz
  apt install graphviz      # Ubuntu/Debian
  brew install graphviz      # macOS
  ```

### Viewing the Graph
```bash
# Open in browser or image viewer
open docs/graphs/module-graph.svg    # macOS
xdg-open docs/graphs/module-graph.svg # Linux
```

### Use Cases
- Identify circular dependencies
- Understand module structure
- Detect excessive coupling
- Visualize dependency chains

---

## 🎯 Revive - Code Style Linter

A faster, more configurable linter for Go code style and best practices.

### Installation
```bash
make install-revive
```

### Usage

#### Lint all packages (default)
```bash
make revive
# On success: ✅ No lint issues found!
# On failure: ❌ Found N lint issue(s).  (exits with code 1)
```

#### Lint specific package
```bash
make revive PKG='./cmd/api'
make revive PKG='./internal/cli'
make revive PKG='./tests/unit'
```

#### Manual usage
```bash
revive -config .revive.toml ./...
revive -config .revive.toml ./cmd/api
```

### Configuration
Configuration file: `.revive.toml`

#### Key Rules
- **blank-imports** - Disallow unused imports
- **context-as-argument** - Context should not be function argument
- **cyclomatic** - Max cyclomatic complexity (15)
- **error-return** - Error should be last return
- **error-strings** - Error strings should not be capitalized
- **exported** - Exported functions should have comments
- **if-return** - Simplify if-else patterns
- **receiver-naming** - Consistent receiver names

#### Adding Custom Rules
Edit `.revive.toml` to enable/disable rules:
```toml
[rule.cyclomatic]
Arguments = [15]  # Max complexity

[rule.line-length-limit]
Arguments = [120]  # Max line length
```

### Run in CI Pipeline
```bash
# Already integrated in .github/workflows/
revive -config .revive.toml ./...
```

---

## 🤖 Mockery - Mock Generator

Automatically generate mocks for Go interfaces to facilitate unit testing.

### Installation
```bash
make install-mockery
```

### Usage

#### Generate all mocks
```bash
make mockery
```

#### Generate mocks for specific interface
```bash
make mockery PATTERN='MyInterfaceName'
```

#### Manual usage
```bash
# Generate all mocks (recursive)
mockery --all --output=mocks --recursive

# Generate for specific interface
mockery --name=MyInterface --output=mocks --recursive

# Generate with specific patterns
mockery --name='Repository*' --output=mocks --recursive
```

### Output Structure
```
mocks/
├── mock_UserRepository.go
├── mock_PaymentService.go
└── ...
```

### Using Generated Mocks in Tests
```go
package myservice_test

import (
	"testing"
	"myproject/mocks"
)

func TestUserService(t *testing.T) {
	// Create mock
	mockRepo := new(mocks.MockUserRepository)
	
	// Set expectations
	mockRepo.On("GetUser", mock.AnythingOfType("int")).Return(&User{ID: 1}, nil)
	
	// Use in test
	service := NewUserService(mockRepo)
	user, err := service.GetUser(1)
	
	// Verify
	mockRepo.AssertExpectations(t)
}
```

### Configuration
Configuration file: `.mockery.yaml`

#### Available Options
```yaml
all: true                               # Generate for all interfaces
recursive: true                         # Search recursively
with-expecter: true                     # Add expecter methods
unexported: false                       # Don't include unexported
mockname: "{{.InterfaceName}}"          # Mock name pattern
filename: "mock_{{.InterfaceName}}.go"  # File pattern
```

---

## 🎛️ Integration with CI/CD

These tools are integrated into your GitHub Actions workflows:

### PR Checks
- **revive** linting is required before merge
- Mocks should be up-to-date
- Benchmarks help detect performance regressions

### Running Locally
```bash
# Run all dev tools
make revive
make benchstat
make mockery
make modgraphviz
```

---

## 📚 Interactive CLI Usage

Access all tools through the Bubble Tea TUI:

```bash
make cli
```

Then navigate to:
```
Code Quality → Choose dev tool
  ├── Revive Linter
  ├── Run Benchmarks
  ├── Generate Mocks
  └── Module Graph
```

---

## 🔧 Troubleshooting

### Benchstat not found
```bash
make install-benchstat
# Verify installation
benchstat -version
```

### Revive: No `.revive.toml` found
```bash
# Automatically creates default config
make revive
```

### Mockery: Mocks not generated
```bash
# Ensure interfaces are exported (public)
# Check mockery.yaml configuration
make mockery --help
```

### Modgraphviz: Cannot generate SVG
```bash
# Install Graphviz dot command
apt install graphviz  # Ubuntu
brew install graphviz # macOS
```

### For CI/CD failures
Check `.github/workflows/pr-check.yml` and `.github/workflows/ci.yml` for configuration.

---

## 📖 Additional Resources

- [benchstat Documentation](https://golang.org/x/perf/cmd/benchstat)
- [Revive GitHub](https://github.com/mgechev/revive)
- [Revive Rules](https://revive.run/rules/)
- [Mockery GitHub](https://github.com/vektra/mockery)
- [Go Testing Best Practices](https://golang.org/doc/effective_go#names)

---

## 🎯 Best Practices

### Benchmarking
1. Create benchmarks for performance-critical code
2. Compare before and after optimization
3. Run multiple times: `go test -bench=. -count=5 ./...`
4. Use benchstat for statistical analysis

### Linting
1. Fix linting errors early in development
2. Use revive alongside golangci-lint
3. Customize rules in `.revive.toml` for your project
4. Run before committing: `make revive`
5. For focused linting: `make revive PKG='./cmd/api'` - lint only specific package
6. Use package-specific analysis during development for faster feedback

### Mocking
1. Create mocks for external dependencies
2. Use mocks to test error conditions
3. Keep mocks in `./mocks/` directory
4. Regenerate when interfaces change: `make mockery`

### Dependency Management
1. Review full module graph regularly: `make modgraphviz`
2. View package-specific dependencies: `make modgraphviz PKG='github.com/gin-gonic'`
3. Watch for circular dependencies
4. Keep dependencies up-to-date
5. Use modgraphviz before major refactors to understand impact
