#!/usr/bin/env bash
set -euo pipefail

# TLD Redirect Engine — deploy to multi-region infrastructure
# Usage: ./scripts/deploy-multi.sh <mode> <server-ip> [env-file]
#   mode: control | data
#   env-file: path to env file (default: creates from env vars)

MODE="${1:?Usage: deploy-multi.sh <control|data> <server-ip> [env-file]}"
SERVER_IP="${2:?Usage: deploy-multi.sh <control|data> <server-ip> [env-file]}"
ENV_FILE="${3:-}"
SSH_USER="root"
REMOTE_DIR="/opt/tld-redirect"

if [[ "$MODE" != "control" && "$MODE" != "data" ]]; then
  echo "Error: mode must be 'control' or 'data'" >&2
  exit 1
fi

SERVICE_NAME="tld-redirect-${MODE}"
SERVICE_FILE="scripts/${SERVICE_NAME}.service"

if [ ! -f "$SERVICE_FILE" ]; then
  echo "Error: service file not found: $SERVICE_FILE" >&2
  exit 1
fi

echo "==> Building binary (CGO_ENABLED=0 for PG-only)..."
CGO_ENABLED=0 go build \
  -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
  -o "bin/tld-redirect" ./cmd/tld-redirect

echo "==> Deploying ${MODE} plane to ${SERVER_IP}..."
ssh -o StrictHostKeyChecking=no "${SSH_USER}@${SERVER_IP}" "mkdir -p ${REMOTE_DIR}/data"
scp -o StrictHostKeyChecking=no "bin/tld-redirect" "${SSH_USER}@${SERVER_IP}:${REMOTE_DIR}/tld-redirect"
scp -o StrictHostKeyChecking=no "$SERVICE_FILE" "${SSH_USER}@${SERVER_IP}:/etc/systemd/system/${SERVICE_NAME}.service"

if [ -n "$ENV_FILE" ]; then
  echo "==> Uploading env file..."
  scp -o StrictHostKeyChecking=no "$ENV_FILE" "${SSH_USER}@${SERVER_IP}:${REMOTE_DIR}/env"
fi

echo "==> Configuring and starting service..."
ssh "${SSH_USER}@${SERVER_IP}" bash <<REMOTE
set -euo pipefail

# Allow binding to privileged ports
setcap cap_net_bind_service=+ep ${REMOTE_DIR}/tld-redirect

# Ensure env file exists
if [ ! -f ${REMOTE_DIR}/env ]; then
  echo "Warning: ${REMOTE_DIR}/env not found. Create it with required variables." >&2
  echo "Required: TLD_DB_URL, TLD_ADMIN_TOKEN (control), TLD_ADMIN_DOMAIN" >&2
  echo "Optional: TLD_SYNC_ENDPOINT, TLD_SYNC_BUCKET, TLD_SYNC_ACCESS_KEY, TLD_SYNC_SECRET_KEY, TLD_SYNC_REGION" >&2
  echo "Optional (data): TLD_DS2_ENDPOINT" >&2
  exit 1
fi

chmod 600 ${REMOTE_DIR}/env

# Stop old legacy service if running
systemctl stop tld-redirect 2>/dev/null || true
systemctl disable tld-redirect 2>/dev/null || true

# Enable and restart mode-specific service
systemctl daemon-reload
systemctl enable ${SERVICE_NAME}
systemctl restart ${SERVICE_NAME}
sleep 2
systemctl status ${SERVICE_NAME} --no-pager
REMOTE

echo ""
echo "==> Deploy complete!"
echo "    Mode:   ${MODE}"
echo "    Server: ${SERVER_IP}"
echo "    Service: ${SERVICE_NAME}"
if [ "$MODE" = "control" ]; then
  echo "    Admin UI: https://<admin-domain>/ui/?token=<token>"
fi
