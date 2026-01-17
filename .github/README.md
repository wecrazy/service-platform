# CI/CD & Security Setup

This directory contains GitHub Actions workflows and CI/CD configuration for the service-platform project.

## 🚀 Workflows

### 1. **CI Pipeline** (`ci.yml`)
Runs on every push and PR to main/develop branches.

**Jobs:**
- **Lint**: Code quality checks with golangci-lint
- **Test**: Unit and integration tests with PostgreSQL, MongoDB, Redis
- **Build**: Compile all service binaries (api, grpc, whatsapp, scheduler, monitoring, n8n)
- **Security Scan**: Gosec and Trivy vulnerability scanning
- **Docker Build**: Build and scan Docker images

### 2. **PR Checks** (`pr-check.yml`)
Validates pull requests before merging.

**Checks:**
- PR title format (conventional commits)
- Code formatting (gofmt)
- Breaking changes detection
- Test coverage threshold (50%)
- Dependency review
- Build verification

### 3. **Security Scanning** (`security.yml`)
Comprehensive security scanning (runs daily + on-demand).

**Scans:**
- **CodeQL**: Semantic code analysis for Go and JavaScript
- **Gosec**: Go security scanner
- **Trivy**: Vulnerability and secret scanner
- **Nancy**: Go dependency vulnerability checker
- **TruffleHog**: Secret detection
- **License Check**: Compliance verification
- **Docker Security**: Container image scanning with Dockle

### 4. **Release** (`release.yml`)
Automated release process for tagged versions.

**Actions:**
- Build multi-platform binaries (Linux, macOS, Windows)
- Generate SHA256 checksums
- Create GitHub releases with changelog
- Build and push Docker images to GHCR
- Multi-architecture support (amd64, arm64)

### 5. **CodeQL Advanced** (`codeql.yml`)
Deep semantic analysis (weekly + on-demand).

## 🔒 Security Features

### Enabled Scans
- ✅ Code vulnerabilities (CodeQL, Gosec)
- ✅ Dependency vulnerabilities (Trivy, Nancy, Dependabot)
- ✅ Secret detection (TruffleHog)
- ✅ License compliance
- ✅ Docker image scanning
- ✅ SARIF integration with GitHub Security tab

### Dependabot Configuration
Auto-updates for:
- Go dependencies (weekly on Mondays)
- GitHub Actions (weekly)
- Docker images (weekly)

## 📋 Requirements

### Repository Secrets
No secrets required for basic CI/CD. Optional:
- `CODECOV_TOKEN`: For coverage reports

### Repository Settings
Enable in Settings > Security:
- ✅ Dependabot alerts
- ✅ Dependabot security updates
- ✅ Code scanning alerts
- ✅ Secret scanning

### Branch Protection (Recommended)
For `main` branch:
- Require PR reviews (1-2 reviewers)
- Require status checks: `lint`, `test`, `build`, `security-scan`
- Require branches to be up to date
- Require conversation resolution

## 🏗️ Docker Images

Built for these services:
- `ghcr.io/wecrazy/service-platform-api`
- `ghcr.io/wecrazy/service-platform-grpc`
- `ghcr.io/wecrazy/service-platform-whatsapp`
- `ghcr.io/wecrazy/service-platform-scheduler`

## 🎯 Usage

### Running Tests Locally
```bash
# Run all tests
make test

# Run specific tests
make test-mongo
```

### Building Locally
```bash
# Build all services
make build

# Build specific service
make build-api
```

### Linting
```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run
```

### Manual Security Scan
```bash
# Install tools
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install github.com/sonatype-nexus-community/nancy@latest

# Run scans
gosec ./...
go list -json -deps ./... | nancy sleuth
```

## 🔄 Release Process

1. Update version and changelog
2. Create and push tag:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```
3. Release workflow runs automatically
4. Docker images published to GHCR
5. Binaries attached to GitHub release

## 📊 Monitoring

Check workflow status:
- [Actions tab](https://github.com/wecrazy/service-platform/actions)
- [Security tab](https://github.com/wecrazy/service-platform/security)
- [Dependabot](https://github.com/wecrazy/service-platform/security/dependabot)

## 🐛 Troubleshooting

### Workflow Fails
- Check [Actions logs](https://github.com/wecrazy/service-platform/actions)
- Verify all tests pass locally
- Ensure dependencies are up to date

### Security Alerts
- Review in [Security tab](https://github.com/wecrazy/service-platform/security)
- Update vulnerable dependencies
- Check Dependabot PRs

### Docker Build Fails
- Verify Dockerfiles exist in `docker/` directory
- Check Go version matches across Dockerfiles and workflows
- Test build locally: `docker build -f docker/Dockerfile.api .`

## 📚 Additional Resources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [golangci-lint](https://golangci-lint.run/)
- [Gosec](https://github.com/securego/gosec)
- [Trivy](https://aquasecurity.github.io/trivy/)
- [CodeQL](https://codeql.github.com/)
