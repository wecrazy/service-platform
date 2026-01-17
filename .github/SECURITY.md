# Security Policy

## Supported Versions

We release security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of service-platform seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### How to Report a Security Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via one of the following methods:

1. **GitHub Security Advisories** (Preferred)
   - Go to the [Security tab](https://github.com/wecrazy/service-platform/security/advisories)
   - Click "Report a vulnerability"
   - Fill in the details

2. **Email**
   - Send an email to: security@wecrazy.com
   - Include as much information as possible (see below)

### Information to Include

Please include the following information in your report:

- Type of issue (e.g., buffer overflow, SQL injection, cross-site scripting, etc.)
- Full paths of source file(s) related to the manifestation of the issue
- The location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

### What to Expect

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Vulnerability Assessment**: Within 14 days
- **Fix Timeline**: Depends on severity and complexity

### Disclosure Policy

- We will coordinate with you on the timing of public disclosure
- We will credit you in the security advisory (unless you prefer to remain anonymous)
- We follow coordinated disclosure practices
- Typical embargo period: 90 days from initial report

## Security Best Practices

When using service-platform:

### Configuration
- Always use strong passwords and rotate them regularly
- Enable TLS/SSL for all external communications
- Use environment variables for sensitive configuration
- Never commit secrets to the repository

### Deployment
- Keep all dependencies up to date
- Use the latest stable release
- Run services with minimal privileges
- Enable audit logging
- Implement rate limiting
- Use a Web Application Firewall (WAF) in production

### Monitoring
- Enable security scanning in your CI/CD pipeline
- Monitor logs for suspicious activity
- Set up alerts for security events
- Regularly review access logs

### Database Security
- Use prepared statements to prevent SQL injection
- Implement proper access controls
- Encrypt sensitive data at rest
- Use connection pooling with limits
- Regular backups with encryption

### API Security
- Implement proper authentication and authorization
- Use API keys with limited scope
- Implement rate limiting
- Validate all input
- Use HTTPS only

## Security Features

service-platform includes:

- JWT-based authentication
- Role-based access control (RBAC)
- Rate limiting
- Input validation and sanitization
- Audit logging
- Secure session management
- CORS configuration
- SQL injection prevention
- XSS protection

## Security Scanning

We use multiple security scanning tools:

- **CodeQL**: Semantic code analysis
- **Gosec**: Go security checker
- **Trivy**: Vulnerability and secret scanner
- **Nancy**: Go dependency scanner
- **TruffleHog**: Secret detection
- **Dependabot**: Automated dependency updates

## Known Security Issues

Check our [Security Advisories](https://github.com/wecrazy/service-platform/security/advisories) for known issues and patches.

## Security Updates

Subscribe to security announcements:
- Watch this repository
- Enable security alerts in your GitHub settings
- Check release notes for security fixes

## Contact

For security-related questions that are not vulnerabilities:
- Open a discussion in the repository
- Contact: security@wecrazy.com

Thank you for helping keep service-platform and its users safe!
