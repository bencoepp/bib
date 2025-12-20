# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

**DO NOT** create a public GitHub issue for security vulnerabilities.

Instead, please report security vulnerabilities by emailing:

**security@bib.dev** (or create a private security advisory on GitHub)

### What to Include

Please include the following in your report:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)
- Your contact information

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Resolution Target**: Within 90 days (depending on severity)

### What to Expect

1. **Acknowledgment**: We'll confirm receipt of your report
2. **Investigation**: We'll investigate and determine impact
3. **Fix Development**: We'll develop and test a fix
4. **Coordinated Disclosure**: We'll work with you on disclosure timing
5. **Credit**: We'll credit you in the security advisory (if desired)

## Security Best Practices for Users

### Running bibd

- Run bibd as a non-root user
- Use the systemd service file for proper sandboxing
- Keep bibd updated to the latest version
- Use PostgreSQL mode for production (not SQLite)
- Enable TLS for gRPC connections
- Review and restrict network exposure

### Configuration Security

- Never commit configuration files with secrets
- Use environment variables for sensitive values
- Protect `~/.config/bibd/` directory permissions (700)
- Rotate credentials regularly

### Container Security

- Use official images from `ghcr.io/bencoepp/bibd`
- Run containers as non-root (our images do this by default)
- Use read-only root filesystem where possible
- Scan images for vulnerabilities regularly

## Known Security Considerations

### P2P Networking

- Peer connections are encrypted using Noise protocol
- Node identities are Ed25519 keypairs
- Bootstrap peers should be from trusted sources

### Database Security

- bibd manages PostgreSQL credentials internally
- Break-glass access is audited
- See [Database Security](docs/storage/database-security.md)

### Authentication

- Ed25519 signature-based authentication
- Challenge-response protocol
- See [Authentication](docs/concepts/authentication.md)

## Security Updates

Security updates are released as patch versions and announced via:

- GitHub Security Advisories
- Release notes
- bib.dev website (coming soon)

## Bug Bounty

We do not currently have a bug bounty program, but we deeply appreciate security researchers who help keep Bib secure. Responsible disclosure will be acknowledged in our security advisories.

