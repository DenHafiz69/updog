package model

import "time"

const (
	MonitorTypePing = "ping"
	MonitorTypePush = "push"

	StatusUp       = "up"
	StatusDown     = "down"
	StatusDegraded = "degraded"
	StatusPending  = "pending"

	AlertTypeWebhook  = "webhook"
	AlertTypeDiscord  = "discord"
	AlertTypeSlack    = "slack"
	AlertTypeTelegram = "telegram"

	EventDown      = "down"
	EventUp        = "up"
	EventThreshold = "threshold"
)

type Monitor struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"` // "ping" or "push"
	URL             string     `json:"url"`
	Method          string     `json:"method"`
	Keyword         string     `json:"keyword"`
	IntervalSeconds int        `json:"interval_seconds"`
	TimeoutSeconds  int        `json:"timeout_seconds"`
	GraceSeconds    int        `json:"grace_seconds"`
	PushToken       string     `json:"push_token,omitempty"`
	IsActive        bool       `json:"is_active"`
	CPUThreshold    float64    `json:"cpu_threshold"`
	RAMThreshold    float64    `json:"ram_threshold"`
	DiskThreshold   float64    `json:"disk_threshold"`
	Status          string     `json:"status"`
	LastCheckedAt   *time.Time `json:"last_checked_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type CheckResult struct {
	ID             int64     `json:"id"`
	MonitorID      string    `json:"monitor_id"`
	Status         string    `json:"status"`
	StatusCode     int       `json:"status_code"`
	ResponseTimeMs int64     `json:"response_time_ms"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	CPUPct         *float64  `json:"cpu_pct,omitempty"`
	RAMPct         *float64  `json:"ram_pct,omitempty"`
	RAMUsedMB      *float64  `json:"ram_used_mb,omitempty"`
	RAMTotalMB     *float64  `json:"ram_total_mb,omitempty"`
	DiskPct        *float64  `json:"disk_pct,omitempty"`
	LoadAvg        *float64  `json:"load_avg,omitempty"`
	UptimeSec      *int64    `json:"uptime_sec,omitempty"`
	AppVersion     string    `json:"app_version,omitempty"`
	MetaJSON       string    `json:"meta_json,omitempty"`
	CheckedAt      time.Time `json:"checked_at"`
}

type MonitorOverview struct {
	Monitor
	Uptime24h   float64       `json:"uptime_24h"`
	AvgLatency  float64       `json:"avg_latency_ms"`
	LatestCheck *CheckResult  `json:"latest_check,omitempty"`
	RecentBars  []CheckResult `json:"recent_bars"`
}

type MonitorHistory struct {
	Monitor     Monitor       `json:"monitor"`
	UptimePct   float64       `json:"uptime_pct"`
	AvgLatency  float64       `json:"avg_latency_ms"`
	TotalChecks int           `json:"total_checks"`
	Checks      []CheckResult `json:"checks"`
}

type HeartbeatPayload struct {
	Status         string         `json:"status,omitempty"` // optional: "up", "degraded", "down"
	Message        string         `json:"message,omitempty"`
	ResponseTimeMs int64          `json:"response_time_ms,omitempty"`
	CPUPct         *float64       `json:"cpu_pct,omitempty"`
	RAMPct         *float64       `json:"ram_pct,omitempty"`
	RAMUsedMB      *float64       `json:"ram_used_mb,omitempty"`
	RAMTotalMB     *float64       `json:"ram_total_mb,omitempty"`
	DiskPct        *float64       `json:"disk_pct,omitempty"`
	LoadAvg        *float64       `json:"load_avg,omitempty"`
	UptimeSec      *int64         `json:"uptime_sec,omitempty"`
	AppVersion     string         `json:"version,omitempty"`
	Meta           map[string]any `json:"meta,omitempty"`
}

type AlertChannel struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"` // webhook, discord, slack, telegram
	TargetURL       string    `json:"target_url"`
	ConfigJSON      string    `json:"config_json,omitempty"` // e.g. {"chat_id":"..."} for telegram
	Enabled         bool      `json:"enabled"`
	CooldownSeconds int       `json:"cooldown_seconds"`
	CreatedAt       time.Time `json:"created_at"`
}

type AlertLog struct {
	ID          int64     `json:"id"`
	MonitorID   string    `json:"monitor_id"`
	MonitorName string    `json:"monitor_name,omitempty"`
	ChannelID   string    `json:"channel_id"`
	ChannelName string    `json:"channel_name,omitempty"`
	EventType   string    `json:"event_type"`
	Message     string    `json:"message"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
	SentAt      time.Time `json:"sent_at"`
}
