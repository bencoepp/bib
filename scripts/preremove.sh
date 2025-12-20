#!/bin/sh
# Pre-removal script for bibd package

set -e

# Stop the service if it's running
if command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet bibd 2>/dev/null; then
        echo "Stopping bibd service..."
        systemctl stop bibd
    fi

    if systemctl is-enabled --quiet bibd 2>/dev/null; then
        echo "Disabling bibd service..."
        systemctl disable bibd
    fi
fi

echo ""
echo "bibd service has been stopped and disabled."
echo ""
echo "Note: Configuration and data directories are preserved:"
echo "  - /etc/bibd (configuration)"
echo "  - /var/lib/bibd (data)"
echo "  - /var/log/bibd (logs)"
echo ""
echo "To completely remove all data, run:"
echo "  sudo rm -rf /etc/bibd /var/lib/bibd /var/log/bibd"
echo "  sudo userdel bibd"
echo "  sudo groupdel bibd"

exit 0

