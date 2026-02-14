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

echo "Building and deploying locally..."
docker compose down
docker compose up -d --build

if [ -n "$DEPLOY_TUNNEL_NETWORK" ]; then
    echo "Connecting frontend to tunnel network..."
    docker network connect "$DEPLOY_TUNNEL_NETWORK" treasury_frontend 2>/dev/null || true
fi

echo "Waiting for health checks..."
sleep 5
docker compose ps

echo "Deploy complete."
