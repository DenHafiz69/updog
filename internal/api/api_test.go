package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DenHafiz69/updog/internal/alerts"
	"github.com/DenHafiz69/updog/internal/api"
	"github.com/DenHafiz69/updog/internal/checker"
	"github.com/DenHafiz69/updog/internal/model"
	"github.com/DenHafiz69/updog/internal/scheduler"
	"github.com/DenHafiz69/updog/internal/sse"
	"github.com/DenHafiz69/updog/internal/store"
	"github.com/DenHafiz69/updog/web"
)

func setupTestServer(t *testing.T, password string) http.Handler {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	hub := sse.NewHub()
	chk := checker.New()
	disp := alerts.NewDispatcher(st)
	sched := scheduler.New(st, chk, disp, hub, 30)
	srv := api.NewServer(st, sched, disp, hub, web.FS(), password)
	return srv.Routes()
}

func TestPingAndPushTelemetryFlow(t *testing.T) {
	handler := setupTestServer(t, "")

	// 1. Target HTTP server for active ping test
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("service-healthy-ok"))
	}))
	defer target.Close()

	// 2. Create Ping monitor
	pingBody := `{"name":"Test API","type":"ping","url":"` + target.URL + `","keyword":"service-healthy","interval_seconds":30}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitors", strings.NewReader(pingBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ping monitor: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var pingMon model.Monitor
	_ = json.Unmarshal(rec.Body.Bytes(), &pingMon)

	// Trigger immediate check
	req = httptest.NewRequest(http.MethodPost, "/api/v1/monitors/"+pingMon.ID+"/check", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("manual ping check failed: %d %s", rec.Code, rec.Body.String())
	}
	var pingRes model.CheckResult
	_ = json.Unmarshal(rec.Body.Bytes(), &pingRes)
	if pingRes.Status != model.StatusUp || pingRes.StatusCode != 200 {
		t.Fatalf("expected ping UP/200, got %+v", pingRes)
	}

	// 3. Create Push monitor with CPU threshold 80%
	pushBody := `{"name":"App Node","type":"push","interval_seconds":60,"grace_seconds":30,"cpu_threshold":80,"ram_threshold":90}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/monitors", strings.NewReader(pushBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create push monitor: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var pushMon model.Monitor
	_ = json.Unmarshal(rec.Body.Bytes(), &pushMon)
	if pushMon.PushToken == "" {
		t.Fatalf("expected generated push_token")
	}

	// 4. Verify snippet generation
	for _, lang := range []string{"bash", "node", "python", "go"} {
		req = httptest.NewRequest(http.MethodGet, "/api/v1/monitors/"+pushMon.ID+"/snippet?lang="+lang, nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("snippet %s failed: %d", lang, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), pushMon.PushToken) {
			t.Fatalf("snippet %s missing push token", lang)
		}
	}

	// 5. Send normal heartbeat -> UP
	hb := bytes.NewBufferString(`{"cpu_pct":35.5,"ram_used_mb":512,"ram_total_mb":2048,"disk_pct":40.0,"version":"1.0.0"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat/"+pushMon.PushToken, hb)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("heartbeat failed: %d %s", rec.Code, rec.Body.String())
	}
	var hbResp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &hbResp)
	if hbResp["status"] != model.StatusUp {
		t.Fatalf("expected status up, got %v", hbResp["status"])
	}

	// 6. Send high-CPU heartbeat -> DEGRADED
	hbHigh := bytes.NewBufferString(`{"cpu_pct":92.4,"ram_pct":55.0}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat/"+pushMon.PushToken, hbHigh)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("high-cpu heartbeat failed: %d", rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &hbResp)
	if hbResp["status"] != model.StatusDegraded {
		t.Fatalf("expected status degraded on CPU breach, got %v", hbResp["status"])
	}

	// 7. Embedded UI served at /
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Updog") {
		t.Fatalf("expected embedded UI at /, got %d", rec.Code)
	}
}

func TestPasswordAuthAndHardening(t *testing.T) {
	// Test with a very long 200+ character password
	longPassword := strings.Repeat("super-long-passphrase-segment-", 8) + "end!"
	handler := setupTestServer(t, longPassword)

	// 1. Unauthenticated monitor list -> 401
	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthenticated, got %d", rec.Code)
	}

	// 2. Wrong password -> 401
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"wrong"}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong password, got %d", rec.Code)
	}

	// 3. Correct long password -> 200 + HttpOnly cookie
	loginBody, _ := json.Marshal(map[string]string{"password": longPassword})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid login, got %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || !cookies[0].HttpOnly {
		t.Fatalf("expected HttpOnly session cookie, got %+v", cookies)
	}
	sessionCookie := cookies[0]

	// 4. Create push monitor using session cookie
	req = httptest.NewRequest(http.MethodPost, "/api/v1/monitors", strings.NewReader(`{"name":"Remote App","type":"push","interval_seconds":60}`))
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 with session cookie, got %d", rec.Code)
	}
	var pushMon model.Monitor
	_ = json.Unmarshal(rec.Body.Bytes(), &pushMon)

	// 5. Heartbeat endpoint works WITHOUT session cookie (uses push token)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat/"+pushMon.PushToken, strings.NewReader(`{"cpu_pct":12.3}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected heartbeat 200 without auth cookie, got %d", rec.Code)
	}

	// 6. Bearer auth works & alert target_url is masked in GET /api/v1/alerts
	req = httptest.NewRequest(http.MethodPost, "/api/v1/alerts", strings.NewReader(`{"name":"Discord","type":"discord","target_url":"https://discord.com/api/webhooks/123456/secret-token-xyz"}`))
	req.Header.Set("Authorization", "Bearer "+longPassword)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating alert with bearer auth, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret-token") {
		t.Fatalf("expected webhook secret to be masked, got %s", rec.Body.String())
	}
}
