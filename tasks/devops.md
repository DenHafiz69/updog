# Updog DevOps & Infrastructure Tasks

Quantized tasks for containerizing, orchestrating, automating builds, and establishing CI/CD pipelines for Updog.

---

# Milestone 1: Containerization & Local Orchestration

## [ ] Task 01: Multi-Stage Dockerfile for API Service
- **Target File:** `build/Dockerfile.api`
- [ ] Write Stage 1 (`builder`): `golang:1.22-alpine` to compile `cmd/api/main.go` into static binary
- [ ] Write Stage 2 (`runtime`): `alpine:latest` or `gcr.io/distroless/static` copying only the binary
- [ ] Expose port 8080 and set non-root user execution
- **DoD:** `docker build -f build/Dockerfile.api -t updog-api .` succeeds producing image < 20MB

## [ ] Task 02: Multi-Stage Dockerfile for Worker Service
- **Target File:** `build/Dockerfile.worker`
- [ ] Write Stage 1 (`builder`): compile `cmd/worker/main.go` with CGO disabled (`CGO_ENABLED=0`)
- [ ] Write Stage 2 (`runtime`): minimal runtime image containing SSL CA certificates (`ca-certificates`) for HTTPS monitoring
- **DoD:** `docker build -f build/Dockerfile.worker -t updog-worker .` compiles static binary successfully

## [ ] Task 03: Centralized Environment Configuration
- **Target File:** `.env.example`
- [ ] Define standard environment variables (`POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `API_PORT`, `WORKER_TICK_SECONDS`)
- [ ] Create `internal/config/config.go` loader to read env vars with sensible defaults
- **DoD:** App services correctly read configuration values from `.env` file or environment variables

## [ ] Task 04: Full Stack Docker Compose Setup
- **Target File:** `docker-compose.yml`
- [ ] Define services: `db` (Postgres 16), `api` (build `Dockerfile.api`), `worker` (build `Dockerfile.worker`)
- [ ] Add `healthcheck` block for PostgreSQL service to defer dependent service startups (`depends_on: db: condition: service_healthy`)
- [ ] Add volume mount for database persistence (`postgres_data`)
- **DoD:** `docker compose up --build` launches DB, API, and Worker into ready operating state with one command

---

# Milestone 2: Automation & Tooling

## [ ] Task 05: Build Automation Makefile
- **Target File:** `Makefile`
- [ ] Add targets: `make build`, `make run-api`, `make run-worker`, `make docker-up`, `make docker-down`, `make test`
- [ ] Add `make migrate-up` command linking to SQL migration runner
- **DoD:** `make help` lists available targets and `make test` executes `go test ./...`

## [ ] Task 06: Database Migration Script
- **Target File:** `scripts/migrate.sh`
- [ ] Create shell script using `psql` or `migrate` CLI to auto-apply `migrations/*.sql` on start
- [ ] Wire script as an init container or entrypoint check for DB initialization
- **DoD:** Script runs migrations idempotently without failing if tables already exist

---

# Milestone 3: CI/CD Pipelines & Production Readiness

## [ ] Task 07: GitHub Actions CI Workflow
- **Target File:** `.github/workflows/ci.yml`
- [ ] Configure workflow triggered on `push` and `pull_request` to `main`
- [ ] Step 1: Run `go vet ./...` and `golangci-lint`
- [ ] Step 2: Run unit and integration tests (`go test -v -race ./...`)
- [ ] Step 3: Build Docker images for `api` and `worker` to verify Dockerfile validity
- **DoD:** GitHub Actions workflow executes cleanly on push and reports build status

## [ ] Task 08: Operational Health Checks & Graceful Shutdown
- **Target File:** `cmd/api/main.go` & `cmd/worker/main.go`
- [ ] Implement `GET /healthz` (liveness) and `GET /readyz` (readiness checking DB ping)
- [ ] Implement OS signal handling (`SIGINT`, `SIGTERM`) for smooth container termination without dropping checks
- **DoD:** Killing container with `SIGTERM` allows in-flight HTTP requests and database operations to finish cleanly
