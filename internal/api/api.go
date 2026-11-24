package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DenHafiz69/updog/internal/alerts"
	"github.com/DenHafiz69/updog/internal/model"
	"github.com/DenHafiz69/updog/internal/scheduler"
	"github.com/DenHafiz69/updog/internal/snippets"
	"github.com/DenHafiz69/updog/internal/sse"
	"github.com/DenHafiz69/updog/internal/store"
)

type Server struct {
	store       *store.Store
	scheduler   *scheduler.Scheduler
	alerts      *alerts.Dispatcher
	hub         *sse.Hub
	webFS       fs.FS
	auth        *authManager
	loginLimit  *rateLimiter
	hbLimit     *rateLimiter
}

func NewServer(s *store.Store, sched *scheduler.Scheduler, a *alerts.Dispatcher, hub *sse.Hub, webFS fs.FS, password string) *Server {
	return &Server{
		store:      s,
		scheduler:  sched,
		alerts:     a,
		hub:        hub,
		webFS:      webFS,
		auth:       newAuthManager(password),
		loginLimit: newRateLimiter(10, time.Minute),
		hbLimit:    newRateLimiter(60, time.Minute),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Auth endpoints
	mux.HandleFunc("GET /api/v1/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)

	// Push Heartbeat (token-authenticated; open for remote apps & scripts)
	mux.HandleFunc("POST /api/v1/heartbeat/{token}", s.withHeartbeatCORS(s.handleHeartbeat))
	mux.HandleFunc("GET /api/v1/heartbeat/{token}", s.withHeartbeatCORS(s.handleHeartbeat))
	mux.HandleFunc("OPTIONS /api/v1/heartbeat/{token}", s.withHeartbeatCORS(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	// Protected SSE live stream
	mux.HandleFunc("GET /api/v1/events", s.requireAuth(s.handleSSE))

	// Protected Monitors
	mux.HandleFunc("GET /api/v1/monitors", s.requireAuth(s.handleListMonitors))
	mux.HandleFunc("POST /api/v1/monitors", s.requireAuth(s.handleCreateMonitor))
	mux.HandleFunc("PUT /api/v1/monitors/{id}", s.requireAuth(s.handleUpdateMonitor))
	mux.HandleFunc("DELETE /api/v1/monitors/{id}", s.requireAuth(s.handleDeleteMonitor))
	mux.HandleFunc("POST /api/v1/monitors/{id}/check", s.requireAuth(s.handleCheckNow))
	mux.HandleFunc("GET /api/v1/monitors/{id}/history", s.requireAuth(s.handleMonitorHistory))
	mux.HandleFunc("GET /api/v1/monitors/{id}/snippet", s.requireAuth(s.handleMonitorSnippet))

	// Protected Alert Channels & Logs
	mux.HandleFunc("GET /api/v1/alerts", s.requireAuth(s.handleListAlerts))
	mux.HandleFunc("POST /api/v1/alerts", s.requireAuth(s.handleCreateAlert))
	mux.HandleFunc("PUT /api/v1/alerts/{id}", s.requireAuth(s.handleUpdateAlert))
	mux.HandleFunc("DELETE /api/v1/alerts/{id}", s.requireAuth(s.handleDeleteAlert))
	mux.HandleFunc("POST /api/v1/alerts/{id}/test", s.requireAuth(s.handleTestAlert))
	mux.HandleFunc("GET /api/v1/alerts/logs", s.requireAuth(s.handleListAlertLogs))

	// Embedded Web UI
	if s.webFS != nil {
		fileServer := http.FileServer(http.FS(s.webFS))
		mux.Handle("GET /", fileServer)
	}

	return withSecurityHeaders(mux)
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.isAuthenticated(r) {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"auth_required": s.auth.enabled,
		"authenticated": s.auth.isAuthenticated(r),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.auth.enabled {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if !s.loginLimit.Allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "too many login attempts, try again in a minute")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	if !s.auth.verifyPassword(req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}

	token := s.auth.createSession()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		s.auth.deleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.hub.Subscribe()
	defer s.hub.Unsubscribe(ch)

	_, _ = fmt.Fprintf(w, "data: {\"type\":\"connected\"}\n\n")
	flusher.Flush()

	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-keepalive.C:
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleListMonitors(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListMonitorOverviews(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateMonitor(w http.ResponseWriter, r *http.Request) {
	var req model.Monitor
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.URL = strings.TrimSpace(req.URL)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Type == "" {
		req.Type = model.MonitorTypePing
	}
	if req.Type != model.MonitorTypePing && req.Type != model.MonitorTypePush {
		writeError(w, http.StatusBadRequest, "type must be 'ping' or 'push'")
		return
	}
	if req.Type == model.MonitorTypePing {
		u, err := url.ParseRequestURI(req.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			writeError(w, http.StatusBadRequest, "valid http/https url is required for ping monitors")
			return
		}
	}
	req.IsActive = true

	if err := s.store.CreateMonitor(r.Context(), &req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.hub.Broadcast("monitor_created", req)
	if req.Type == model.MonitorTypePing {
		go s.scheduler.RunPingNow(r.Context(), req)
	}
	writeJSON(w, http.StatusCreated, req)
}

func (s *Server) handleUpdateMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.store.GetMonitor(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req model.Monitor
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if strings.TrimSpace(req.Name) != "" {
		existing.Name = strings.TrimSpace(req.Name)
	}
	if existing.Type == model.MonitorTypePing && strings.TrimSpace(req.URL) != "" {
		existing.URL = strings.TrimSpace(req.URL)
	}
	if req.Method != "" {
		existing.Method = req.Method
	}
	existing.Keyword = req.Keyword
	if req.IntervalSeconds >= 5 {
		existing.IntervalSeconds = req.IntervalSeconds
	}
	if req.TimeoutSeconds >= 1 {
		existing.TimeoutSeconds = req.TimeoutSeconds
	}
	if req.GraceSeconds >= 5 {
		existing.GraceSeconds = req.GraceSeconds
	}
	existing.IsActive = req.IsActive
	existing.CPUThreshold = req.CPUThreshold
	existing.RAMThreshold = req.RAMThreshold
	existing.DiskThreshold = req.DiskThreshold

	if err := s.store.UpdateMonitor(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.hub.Broadcast("monitor_updated", existing)
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteMonitor(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.hub.Broadcast("monitor_deleted", map[string]string{"id": id})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCheckNow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := s.store.GetMonitor(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if m.Type != model.MonitorTypePing {
		writeError(w, http.StatusBadRequest, "manual check only applies to ping monitors")
		return
	}
	res := s.scheduler.RunPingNow(r.Context(), *m)
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleMonitorHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := s.store.GetMonitor(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	limit := 120
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	checks, err := s.store.GetRecentChecks(r.Context(), id, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var upCount int
	var latSum int64
	var latCount int
	for _, c := range checks {
		if c.Status != model.StatusDown {
			upCount++
		}
		if c.ResponseTimeMs > 0 {
			latSum += c.ResponseTimeMs
			latCount++
		}
	}
	var uptimePct, avgLat float64
	if len(checks) > 0 {
		uptimePct = (float64(upCount) / float64(len(checks))) * 100
	}
	if latCount > 0 {
		avgLat = float64(latSum) / float64(latCount)
	}

	writeJSON(w, http.StatusOK, model.MonitorHistory{
		Monitor:     *m,
		UptimePct:   uptimePct,
		AvgLatency:  avgLat,
		TotalChecks: len(checks),
		Checks:      checks,
	})
}

func (s *Server) handleMonitorSnippet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := s.store.GetMonitor(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if m.Type != model.MonitorTypePush {
		writeError(w, http.StatusBadRequest, "snippets are only available for push monitors")
		return
	}

	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "bash"
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)
	resolvedLang, code := snippets.Generate(lang, baseURL, *m)

	writeJSON(w, http.StatusOK, map[string]string{
		"lang":     resolvedLang,
		"endpoint": fmt.Sprintf("%s/api/v1/heartbeat/%s", baseURL, m.PushToken),
		"code":     code,
	})
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !s.hbLimit.Allow(clientIP(r) + ":" + token) {
		writeError(w, http.StatusTooManyRequests, "heartbeat rate limit exceeded")
		return
	}
	m, err := s.store.GetMonitorByToken(r.Context(), token)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "unknown push token")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var payload model.HeartbeatPayload
	if r.Body != nil {
		bodyBytes, _ := io.ReadAll(io.LimitReader(r.Body, 512*1024))
		if len(strings.TrimSpace(string(bodyBytes))) > 0 {
			if err := json.Unmarshal(bodyBytes, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON telemetry payload")
				return
			}
		}
	}

	// Auto-derive RAM percentage if used/total provided
	if payload.RAMPct == nil && payload.RAMUsedMB != nil && payload.RAMTotalMB != nil && *payload.RAMTotalMB > 0 {
		pct := (*payload.RAMUsedMB / *payload.RAMTotalMB) * 100
		payload.RAMPct = &pct
	}

	var metaJSON string
	if len(payload.Meta) > 0 {
		if b, err := json.Marshal(payload.Meta); err == nil {
			metaJSON = string(b)
		}
	}

	status := model.StatusUp
	if payload.Status == model.StatusDown || payload.Status == model.StatusDegraded {
		status = payload.Status
	}

	res := model.CheckResult{
		MonitorID:      m.ID,
		Status:         status,
		StatusCode:     200,
		ResponseTimeMs: payload.ResponseTimeMs,
		ErrorMessage:   payload.Message,
		CPUPct:         payload.CPUPct,
		RAMPct:         payload.RAMPct,
		RAMUsedMB:      payload.RAMUsedMB,
		RAMTotalMB:     payload.RAMTotalMB,
		DiskPct:        payload.DiskPct,
		LoadAvg:        payload.LoadAvg,
		UptimeSec:      payload.UptimeSec,
		AppVersion:     payload.AppVersion,
		MetaJSON:       metaJSON,
		CheckedAt:      time.Now().UTC(),
	}

	if breaches := s.scheduler.EvaluateThresholds(*m, &res); len(breaches) > 0 && res.Status == model.StatusUp {
		res.Status = model.StatusDegraded
		if res.ErrorMessage == "" {
			res.ErrorMessage = "Thresholds exceeded: " + strings.Join(breaches, ", ")
		}
	}

	if err := s.store.RecordCheck(r.Context(), &res); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.scheduler.HandleTransition(r.Context(), *m, m.Status, res)
	s.hub.Broadcast("check", map[string]any{
		"monitor_id": m.ID,
		"result":     res,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"monitor_id": m.ID,
		"status":     res.Status,
	})
}

// --- Alerts ---

func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListAlertChannels(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []model.AlertChannel{}
	}
	for i := range list {
		list[i].TargetURL = maskSecretURL(list[i].TargetURL)
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateAlert(w http.ResponseWriter, r *http.Request) {
	var ch model.AlertChannel
	if err := json.NewDecoder(io.LimitReader(r.Body, 256*1024)).Decode(&ch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	ch.Name = strings.TrimSpace(ch.Name)
	ch.TargetURL = strings.TrimSpace(ch.TargetURL)
	if ch.Name == "" || ch.TargetURL == "" {
		writeError(w, http.StatusBadRequest, "name and target_url are required")
		return
	}
	switch ch.Type {
	case model.AlertTypeWebhook, model.AlertTypeDiscord, model.AlertTypeSlack, model.AlertTypeTelegram:
	default:
		writeError(w, http.StatusBadRequest, "type must be webhook, discord, slack, or telegram")
		return
	}
	ch.Enabled = true
	if err := s.store.CreateAlertChannel(r.Context(), &ch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ch.TargetURL = maskSecretURL(ch.TargetURL)
	writeJSON(w, http.StatusCreated, ch)
}

func (s *Server) handleUpdateAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.store.GetAlertChannel(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "alert channel not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req model.AlertChannel
	if err := json.NewDecoder(io.LimitReader(r.Body, 256*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	if req.Name != "" {
		existing.Name = strings.TrimSpace(req.Name)
	}
	if req.TargetURL != "" && !strings.Contains(req.TargetURL, "••••") {
		existing.TargetURL = strings.TrimSpace(req.TargetURL)
	}
	if req.Type != "" {
		existing.Type = req.Type
	}
	if req.ConfigJSON != "" {
		existing.ConfigJSON = req.ConfigJSON
	}
	if req.CooldownSeconds > 0 {
		existing.CooldownSeconds = req.CooldownSeconds
	}
	existing.Enabled = req.Enabled

	if err := s.store.UpdateAlertChannel(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	existing.TargetURL = maskSecretURL(existing.TargetURL)
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteAlertChannel(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ch, err := s.store.GetAlertChannel(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "alert channel not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.alerts.SendTest(r.Context(), *ch); err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("test failed: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleListAlertLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.store.ListAlertLogs(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if logs == nil {
		logs = []model.AlertLog{}
	}
	writeJSON(w, http.StatusOK, logs)
}

// --- HTTP helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) withHeartbeatCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		next(w, r)
	}
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
