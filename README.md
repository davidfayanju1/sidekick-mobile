# Sidekick — Two-Sided Marketplace for Local Tasks

A full-stack mobile marketplace connecting people who need tasks done with local workers. Built with a Go backend (PostgreSQL) and React Native (Expo) frontend.

## Project Structure

```
├── backend/          Go API server (Phases 0–11)
│   ├── cmd/api/      Entrypoint
│   ├── internal/     Domain packages (auth, tasks, finance, etc.)
│   ├── migrations/   SQL migrations (run on startup)
│   ├── tests/        Integration tests
│   ├── Dockerfile    Multi-stage build for production
│   └── render.yaml   Render infrastructure definition
├── frontend/         React Native (Expo) mobile app
└── README.md
```

## Backend

### Tech Stack

- **Go** with stdlib `net/http` (no frameworks)
- **PostgreSQL** 15+ with `pgx/v5`
- **JWT** auth via `golang-jwt/jwt/v5`
- **bcrypt** password hashing
- **PostGIS** geo queries for location-based feed

### Features (Phases 0–11)

| Phase | Feature |
|-------|---------|
| 0 | Schema, RLS default-deny, audit log, payments mock, storage |
| 1 | Auth, OTP, profiles, roles, password reset, sessions |
| 2 | Task CRUD, drafts, photos, fee quotes, escrow funding |
| 3 | Discovery feed, full-text search, rate limiting |
| 4 | Offers (accept/counter/decline), atomic assignment |
| 5 | Messaging, blocks, reports, system messages |
| 6 | Task completion, confirmation, disputes, auto-release |
| 7 | Ledger, wallet, cards, payouts, webhook handling |
| 8 | Reviews, double-blind publish, aggregate recompute |
| 9 | Verification review, report triage, leak detection |
| 10 | Notifications (create, list, mark-read, unread count) |
| 11 | Admin console (suspend/ban, disputes, payouts, search) |

### Deploy to Render (One-Click)

**Option A — Blueprint (recommended)**

1. Fork this repo
2. In [Render Dashboard](https://dashboard.render.com), click **New > Blueprint**
3. Select the forked repo — Render reads `render.yaml` and creates:
   - A **PostgreSQL** database (`sidekick-db`)
   - A **Web Service** (`sidekick-api`) with Docker build
4. Render auto-generates a `JWT_SECRET` and wires `DATABASE_URL`
5. Wait ~3 minutes for the first deploy to finish
6. Open `https://sidekick-api.onrender.com/health` — you should see `{"ok":true}`

**Option B — Manual**

1. Create a **PostgreSQL** instance on Render (Starter plan)
2. Create a **Web Service** from this repo:
   - Runtime: **Docker**
   - Dockerfile path: `backend/Dockerfile`
   - Docker context: `backend/`
   - Health check path: `/health`
3. Set environment variables:

   | Variable | Value |
   |----------|-------|
   | `APP_ENV` | `prod` |
   | `DATABASE_URL` | Copy from your Render PostgreSQL dashboard |
   | `JWT_SECRET` | Any random string ≥ 32 characters |

4. Deploy

### Local Development

```bash
# Start Postgres
docker run -d --name sidekick-pg \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=sidekick_dev \
  -p 5433:5432 \
  postgres:15-alpine

# Set env vars
export DATABASE_URL="postgres://postgres:postgres@localhost:5433/sidekick_dev?sslmode=disable"
export JWT_SECRET="dev-secret-must-be-at-least-32-characters-long"
export APP_ENV="dev"

# Run
cd backend
go run ./cmd/api

# Run tests
go test ./tests/... -count=1 -timeout 120s
```

### API Docs

Once running, visit:
- **Interactive docs**: `http://localhost:8080/docs` (Scalar UI)
- **OpenAPI spec**: `http://localhost:8080/openapi.yaml`

### Key Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `POST` | `/auth/signup` | Register with email |
| `POST` | `/auth/login` | Email + password login |
| `POST` | `/auth/otp/send` | Send phone OTP |
| `POST` | `/auth/otp/verify` | Verify OTP |
| `GET` | `/me` | Current user profile |
| `POST` | `/tasks` | Create a task |
| `POST` | `/tasks/{id}/fund` | Fund task (escrow) |
| `GET` | `/feed` | Discovery feed (geo-filtered) |
| `GET` | `/search` | Full-text + trigram search |
| `POST` | `/tasks/{id}/offers` | Make an offer |
| `POST` | `/offers/{id}/accept` | Accept offer (atomic) |
| `GET` | `/conversations` | List conversations |
| `POST` | `/conversations/{id}/messages` | Send message |
| `POST` | `/tasks/{id}/complete` | Mark task complete |
| `POST` | `/tasks/{id}/confirm` | Confirm + settle |
| `POST` | `/tasks/{id}/disputes` | Raise dispute |
| `POST` | `/tasks/{id}/reviews` | Leave review |
| `GET` | `/me/notifications` | List notifications |
| `GET` | `/admin/verifications` | Admin: verification queue |
| `GET` | `/admin/disputes` | Admin: disputes queue |
| `GET` | `/admin/search` | Admin: user search |

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `APP_ENV` | No | `dev` / `staging` / `prod` (default: `dev`) |
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `JWT_SECRET` | Yes | JWT signing secret (≥ 32 chars) |
| `PORT` | No | Server port (default: `8080`) |
| `S3_ENDPOINT` | No | S3-compatible storage endpoint |
| `S3_REGION` | No | S3 region (default: `eu-west-2`) |
| `S3_BUCKET_PREFIX` | No | S3 bucket name prefix |

## Frontend

React Native app built with Expo. See `frontend/` for setup instructions.

## License

Private — All rights reserved.
