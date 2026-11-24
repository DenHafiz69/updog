# Updog Implementation Tasks

# Milestone 1: Workspace Initialization & Core Data Models

## [x] Task 01: Initialize Go Module & Directory Tree
- **Target File:** `go.mod`
- [x] Initialize Go module (`go mod init updog`)
- [x] Create directory structure: `cmd/api`, `cmd/worker`, `internal/model`, `internal/worker`, `internal/api/handlers`, `internal/repository`, `migrations`
- **DoD:** Directory structure exists and `go env` runs cleanly without errors

## [x] Task 02: Setup Local PostgreSQL Docker Environment
- **Target File:** `docker-compose.yml`
- [x] Create `docker-compose.yml` defining PostgreSQL service (port 5432, database `updog`, user/pass credentials)
- [x] Verify database container starts cleanly via `docker compose up -d`
- **DoD:** `docker compose ps` shows healthy Postgres instance running on port 5432

## [ ] Task 03: Create Database Migration Schema
- **Target File:** `migrations/001_init.sql`
- [ ] Write DDL for `monitors` table (ID UUID, URL, interval_seconds, timeout_seconds, method, is_active, timestamps)
- [ ] Write DDL for `check_logs` table (id BIGSERIAL, monitor_id UUID FK, is_up, status_code, response_time_ms, error_message, checked_at)
- [ ] Add index `idx_check_logs_monitor_time` on `check_logs (monitor_id, checked_at DESC)`
- **DoD:** SQL script executes without syntax errors against Postgres database

## [ ] Task 04: Define Core Data Models
- **Target File:** `internal/model/monitor.go`
- [ ] Define `Monitor` struct (ID, URL, IntervalSeconds, TimeoutSeconds, Method, IsActive, CreatedAt, UpdatedAt, LastCheckedAt)
- [ ] Define `CheckResult` struct (ID, MonitorID, IsUp, StatusCode, ResponseTimeMs, ErrorMessage, CheckedAt)
- **DoD:** Code compiles cleanly with `go build ./...`

---

# Milestone 2: Core HTTP Worker Engine

## [ ] Task 05: Implement HTTP Request Execution
- **Target File:** `internal/worker/http.go`
- [ ] Write function signature `ExecuteCheck(ctx context.Context, monitor model.Monitor) model.CheckResult`
- [ ] Create child context with timeout using `monitor.TimeoutSeconds`
- [ ] Construct `http.Request` with `monitor.Method`, `monitor.URL`, and set `User-Agent: Updog-Monitor/1.0` header
- [ ] Measure latency using `time.Now()` and `time.Since()` around `httpClient.Do(req)`
- **DoD:** Compiles cleanly with `go build ./...`

## [ ] Task 06: Handle Response Evaluation & Body Cleanup
- **Target File:** `internal/worker/http.go`
- [ ] Add `io.Copy(io.Discard, resp.Body)` and `resp.Body.Close()` inside cleanup logic
- [ ] Record network/timeout errors into `CheckResult` (`IsUp = false`, `ErrorMessage = err.Error()`)
- [ ] Evaluate HTTP status code (200-399 as `IsUp = true`, >=400 as `IsUp = false` with `http.StatusText`)
- **DoD:** `ExecuteCheck` correctly evaluates both successful (200) and failing (404/500/timeout) HTTP endpoints

## [ ] Task 07: Create HTTP Checker CLI Prototype
- **Target File:** `cmd/worker/main.go`
- [ ] Write temporary `main()` function constructing a sample `model.Monitor` for `https://google.com`
- [ ] Call `worker.ExecuteCheck()` and print formatted `CheckResult` output to console
- **DoD:** `go run cmd/worker/main.go` prints valid status, latency, and status code for Google

---

# Milestone 3: Database Access & Repository Layer

## [ ] Task 08: Setup Database Connection Pool
- **Target File:** `internal/db/db.go`
- [ ] Import `pgx/v5/pgxpool` or standard `database/sql` with `lib/pq` / `pgx` driver
- [ ] Create `ConnectDB(connString string) (*pgxpool.Pool, error)` with ping verification
- **DoD:** Connecting to local Postgres returns a non-nil DB pool without error

## [ ] Task 09: Implement Monitor Repository
- **Target File:** `internal/repository/monitor.go`
- [ ] Create `MonitorRepository` interface and struct implementation
- [ ] Add `Create(ctx, monitor)` to insert new target into `monitors` table
- [ ] Add `GetActiveMonitors(ctx)` to fetch all records where `is_active = true`
- [ ] Add `UpdateLastChecked(ctx, monitorID, checkedAt)` to update check timestamps
- **DoD:** Unit/integration test successfully inserts and retrieves a `Monitor` record from DB

## [ ] Task 10: Implement Check Result Log Repository
- **Target File:** `internal/repository/check_result.go`
- [ ] Create `CheckResultRepository` interface and struct implementation
- [ ] Add `SaveCheckResult(ctx, result)` to insert check logs into `check_logs` table
- [ ] Add `GetLatestResults(ctx, monitorID, limit)` to fetch recent check history ordered by `checked_at DESC`
- **DoD:** `SaveCheckResult` successfully inserts check logs into database

---

# Milestone 4: Worker Loop & Concurrency Scheduler

## [ ] Task 11: Implement Individual Check Worker Routine
- **Target File:** `internal/worker/scheduler.go`
- [ ] Implement `processMonitorCheck(ctx, repo, monitor)` helper function
- [ ] Execute `ExecuteCheck(ctx, monitor)`
- [ ] Persist `CheckResult` via `CheckResultRepository.SaveCheckResult`
- [ ] Update monitor's check timestamp via `MonitorRepository.UpdateLastChecked`
- **DoD:** Helper function executes check and writes result to DB concurrently without race conditions

## [ ] Task 12: Implement Worker Loop Ticker
- **Target File:** `internal/worker/scheduler.go`
- [ ] Implement `StartWorkerLoop(ctx context.Context, monitorRepo, resultRepo)`
- [ ] Create `time.NewTicker(10 * time.Second)` loop responding to `ctx.Done()`
- [ ] Query active monitors and spawn goroutine per eligible monitor (if interval elapsed)
- **DoD:** Worker loop runs continuously, picking up monitors and dispatching checks on schedule

## [ ] Task 13: Assemble & Run Worker Executable
- **Target File:** `cmd/worker/main.go`
- [ ] Initialize DB connection from environment variables
- [ ] Instantiate repositories and launch `StartWorkerLoop` with graceful shutdown signal (`os.Interrupt`, `syscall.SIGTERM`)
- **DoD:** `go run cmd/worker/main.go` runs continuously, executing scheduled checks and logging to Postgres

---

# Milestone 5: REST API & Management Handlers

## [ ] Task 14: Setup HTTP API Router Engine
- **Target File:** `cmd/api/main.go`
- [ ] Choose and initialize HTTP router (e.g. `net/http` servemux or `go-chi/chi`)
- [ ] Implement basic `/healthz` endpoint returning `200 OK`
- **DoD:** `curl http://localhost:8080/healthz` returns `200 OK`

## [ ] Task 15: Define API Data Transfer Objects (DTOs) & Validation
- **Target File:** `internal/api/dto/monitor.go`
- [ ] Define `CreateMonitorRequest` struct (URL, IntervalSeconds, TimeoutSeconds, Method)
- [ ] Implement `Validate()` method (URL format checking, IntervalSeconds >= 10, TimeoutSeconds >= 1)
- [ ] Define `MonitorResponse` and `StatsResponse` JSON structures
- **DoD:** `Validate()` returns descriptive error for invalid URLs or interval < 10

## [ ] Task 16: Implement Create Monitor API Handler
- **Target File:** `internal/api/handlers/monitor.go`
- [ ] Create `CreateMonitorHandler(httpWriter, httpRequest)`
- [ ] Parse JSON body into `CreateMonitorRequest` DTO and validate
- [ ] Map DTO to `model.Monitor` with newly generated UUID and timestamps
- [ ] Call `MonitorRepository.Create` and return `201 Created` with JSON payload
- **DoD:** `POST /api/v1/monitors` with valid JSON creates record in Postgres and returns 201 Created

## [ ] Task 17: Implement Monitor Stats & History Handler
- **Target File:** `internal/api/handlers/stats.go`
- [ ] Create `GetMonitorStatsHandler(httpWriter, httpRequest)`
- [ ] Extract `monitor_id` UUID from path parameter
- [ ] Query last 100 `CheckResult` records via repository
- [ ] Calculate aggregate metrics: Uptime percentage `(IsUp count / Total) * 100` and Average Response Time
- [ ] Return `200 OK` with JSON array of results and summary statistics
- **DoD:** `GET /api/v1/monitors/{id}/stats` returns valid JSON stats and history array

## [ ] Task 18: Register API Routes & Server Graceful Shutdown
- **Target File:** `cmd/api/main.go`
- [ ] Register routes `POST /api/v1/monitors` and `GET /api/v1/monitors/{id}/stats`
- [ ] Wire DB connection and repositories to API handlers
- [ ] Configure `http.Server` with timeouts and graceful shutdown handler
- **DoD:** Full API server starts up, serves endpoints, and shuts down cleanly on SIGINT
