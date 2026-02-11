#!/bin/bash
set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

if [ ! -f "$SCRIPT_DIR/.env" ]; then
    echo "Error: .env not found. Copy .env.example and fill in values."
    exit 1
fi

set -a
source "$SCRIPT_DIR/.env"
set +a

: "${DEPLOY_HOST:?DEPLOY_HOST not set in .env}"
: "${DEPLOY_DIR:?DEPLOY_DIR not set in .env}"

echo "Syncing to $DEPLOY_HOST:$DEPLOY_DIR..."
rsync -avz --delete \
    --exclude '.env' \
    --exclude '.git' \
    --exclude 'node_modules' \
    --exclude '.claude' \
    ./ "$DEPLOY_HOST:$DEPLOY_DIR/"

echo "Pushing .env to remote..."
scp "$SCRIPT_DIR/.env" "$DEPLOY_HOST:$DEPLOY_DIR/.env"

echo "Deploying on remote..."
ssh "$DEPLOY_HOST" "cd $DEPLOY_DIR && docker compose down && docker compose up -d --build"

if [ -n "$DEPLOY_TUNNEL_NETWORK" ]; then
    echo "Connecting frontend to tunnel network..."
    ssh "$DEPLOY_HOST" "docker network connect $DEPLOY_TUNNEL_NETWORK treasury_frontend 2>/dev/null || true"
fi

echo "Waiting for health checks..."
sleep 5
ssh "$DEPLOY_HOST" "docker compose -f $DEPLOY_DIR/docker-compose.yml ps"

echo "Deploy complete."
