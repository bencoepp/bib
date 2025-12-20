#!/bin/sh
# Post-installation script for bibd package

set -e

# Create bibd user and group if they don't exist
if ! getent group bibd >/dev/null 2>&1; then
    groupadd --system bibd
fi

if ! getent passwd bibd >/dev/null 2>&1; then
    useradd --system --gid bibd --home-dir /var/lib/bibd --shell /usr/sbin/nologin bibd
fi

# Create necessary directories
mkdir -p /var/lib/bibd
mkdir -p /etc/bibd
mkdir -p /var/log/bibd

# Set ownership
chown -R bibd:bibd /var/lib/bibd
chown -R bibd:bibd /etc/bibd
chown -R bibd:bibd /var/log/bibd

# Set permissions
chmod 750 /var/lib/bibd
chmod 750 /etc/bibd
chmod 750 /var/log/bibd

# Create default config if it doesn't exist
if [ ! -f /etc/bibd/config.yaml ]; then
    cat > /etc/bibd/config.yaml << 'EOF'
# bibd configuration
# See: https://github.com/bencoepp/bib/blob/main/docs/getting-started/configuration.md

server:
  address: "0.0.0.0:8080"
  data_dir: "/var/lib/bibd/data"

log:
  level: "info"
  format: "json"
  path: "/var/log/bibd/bibd.log"

# Storage configuration
storage:
  type: "sqlite"
  path: "/var/lib/bibd/data/bib.db"

# P2P configuration
p2p:
  enabled: true
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/4001"
    - "/ip6/::/tcp/4001"
EOF
    chown bibd:bibd /etc/bibd/config.yaml
    chmod 640 /etc/bibd/config.yaml
fi

# Reload systemd and enable service
if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
    echo ""
    echo "bibd has been installed. To start the service:"
    echo "  sudo systemctl start bibd"
    echo "  sudo systemctl enable bibd"
    echo ""
    echo "Configuration file: /etc/bibd/config.yaml"
    echo "Data directory: /var/lib/bibd"
    echo "Log directory: /var/log/bibd"
fi

exit 0

