#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_SRC="$SCRIPT_DIR/crybaby-controller.service"
SERVICE_DST="/etc/systemd/system/crybaby-controller.service"

echo "======================================================="
echo "   CRYBABY CONTROLLER - SYSTEMD SERVICE INSTALLER      "
echo "======================================================="

if [ "$EUID" -ne 0 ]; then
    echo "Memasang service crybaby-controller ke $SERVICE_DST via sudo..."
    sudo cp "$SERVICE_SRC" "$SERVICE_DST"
    sudo systemctl daemon-reload
    sudo systemctl enable crybaby-controller.service
    sudo systemctl restart crybaby-controller.service
else
    cp "$SERVICE_SRC" "$SERVICE_DST"
    systemctl daemon-reload
    systemctl enable crybaby-controller.service
    systemctl restart crybaby-controller.service
fi

echo "✅ Service crybaby-controller berhasil dipasang dan diaktifkan!"
echo "Status service:"
systemctl status crybaby-controller.service --no-pager || true
