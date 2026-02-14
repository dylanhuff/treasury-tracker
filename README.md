# Treasury Tracker

A full-stack application for managing treasury securities and liquidity. Built as a weekend project to explore treasury yield curves and learn how T-Bill discount pricing works.

## Overview

This application provides real-time treasury yield curve visualization and allows users to manage treasury security portfolios with buy/sell functionality and comprehensive transaction tracking.

## Features

- Live treasury yield data from Treasury.gov API (1-hour cache)
- Interactive yield curve visualization
- User account management with fund/withdraw operations
- Buy treasury securities (bills, notes, bonds)
- Sell holdings with yield calculations
- Complete transaction history
- Mobile-responsive interface

## Tech Stack

**Backend:** Go 1.24+, Chi router, PostgreSQL driver (pgx), sqlc for type-safe SQL
**Frontend:** React 18, TypeScript, Vite, Tailwind CSS, Tremor charts
**Database:** PostgreSQL 16
**Deployment:** Docker Compose

## Prerequisites

- Docker Desktop 20.10+ with Docker Compose v2
- (For local development) Go 1.24+, Node.js 18+, npm

## Quick Start (Docker - Recommended)

### 1. Configure environment
```bash
cp .env.example .env
```

Edit `.env` with your database credentials. Defaults are provided for local development.

### 2. Start all services
```bash
docker compose up -d
```

This command automatically builds the Docker images from source (first run takes 2-3 minutes) and starts all services.

### 3. Initialize database with schema and demo data
```bash
cd backend/db
./deploy-fresh-schema.sh
```

### 4. Access the application
- **Frontend:** http://localhost
- **Backend API:** http://localhost:8080
- **Health check:** http://localhost:8080/health

**Note:** The backend preloads yield data cache on startup (10-30 seconds). The chart may need 1-2 refreshes initially.

### Stop the application
```bash
docker compose down

# To remove all data and start fresh
docker compose down -v
```

## Development Setup

For faster development with hot reloading:

### 1. Configure environment
```bash
cp .env.example .env
```

### 2. Start PostgreSQL
```bash
docker compose up -d postgres
```

### 3. Deploy Database Schema
```bash
cd backend/db
./deploy-fresh-schema.sh
```

### 4. Configure Backend Environment
Create a `backend/.env` using credentials from your root `.env`:
```bash
# Example using defaults from .env.example:
echo 'DATABASE_URL=postgres://postgres:postgres@localhost:5432/treasury_db?sslmode=disable' > backend/.env
```

### 5. Start Backend
```bash
cd backend
go run ./cmd/server/main.go
```

Backend runs on http://localhost:8080

**Note:** On first startup, the backend will warm the cache by fetching historical yield data for all time periods (1W, 1M, 3M, 6M, 1Y, 5Y, 10Y, 30Y). This process takes 10-30 seconds. During this time, the yield curve chart may show loading states or require a refresh.

### 6. Start Frontend
```bash
cd frontend
npm install
npm run dev
```

Frontend runs on http://localhost:5173

## Rebuilding Docker Images

Images are built automatically on first `docker compose up -d`. Rebuild manually when you make code changes:

```bash
# Rebuild all services
docker compose build

# Rebuild and restart specific service
docker compose up -d --build backend
docker compose up -d --build frontend
```

## API Endpoints

- `GET /api/yields` - Current treasury yield curve data
- `GET /api/yields/historical` - Historical yield data for charting
- `GET /api/v1/users` - List all users
- `GET /api/v1/users/{userId}/transactions` - User transaction history
- `GET /api/v1/users/{userId}/holdings` - User active holdings
- `POST /api/v1/fund` - Add funds to account
- `POST /api/v1/withdraw` - Withdraw funds from account
- `POST /api/v1/buy` - Purchase treasury security
- `POST /api/v1/sell` - Sell treasury holding
- `GET /health` - Backend health check

## Database Schema

The application uses PostgreSQL with the following main tables:
- `users` - User accounts with balances
- `transactions` - All financial transactions (fund, withdraw, buy, sell)
- `holdings` - Treasury security holdings with remaining amounts

Schema is in `backend/db/schema.sql` and is automatically applied via Docker.

## Project Structure

```
.
├── backend/
│   ├── cmd/server/          # Main application entry point
│   ├── internal/
│   │   ├── database/        # sqlc generated code
│   │   ├── handlers/        # HTTP request handlers
│   │   ├── services/        # Business logic
│   │   └── utils/           # Utilities (yield calculations)
│   └── db/
│       ├── queries/         # SQL queries for sqlc
│       └── schema.sql       # Database schema
├── frontend/
│   └── src/
│       ├── components/      # React components
│       ├── services/        # API client
│       ├── contexts/        # React contexts
│       └── types/           # TypeScript types
├── docker-compose.yml       # Docker orchestration
├── deploy-local.sh          # Deploy on the current machine
└── deploy-remote.sh         # Deploy to a remote host via SSH
```

## Testing

```bash
# Backend tests
cd backend
go test -v ./...

# Frontend tests
cd frontend
npm test
```

## Troubleshooting

### Starting completely fresh
If you want to remove all existing containers and start from scratch:
```bash
# Remove all containers and volumes
docker compose down -v

# Remove any lingering containers (if any exist from previous runs)
docker rm -f treasury_postgres treasury_backend treasury_frontend 2>/dev/null || true

# Restart with Quick Start or Development Setup steps
docker compose up -d
cd backend/db && ./deploy-fresh-schema.sh
```

### Port 80 already in use
```bash
# Check what's using port 80
lsof -i :80

# Or change the port in docker-compose.yml:
# ports:
#   - "8080:80"  # Access on localhost:8080 instead
```

### Reset database
```bash
docker compose down -v
docker compose up -d
```

### Backend won't start - "DATABASE_URL not set"
Make sure you've created the `.env` file in the `backend/` directory (see Development Setup step 4).

### View logs
```bash
# All services
docker compose logs -f

# Specific service
docker compose logs -f backend
docker compose logs -f frontend
```

## Deployment

### Deploy locally (on the host machine)
```bash
./deploy-local.sh
```
Builds and starts all containers with `docker compose` on the current machine. Requires Docker and a `.env` file.

### Deploy remotely (from another machine via SSH)
```bash
./deploy-remote.sh
```
Rsyncs the project to a remote host and builds there. Requires `DEPLOY_HOST` and `DEPLOY_DIR` set in `.env`.

Both scripts source `.env` for configuration. See `.env.example` for available options.

## Notes

- Initial user accounts are created via seed data with demo balances
- Treasury yield data is cached for 1 hour from Treasury.gov
- **First-time startup:** The backend preloads the yield data cache on startup, which can take 10-30 seconds. The yield curve chart may require 1-2 manual refreshes during this initial cache warming period.
- Buy orders for T-Bills use discount pricing (pay less than face value)
- Sell operations calculate accrued yield based on time held and current rates
- Database credentials are managed via `.env` (see `.env.example`). In production, these are populated by the deploy pipeline.
