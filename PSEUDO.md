# Updog - Architecture Blueprint & Pseudocode Guide

This file serves as the master blueprint for building **Updog** by hand.
Refer to this document when implementing components, functions, and workflows.

---

## 1. System Architecture Overview

Updog consists of two primary Go executables and a database layer:

┌─────────────────┐       ┌────────────────────────┐       ┌─────────────────────┐
│   Go REST API   │ ────> │  PostgreSQL Database   │ <──── │ Go Worker Scheduler │
│  (cmd/api/main) │       │ (Monitors & Check Logs)│       │(cmd/worker/main.go) │
└─────────────────┘       └────────────────────────┘       └─────────────────────┘
        ^                                                             │
        │                                                             ▼
┌─────────────────┐                                        ┌──────────────────────┐
│ User / Dashboard│                                        │   Target Website /   │
│  (HTTP Requests)│                                        │   Public API Endpoint│
└─────────────────┘                                        └──────────────────────┘

---

## 2. Core Data Structures (`internal/model/monitor.go`)

### Monitor Model
- **ID**: UUID
- **URL**: String
- **IntervalSeconds**: Integer (e.g., 60)
- **TimeoutSeconds**: Integer (e.g., 10)
- **Method**: String (e.g., "GET", "HEAD")
- **IsActive**: Boolean
- **CreatedAt**: Timestamp
- **UpdatedAt**: Timestamp

### CheckResult Model
- **ID**: BigInt / UUID
- **MonitorID**: UUID
- **IsUp**: Boolean
- **StatusCode**: Integer
- **ResponseTimeMs**: Integer / Duration
- **ErrorMessage**: String (Nullable)
- **CheckedAt**: Timestamp

---

## 3. Worker Pseudocode Logic (`internal/worker/`)

### A. HTTP Checker Function (`http.go`)

```text
FUNCTION ExecuteCheck(context, monitor) RETURNS CheckResult:
    1. Initialize CheckResult:
       - set MonitorID = monitor.ID
       - set CheckedAt = CurrentTimeInUTC()

    2. Create Context with Timeout using monitor.TimeoutSeconds:
       - ctx, cancel = ContextWithTimeout(context, TimeoutDuration)
       - ensure defer cancel() is called

    3. Construct HTTP Request:
       - req = NewRequestWithContext(ctx, monitor.Method, monitor.URL)
       - set req.Header["User-Agent"] = "Updog-Monitor/1.0"

    4. Start Timer:
       - startTime = CurrentTime()

    5. Send Request via HTTP Client:
       - response, err = httpClient.Do(req)
       - duration = ElapsedTimeSince(startTime)
       - set result.ResponseTimeMs = duration

    6. IF err IS NOT NULL:
       - set result.IsUp = FALSE
       - set result.ErrorMessage = err.Error()
       - Log WARN "Check failed with network error"
       - RETURN result

    7. Clean Up Response Body (Crucial for connection pooling):
       - Read and Discard response.Body
       - Close response.Body

    8. Evaluate Response:
       - set result.StatusCode = response.StatusCode
       - IF response.StatusCode >= 200 AND response.StatusCode < 400:
           - set result.IsUp = TRUE
         ELSE:
           - set result.IsUp = FALSE
           - set result.ErrorMessage = HTTPStatusText(response.StatusCode)

    9. Log INFO "Check executed successfully"
   10. RETURN result
END FUNCTION

```

---

### B. Worker Loop & Scheduler (`scheduler.go`)

```text
FUNCTION StartWorkerLoop(context, databaseRepository):
    1. Initialize ticker (e.g., tick every 10 seconds)

    2. LOOP indefinitely WHILE context is active:
       a. SELECT all active monitors FROM database WHERE IsActive = TRUE
       b. FOR EACH monitor IN monitors:
            - Check if it's time to run based on LastCheckedAt and IntervalSeconds
            - IF time to check:
                - Launch Go Routine (concurrent check):
                    1. result = ExecuteCheck(context, monitor)
                    2. SaveCheckResultToDB(databaseRepository, result)
                    3. Update Monitor's LastCheckedAt timestamp in DB
       c. WAIT for next ticker pulse
END FUNCTION

```

---

## 4. API Endpoint Handlers (`internal/api/handlers/`)

### POST /api/v1/monitors (Create Target)

```text
FUNCTION CreateMonitorHandler(httpWriter, httpRequest):
    1. Parse JSON body from httpRequest into CreateMonitorDTO
    2. Validate Input:
       - URL must be valid HTTP/HTTPS format
       - IntervalSeconds must be >= 10
    3. Construct Monitor Struct with new UUID and timestamps
    4. Save Monitor to PostgreSQL via Repository layer
    5. Return 201 Created with JSON payload of created Monitor
END FUNCTION

```

### GET /api/v1/monitors/{id}/stats (Fetch History)

```text
FUNCTION GetMonitorStatsHandler(httpWriter, httpRequest):
    1. Extract monitor_id from URL path parameters
    2. Query database for last 100 CheckResults for monitor_id ordered by CheckedAt DESC
    3. Calculate aggregate stats:
       - Uptime Percentage = (Count of IsUp=TRUE / Total Checks) * 100
       - Average Response Time
    4. Return 200 OK with JSON array of results + summary stats
END FUNCTION

```

---

## 5. PostgreSQL Schema Definition (`migrations/001_init.sql`)

```text
CREATE TABLE monitors (
    id UUID PRIMARY KEY,
    url TEXT NOT NULL,
    interval_seconds INT NOT NULL DEFAULT 60,
    timeout_seconds INT NOT NULL DEFAULT 10,
    method VARCHAR(10) NOT NULL DEFAULT 'GET',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE check_logs (
    id BIGSERIAL,
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    is_up BOOLEAN NOT NULL,
    status_code INT,
    response_time_ms INT NOT NULL,
    error_message TEXT,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (checked_at, id)
);

CREATE INDEX idx_check_logs_monitor_time ON check_logs (monitor_id, checked_at DESC);

```

---

## 6. Implementation Step-by-Step Checklist

Use this checklist to build Updog by hand, item by item:

* [ ] **Step 1:** Initialize Go module (`go mod init <repo>`) and folder structure.
* [ ] **Step 2:** Write `docker-compose.yml` for local PostgreSQL.
* [ ] **Step 3:** Implement structs in `internal/model/monitor.go`.
* [ ] **Step 4:** Write SQL schema in `migrations/` and run against local Postgres.
* [ ] **Step 5:** Write `ExecuteCheck()` in `internal/worker/http.go` and test against a live URL in `cmd/worker/main.go`.
* [ ] **Step 6:** Implement `pgx` repository layer to insert `CheckResult` into DB.
* [ ] **Step 7:** Build ticker loop in `cmd/worker/main.go` to schedule checks periodically.
* [ ] **Step 8:** Setup Chi router in `cmd/api/main.go` for `POST /monitors` and `GET /monitors`.
