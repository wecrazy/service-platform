## Description
<!-- Provide a brief description of the changes in this PR -->

## Services Affected
<!-- Check which services are impacted by this change -->

- [ ] API (REST endpoints)
- [ ] gRPC (gRPC services)
- [ ] WhatsApp
- [ ] Scheduler
- [ ] Telegram
- [ ] Monitoring
- [ ] N8N
- [ ] Twilio WhatsApp
- [ ] Migrations/Database
- [ ] Configuration
- [ ] CI/CD/Infrastructure
- [ ] Other (specify below)

## Type of Change
<!-- Mark the relevant option with an "x" -->

- [ ] 🐛 Bug fix (non-breaking change which fixes an issue)
- [ ] ✨ New feature (non-breaking change which adds functionality)
- [ ] 💥 Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] 📝 Documentation update
- [ ] ♻️ Code refactoring
- [ ] ⚡ Performance improvement
- [ ] ✅ Test update
- [ ] 🔧 Configuration change
- [ ] 🏗️ Infrastructure/Build change
- [ ] 📡 Proto definition change

## Proto Changes (if applicable)
<!-- If you modified any .proto files -->

- [ ] No proto changes
- [ ] Proto files modified
  - [ ] `proto/whatsapp.proto`
  - [ ] `proto/grpc.proto`
  - [ ] `proto/auth.proto`
  - [ ] `proto/scheduler.proto`
  - [ ] `proto/telegram.proto`
  - [ ] `proto/twilio_whatsapp.proto`
  - [ ] Other: ___________

- [ ] Ran `make proto-gen` to regenerate proto files
- [ ] Created migration for schema changes (if applicable)

## Database Changes (if applicable)
<!-- If this PR involves database schema or migration changes -->

- [ ] No database changes
- [ ] Database migrations added
  - Migration file(s): ___________
  - [ ] Tested migration: `make migrate-up`
  - [ ] Tested rollback: `make migrate-down`
- [ ] Schema changes documented in [DATABASE_MIGRATIONS.md](docs/DATABASE_MIGRATIONS.md)

## Configuration Changes (if applicable)
<!-- If this PR modifies config files or system settings -->

- [ ] No configuration changes
- [ ] Configuration changes made to:
  - [ ] `service-platform.dev.yaml`
  - [ ] `service-platform.prod.yaml`
  - [ ] `manage-service.dev.yaml`
  - [ ] `manage-service.prod.yaml`
  - [ ] Scripts in `scripts/`
  - [ ] Other: ___________
- [ ] Tested config with: `make config-dev SERVICE=service-platform`

## Related Issue
<!-- Link to the issue this PR addresses -->
Fixes #(issue)

## Changes Made
<!-- List the specific changes made in this PR -->

- 
- 
- 

## Testing
<!-- Describe the testing you've done -->

- [ ] Unit tests pass: `go test ./tests/unit/...`
- [ ] Integration tests pass: `go test ./tests/...`
- [ ] Manual testing completed
- [ ] New tests added for new functionality

**Test Execution for Affected Services:**
- [ ] API tests: `go test ./tests/api/...`
- [ ] gRPC tests: `go test ./tests/grpc/...`
- [ ] WhatsApp tests: `go test ./tests/whatsapp/...`
- [ ] Scheduler tests: `go test ./tests/scheduler/...`
- [ ] Telegram tests: `go test ./tests/telegram/...`

**Test Coverage:**
- [ ] Coverage maintained or improved (target: ≥50%)

## Local Testing Checklist
<!-- For developers testing locally -->

- [ ] Code builds successfully: `make build`
- [ ] Linting passes: `golangci-lint run`
- [ ] All services start without errors
- [ ] Logs are clean (run `make start-monitoring` if needed)
- [ ] Configuration loads correctly for dev environment

## Code Quality
<!-- Mark completed items with an "x" -->

- [ ] My code follows the project's style guidelines
- [ ] I have performed a self-review of my own code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] My changes generate no new warnings or errors
- [ ] No code formatting issues (code is properly formatted)
- [ ] Any commented-out code has been removed or documented as TODOs

## Documentation
<!-- Mark completed items with an "x" -->

- [ ] Updated README if behavior changed
- [ ] Updated relevant documentation in `docs/`
- [ ] Updated CHANGELOG.md (if applicable)
- [ ] APIs/endpoints documented
- [ ] Configuration options documented
- [ ] No breaking changes without migration guide

## Security Considerations
<!-- If this PR has security implications, describe them here -->

- [ ] No security concerns
- [ ] Security implications assessed
  - [ ] Authentication/Authorization changes
  - [ ] Input validation changes
  - [ ] Data exposure risks reviewed
  - [ ] Secrets not committed
- [ ] Security implications described below:

## Performance Impact
<!-- Describe any performance implications -->

- [ ] No performance impact
- [ ] Performance impact assessed and acceptable
- [ ] Database queries optimized (if applicable)
- [ ] Response times analyzed (if applicable)

## Deployment Notes
<!-- Any special deployment considerations? -->

- Database migrations required: Yes / No
  - If yes, rollback procedure: ___________
- Configuration changes required: Yes / No
  - If yes, which service(s): ___________
- Service restart required: Yes / No
- Zero-downtime deployment possible: Yes / No

## Docker Changes (if applicable)
<!-- If you modified Dockerfiles or docker-compose -->

- [ ] No Docker changes
- [ ] Modified Dockerfile(s):
  - [ ] `docker/Dockerfile.api`
  - [ ] `docker/Dockerfile.grpc`
  - [ ] `docker/Dockerfile.whatsapp`
  - [ ] `docker/Dockerfile.scheduler`
  - [ ] Other: ___________
- [ ] Tested: `docker build -f docker/Dockerfile.api .`

## Backwards Compatibility
<!-- Ensure changes don't break existing functionality -->

- [ ] Fully backwards compatible
- [ ] Breaking changes documented with migration path
- [ ] API versioning considered (if applicable)
- [ ] Client library updates needed (if applicable)

## Additional Notes
<!-- Any additional information that reviewers should know -->

