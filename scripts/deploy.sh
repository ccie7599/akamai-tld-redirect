#!/usr/bin/env bash
set -euo pipefail

# TLD Redirect Engine — deploy to Linode
# Usage: ./scripts/deploy.sh <server-ip> [admin-token]

SERVER_IP="${1:?Usage: deploy.sh <server-ip> [admin-token]}"
ADMIN_TOKEN="${2:-$(openssl rand -hex 16)}"
SSH_USER="root"
REMOTE_DIR="/opt/tld-redirect"
BINARY="bin/tld-redirect"

echo "==> Building binary..."
CGO_ENABLED=1 go build \
  -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
  -o "$BINARY" ./cmd/tld-redirect

echo "==> Uploading to ${SERVER_IP}..."
ssh -o StrictHostKeyChecking=no "${SSH_USER}@${SERVER_IP}" "mkdir -p ${REMOTE_DIR}/data"
scp -o StrictHostKeyChecking=no "$BINARY" "${SSH_USER}@${SERVER_IP}:${REMOTE_DIR}/tld-redirect"
scp -o StrictHostKeyChecking=no sample-data/redirects.json "${SSH_USER}@${SERVER_IP}:${REMOTE_DIR}/redirects.json"
scp -o StrictHostKeyChecking=no scripts/tld-redirect.service "${SSH_USER}@${SERVER_IP}:/etc/systemd/system/tld-redirect.service"

echo "==> Configuring environment..."
ssh "${SSH_USER}@${SERVER_IP}" bash <<REMOTE
set -euo pipefail

# Write env file
cat > ${REMOTE_DIR}/env <<EOF
TLD_ADMIN_TOKEN=${ADMIN_TOKEN}
EOF
chmod 600 ${REMOTE_DIR}/env

# Allow binding to privileged ports
setcap cap_net_bind_service=+ep ${REMOTE_DIR}/tld-redirect

# Seed data on first deploy
if [ ! -f ${REMOTE_DIR}/data/tld-redirect.db ]; then
  echo "First deploy — seeding sample data..."
  cd ${REMOTE_DIR}
  ./tld-redirect -db data/tld-redirect.db -seed redirects.json -token dummy &
  sleep 3
  kill %1 2>/dev/null || true
  wait 2>/dev/null || true
fi

# Enable and restart service
systemctl daemon-reload
systemctl enable tld-redirect
systemctl restart tld-redirect
sleep 2
systemctl status tld-redirect --no-pager
REMOTE

echo ""
echo "==> Deploy complete!"
echo "    Admin token: ${ADMIN_TOKEN}"
echo "    Admin UI:    https://redirects.connected-cloud.io/?token=${ADMIN_TOKEN}"
echo "    API test:    curl -s 'https://redirects.connected-cloud.io/api/v1/domains?token=${ADMIN_TOKEN}'"
echo "    Redirect:    curl -sI -H 'Host: old-brand-financial.example.com' http://${SERVER_IP}/"
