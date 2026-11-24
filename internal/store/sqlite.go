package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/DenHafiz69/updog/internal/model"
)

type Store struct {
	db *sql.DB
}

func Open(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Single writer pool avoids SQLITE_BUSY under concurrent goroutines
	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("pragma %s: %w", p, err)
		}
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS monitors (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL DEFAULT 'ping',
	url TEXT NOT NULL DEFAULT '',
	method TEXT NOT NULL DEFAULT 'GET',
	keyword TEXT NOT NULL DEFAULT '',
	interval_seconds INTEGER NOT NULL DEFAULT 60,
	timeout_seconds INTEGER NOT NULL DEFAULT 10,
	grace_seconds INTEGER NOT NULL DEFAULT 30,
	push_token TEXT UNIQUE,
	is_active INTEGER NOT NULL DEFAULT 1,
	cpu_threshold REAL NOT NULL DEFAULT 0,
	ram_threshold REAL NOT NULL DEFAULT 0,
	disk_threshold REAL NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'pending',
	last_checked_at DATETIME,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS check_results (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	monitor_id TEXT NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
	status TEXT NOT NULL,
	status_code INTEGER NOT NULL DEFAULT 0,
	response_time_ms INTEGER NOT NULL DEFAULT 0,
	error_message TEXT NOT NULL DEFAULT '',
	cpu_pct REAL,
	ram_pct REAL,
	ram_used_mb REAL,
	ram_total_mb REAL,
	disk_pct REAL,
	load_avg REAL,
	uptime_sec INTEGER,
	app_version TEXT NOT NULL DEFAULT '',
	meta_json TEXT NOT NULL DEFAULT '',
	checked_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_check_results_monitor_time
	ON check_results(monitor_id, checked_at DESC);

CREATE TABLE IF NOT EXISTS alert_channels (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	target_url TEXT NOT NULL,
	config_json TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	cooldown_seconds INTEGER NOT NULL DEFAULT 300,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS alert_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	monitor_id TEXT NOT NULL,
	channel_id TEXT NOT NULL,
	event_type TEXT NOT NULL,
	message TEXT NOT NULL,
	success INTEGER NOT NULL DEFAULT 1,
	error TEXT NOT NULL DEFAULT '',
	sent_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alert_logs_lookup
	ON alert_logs(monitor_id, channel_id, event_type, sent_at DESC);
`
	_, err := s.db.Exec(schema)
	return err
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Store) CreateMonitor(ctx context.Context, m *model.Monitor) error {
	now := time.Now().UTC()
	if m.ID == "" {
		m.ID = randomHex(6)
	}
	if m.Type == "" {
		m.Type = model.MonitorTypePing
	}
	if m.Method == "" {
		m.Method = "GET"
	}
	if m.IntervalSeconds < 5 {
		m.IntervalSeconds = 60
	}
	if m.TimeoutSeconds < 1 {
		m.TimeoutSeconds = 10
	}
	if m.GraceSeconds < 5 {
		m.GraceSeconds = 30
	}
	var pushToken any
	if m.Type == model.MonitorTypePush {
		if m.PushToken == "" {
			m.PushToken = "upg_" + randomHex(16)
		}
		pushToken = m.PushToken
	}
	m.Status = model.StatusPending
	m.CreatedAt = now
	m.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO monitors (
			id, name, type, url, method, keyword,
			interval_seconds, timeout_seconds, grace_seconds,
			push_token, is_active, cpu_threshold, ram_threshold, disk_threshold,
			status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		m.ID, m.Name, m.Type, m.URL, m.Method, m.Keyword,
		m.IntervalSeconds, m.TimeoutSeconds, m.GraceSeconds,
		pushToken, boolToInt(m.IsActive), m.CPUThreshold, m.RAMThreshold, m.DiskThreshold,
		m.Status, m.CreatedAt, m.UpdatedAt,
	)
	return err
}

func (s *Store) UpdateMonitor(ctx context.Context, m *model.Monitor) error {
	m.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE monitors SET
			name = ?, url = ?, method = ?, keyword = ?,
			interval_seconds = ?, timeout_seconds = ?, grace_seconds = ?,
			is_active = ?, cpu_threshold = ?, ram_threshold = ?, disk_threshold = ?,
			updated_at = ?
		WHERE id = ?
	`,
		m.Name, m.URL, m.Method, m.Keyword,
		m.IntervalSeconds, m.TimeoutSeconds, m.GraceSeconds,
		boolToInt(m.IsActive), m.CPUThreshold, m.RAMThreshold, m.DiskThreshold,
		m.UpdatedAt, m.ID,
	)
	return err
}

func (s *Store) DeleteMonitor(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM monitors WHERE id = ?`, id)
	return err
}

func (s *Store) GetMonitor(ctx context.Context, id string) (*model.Monitor, error) {
	row := s.db.QueryRowContext(ctx, selectMonitorSQL+` WHERE id = ?`, id)
	m, err := scanMonitor(row)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) GetMonitorByToken(ctx context.Context, token string) (*model.Monitor, error) {
	row := s.db.QueryRowContext(ctx, selectMonitorSQL+` WHERE push_token = ?`, token)
	m, err := scanMonitor(row)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) ListMonitors(ctx context.Context) ([]model.Monitor, error) {
	rows, err := s.db.QueryContext(ctx, selectMonitorSQL+` ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Monitor
	for rows.Next() {
		m, err := scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *Store) ListActiveMonitors(ctx context.Context) ([]model.Monitor, error) {
	rows, err := s.db.QueryContext(ctx, selectMonitorSQL+` WHERE is_active = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Monitor
	for rows.Next() {
		m, err := scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *Store) RecordCheck(ctx context.Context, res *model.CheckResult) error {
	if res.CheckedAt.IsZero() {
		res.CheckedAt = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	r, err := tx.ExecContext(ctx, `
		INSERT INTO check_results (
			monitor_id, status, status_code, response_time_ms, error_message,
			cpu_pct, ram_pct, ram_used_mb, ram_total_mb, disk_pct,
			load_avg, uptime_sec, app_version, meta_json, checked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		res.MonitorID, res.Status, res.StatusCode, res.ResponseTimeMs, res.ErrorMessage,
		res.CPUPct, res.RAMPct, res.RAMUsedMB, res.RAMTotalMB, res.DiskPct,
		res.LoadAvg, res.UptimeSec, res.AppVersion, res.MetaJSON, res.CheckedAt,
	)
	if err != nil {
		return err
	}
	res.ID, _ = r.LastInsertId()

	_, err = tx.ExecContext(ctx, `
		UPDATE monitors SET status = ?, last_checked_at = ?, updated_at = ? WHERE id = ?
	`, res.Status, res.CheckedAt, res.CheckedAt, res.MonitorID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) GetRecentChecks(ctx context.Context, monitorID string, limit int) ([]model.CheckResult, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, monitor_id, status, status_code, response_time_ms, error_message,
			cpu_pct, ram_pct, ram_used_mb, ram_total_mb, disk_pct,
			load_avg, uptime_sec, app_version, meta_json, checked_at
		FROM check_results
		WHERE monitor_id = ?
		ORDER BY checked_at DESC
		LIMIT ?
	`, monitorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.CheckResult
	for rows.Next() {
		var c model.CheckResult
		var cpu, ram, ramUsed, ramTotal, disk, load sql.NullFloat64
		var uptime sql.NullInt64
		if err := rows.Scan(
			&c.ID, &c.MonitorID, &c.Status, &c.StatusCode, &c.ResponseTimeMs, &c.ErrorMessage,
			&cpu, &ram, &ramUsed, &ramTotal, &disk,
			&load, &uptime, &c.AppVersion, &c.MetaJSON, &c.CheckedAt,
		); err != nil {
			return nil, err
		}
		if cpu.Valid {
			v := cpu.Float64
			c.CPUPct = &v
		}
		if ram.Valid {
			v := ram.Float64
			c.RAMPct = &v
		}
		if ramUsed.Valid {
			v := ramUsed.Float64
			c.RAMUsedMB = &v
		}
		if ramTotal.Valid {
			v := ramTotal.Float64
			c.RAMTotalMB = &v
		}
		if disk.Valid {
			v := disk.Float64
			c.DiskPct = &v
		}
		if load.Valid {
			v := load.Float64
			c.LoadAvg = &v
		}
		if uptime.Valid {
			v := uptime.Int64
			c.UptimeSec = &v
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

func (s *Store) GetMonitorStats24h(ctx context.Context, monitorID string) (uptimePct float64, avgLatency float64, err error) {
	since := time.Now().UTC().Add(-24 * time.Hour)
	var total int
	var upCount int
	var avgLat sql.NullFloat64
	err = s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status != 'down' THEN 1 ELSE 0 END), 0),
			AVG(CASE WHEN response_time_ms > 0 THEN response_time_ms END)
		FROM check_results
		WHERE monitor_id = ? AND checked_at >= ?
	`, monitorID, since).Scan(&total, &upCount, &avgLat)
	if err != nil {
		return 0, 0, err
	}
	if total > 0 {
		uptimePct = (float64(upCount) / float64(total)) * 100
	}
	if avgLat.Valid {
		avgLatency = avgLat.Float64
	}
	return uptimePct, avgLatency, nil
}

func (s *Store) ListMonitorOverviews(ctx context.Context) ([]model.MonitorOverview, error) {
	monitors, err := s.ListMonitors(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.MonitorOverview, 0, len(monitors))
	for _, m := range monitors {
		uptime, avgLat, err := s.GetMonitorStats24h(ctx, m.ID)
		if err != nil {
			return nil, err
		}
		recent, err := s.GetRecentChecks(ctx, m.ID, 30)
		if err != nil {
			return nil, err
		}
		ov := model.MonitorOverview{
			Monitor:    m,
			Uptime24h:  uptime,
			AvgLatency: avgLat,
			RecentBars: recent,
		}
		if len(recent) > 0 {
			latest := recent[0]
			ov.LatestCheck = &latest
		}
		out = append(out, ov)
	}
	return out, nil
}

func (s *Store) PruneOldChecks(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	res, err := s.db.ExecContext(ctx, `DELETE FROM check_results WHERE checked_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- Alert Channels & Logs ---

func (s *Store) CreateAlertChannel(ctx context.Context, ch *model.AlertChannel) error {
	if ch.ID == "" {
		ch.ID = randomHex(6)
	}
	if ch.CooldownSeconds <= 0 {
		ch.CooldownSeconds = 300
	}
	ch.CreatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_channels (id, name, type, target_url, config_json, enabled, cooldown_seconds, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, ch.ID, ch.Name, ch.Type, ch.TargetURL, ch.ConfigJSON, boolToInt(ch.Enabled), ch.CooldownSeconds, ch.CreatedAt)
	return err
}

func (s *Store) UpdateAlertChannel(ctx context.Context, ch *model.AlertChannel) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE alert_channels SET name = ?, type = ?, target_url = ?, config_json = ?, enabled = ?, cooldown_seconds = ?
		WHERE id = ?
	`, ch.Name, ch.Type, ch.TargetURL, ch.ConfigJSON, boolToInt(ch.Enabled), ch.CooldownSeconds, ch.ID)
	return err
}

func (s *Store) DeleteAlertChannel(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM alert_channels WHERE id = ?`, id)
	return err
}

func (s *Store) GetAlertChannel(ctx context.Context, id string) (*model.AlertChannel, error) {
	var ch model.AlertChannel
	var enabled int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, type, target_url, config_json, enabled, cooldown_seconds, created_at
		FROM alert_channels WHERE id = ?
	`, id).Scan(&ch.ID, &ch.Name, &ch.Type, &ch.TargetURL, &ch.ConfigJSON, &enabled, &ch.CooldownSeconds, &ch.CreatedAt)
	if err != nil {
		return nil, err
	}
	ch.Enabled = enabled == 1
	return &ch, nil
}

func (s *Store) ListAlertChannels(ctx context.Context) ([]model.AlertChannel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, type, target_url, config_json, enabled, cooldown_seconds, created_at
		FROM alert_channels ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.AlertChannel
	for rows.Next() {
		var ch model.AlertChannel
		var enabled int
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Type, &ch.TargetURL, &ch.ConfigJSON, &enabled, &ch.CooldownSeconds, &ch.CreatedAt); err != nil {
			return nil, err
		}
		ch.Enabled = enabled == 1
		out = append(out, ch)
	}
	return out, rows.Err()
}

func (s *Store) LastAlertSentAt(ctx context.Context, monitorID, channelID, eventType string) (*time.Time, error) {
	var t time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT sent_at FROM alert_logs
		WHERE monitor_id = ? AND channel_id = ? AND event_type = ? AND success = 1
		ORDER BY sent_at DESC LIMIT 1
	`, monitorID, channelID, eventType).Scan(&t)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) RecordAlertLog(ctx context.Context, log *model.AlertLog) error {
	if log.SentAt.IsZero() {
		log.SentAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_logs (monitor_id, channel_id, event_type, message, success, error, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, log.MonitorID, log.ChannelID, log.EventType, log.Message, boolToInt(log.Success), log.Error, log.SentAt)
	return err
}

func (s *Store) ListAlertLogs(ctx context.Context, limit int) ([]model.AlertLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT l.id, l.monitor_id, COALESCE(m.name, l.monitor_id), l.channel_id, COALESCE(c.name, l.channel_id),
		       l.event_type, l.message, l.success, l.error, l.sent_at
		FROM alert_logs l
		LEFT JOIN monitors m ON m.id = l.monitor_id
		LEFT JOIN alert_channels c ON c.id = l.channel_id
		ORDER BY l.sent_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.AlertLog
	for rows.Next() {
		var l model.AlertLog
		var success int
		if err := rows.Scan(&l.ID, &l.MonitorID, &l.MonitorName, &l.ChannelID, &l.ChannelName,
			&l.EventType, &l.Message, &success, &l.Error, &l.SentAt); err != nil {
			return nil, err
		}
		l.Success = success == 1
		out = append(out, l)
	}
	return out, rows.Err()
}

// --- Helpers ---

const selectMonitorSQL = `
SELECT id, name, type, url, method, keyword,
	interval_seconds, timeout_seconds, grace_seconds,
	push_token, is_active, cpu_threshold, ram_threshold, disk_threshold,
	status, last_checked_at, created_at, updated_at
FROM monitors`

type scannable interface {
	Scan(dest ...any) error
}

func scanMonitor(row scannable) (model.Monitor, error) {
	var m model.Monitor
	var pushToken sql.NullString
	var isActive int
	var lastChecked sql.NullTime

	err := row.Scan(
		&m.ID, &m.Name, &m.Type, &m.URL, &m.Method, &m.Keyword,
		&m.IntervalSeconds, &m.TimeoutSeconds, &m.GraceSeconds,
		&pushToken, &isActive, &m.CPUThreshold, &m.RAMThreshold, &m.DiskThreshold,
		&m.Status, &lastChecked, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return m, err
	}
	if pushToken.Valid {
		m.PushToken = pushToken.String
	}
	m.IsActive = isActive == 1
	if lastChecked.Valid {
		t := lastChecked.Time
		m.LastCheckedAt = &t
	}
	return m, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
