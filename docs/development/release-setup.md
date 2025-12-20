# Release Setup Guide

This document explains how to set up the release pipeline for the bib project.

## Prerequisites

- GitHub repository: `bencoepp/bib`
- Homebrew tap repository: `bencoepp/homebrew-bib`
- GPG key for signing releases
- GitHub Personal Access Token for Homebrew tap

## Required GitHub Secrets

The following secrets must be configured in your GitHub repository settings (`Settings > Secrets and variables > Actions`):

| Secret Name | Description |
|-------------|-------------|
| `GPG_PRIVATE_KEY` | ASCII-armored GPG private key for signing |
| `GPG_PASSPHRASE` | Passphrase for the GPG key |
| `GPG_FINGERPRINT` | GPG key fingerprint (40-character hex string) |
| `HOMEBREW_TAP_GITHUB_TOKEN` | Personal Access Token with `repo` scope for `bencoepp/homebrew-bib` |
| `WINGET_TOKEN` | Personal Access Token for Winget submissions (needs `public_repo` scope) |

## Step-by-Step Setup

### 1. Create a GPG Key for Signing

If you don't have a GPG key, create one:

```bash
# Generate a new GPG key
gpg --full-generate-key

# Select:
# - RSA and RSA (default)
# - 4096 bits
# - Key does not expire (or set expiry as needed)
# - Your name and email (use your GitHub email)
# - Set a strong passphrase
```

### 2. Export GPG Key for GitHub

```bash
# List your keys to get the key ID
gpg --list-secret-keys --keyid-format=long

# Output looks like:
# sec   rsa4096/ABCD1234EFGH5678 2024-01-01 [SC]
#       FINGERPRINT1234567890FINGERPRINT1234567890
# uid                 [ultimate] Your Name <your.email@example.com>

# Export the private key (ASCII-armored)
gpg --armor --export-secret-keys ABCD1234EFGH5678 > private-key.asc

# The FINGERPRINT is the 40-character hex string shown
# Copy the fingerprint for GPG_FINGERPRINT secret
```

### 3. Add GPG Secrets to GitHub

1. Go to `https://github.com/bencoepp/bib/settings/secrets/actions`
2. Click "New repository secret"
3. Add the following:

   - **GPG_PRIVATE_KEY**: Paste the entire contents of `private-key.asc`
   - **GPG_PASSPHRASE**: Your GPG key passphrase
   - **GPG_FINGERPRINT**: The 40-character fingerprint (no spaces)

4. **Delete the private-key.asc file** after uploading:
   ```bash
   rm private-key.asc
   ```

### 4. Create Homebrew Tap Repository

1. Create a new repository: `bencoepp/homebrew-bib`
2. Add a README.md:

```markdown
# Homebrew Tap for Bib

This tap contains Homebrew formulae for the Bib distributed data management platform.

## Installation

```bash
brew tap bencoepp/bib
brew install bib
brew install bibd
```

## Formulae

- **bib** - Command-line interface for Bib
- **bibd** - Background daemon for Bib
```

3. Create an empty `Formula/` directory with a `.gitkeep` file

### 5. Create Personal Access Token for Homebrew

1. Go to `https://github.com/settings/tokens`
2. Click "Generate new token (classic)"
3. Name: `homebrew-bib-releases`
4. Expiration: Set as appropriate (recommend 1 year)
5. Scopes: Select `repo` (full control of private repositories)
6. Click "Generate token"
7. Copy the token
8. Add as `HOMEBREW_TAP_GITHUB_TOKEN` secret in `bencoepp/bib`

### 6. Create Personal Access Token for Winget

1. Go to `https://github.com/settings/tokens`
2. Click "Generate new token (classic)"
3. Name: `winget-submissions`
4. Expiration: Set as appropriate
5. Scopes: Select `public_repo`
6. Click "Generate token"
7. Copy the token
8. Add as `WINGET_TOKEN` secret in `bencoepp/bib`

## Creating a Release

### Automated Release (Recommended)

1. Update the version in your code if needed
2. Create and push a tag:

```bash
# Create an annotated tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag
git push origin v1.0.0
```

3. The GitHub Action will automatically:
   - Build binaries for all platforms
   - Create Linux packages (.deb, .rpm)
   - Build and push Docker images
   - Create a GitHub release with all assets
   - Update Homebrew tap
   - Submit to Winget (Windows Package Manager)

### Manual Release (For Testing)

```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser/v2@latest

# Dry run (no publishing)
goreleaser release --snapshot --clean

# Check the dist/ directory for generated artifacts
ls -la dist/
```

## Verifying Releases

### Verify GPG Signatures

```bash
# Download the checksum file and signature
curl -LO https://github.com/bencoepp/bib/releases/download/v1.0.0/checksums.txt
curl -LO https://github.com/bencoepp/bib/releases/download/v1.0.0/checksums.txt.sig

# Import the public key (first time only)
curl -sSL https://github.com/bencoepp.gpg | gpg --import

# Verify the signature
gpg --verify checksums.txt.sig checksums.txt

# Verify individual files
sha256sum -c checksums.txt --ignore-missing
```

### Verify Docker Images

```bash
# Pull and verify the image
docker pull ghcr.io/bencoepp/bibd:v1.0.0

# Check image labels
docker inspect ghcr.io/bencoepp/bibd:v1.0.0 | jq '.[0].Config.Labels'
```

## Package Installation

### Homebrew (macOS/Linux)

```bash
# Add the tap
brew tap bencoepp/bib

# Install CLI
brew install bib

# Install daemon
brew install bibd

# Start daemon as a service
brew services start bibd
```

### Windows (Winget)

```powershell
# Install CLI
winget install bencoepp.bib

# Install daemon
winget install bencoepp.bibd
```

### Debian/Ubuntu (.deb)

```bash
# Download the package
curl -LO https://github.com/bencoepp/bib/releases/download/v1.0.0/bibd_1.0.0_linux_amd64.deb

# Install
sudo dpkg -i bibd_1.0.0_linux_amd64.deb

# Start the service
sudo systemctl start bibd
sudo systemctl enable bibd
```

### RHEL/Fedora/CentOS (.rpm)

```bash
# Download the package
curl -LO https://github.com/bencoepp/bib/releases/download/v1.0.0/bibd-1.0.0.x86_64.rpm

# Install
sudo rpm -i bibd-1.0.0.x86_64.rpm

# Or with dnf
sudo dnf install ./bibd-1.0.0.x86_64.rpm

# Start the service
sudo systemctl start bibd
sudo systemctl enable bibd
```

### Docker

```bash
# Pull the image
docker pull ghcr.io/bencoepp/bibd:latest

# Run with data persistence
docker run -d \
  --name bibd \
  -v bibd-data:/data \
  -p 8080:8080 \
  -p 4001:4001 \
  ghcr.io/bencoepp/bibd:latest
```

## Future: Self-Hosted Repositories

In a future release, `bibd` will be able to host its own package repositories. The bootstrap instance at `bib.dev` will serve as the primary repository, with other nodes able to mirror packages for their users.

This will enable:
- Decentralized package distribution
- Self-update capability for `bib` and `bibd`
- Auto-update with configurable policies
- Offline/air-gapped installations via local mirrors

## Troubleshooting

### GoReleaser Validation Errors

```bash
# Check configuration
goreleaser check

# Common issues:
# - Missing LICENSE file
# - Invalid YAML syntax
# - Missing required fields
```

### GPG Signing Failures

```bash
# Test GPG locally
echo "test" | gpg --clearsign

# If it fails, check:
# - GPG agent is running
# - Key is not expired
# - Passphrase is correct
```

### Docker Build Failures

```bash
# Test Docker build locally
docker build -f docker/bibd.Dockerfile -t bibd-test .

# Common issues:
# - Dockerfile syntax errors
# - Missing COPY files
# - Platform-specific issues
```

### Homebrew Formula Errors

```bash
# Test formula locally
brew install --build-from-source ./Formula/bib.rb

# Audit the formula
brew audit --strict bib
```

