package snippets

import (
	"fmt"

	"github.com/DenHafiz69/updog/internal/model"
)

func Generate(lang, baseURL string, m model.Monitor) (string, string) {
	endpoint := fmt.Sprintf("%s/api/v1/heartbeat/%s", baseURL, m.PushToken)
	interval := m.IntervalSeconds
	if interval <= 0 {
		interval = 60
	}

	switch lang {
	case "node":
		return "javascript", fmt.Sprintf(`// Updog Push Heartbeat + Telemetry (Zero dependencies, Node.js 18+)
// Call startUpdogTracker() once when your app boots.
import os from 'node:os';

const UPDOG_URL = %q;
const INTERVAL_MS = %d * 1000;

function sampleCPU() {
  const cpus = os.cpus();
  let idle = 0, total = 0;
  for (const c of cpus) {
    for (const k in c.times) total += c.times[k];
    idle += c.times.idle;
  }
  return { idle, total };
}

let prev = sampleCPU();

export function startUpdogTracker(version = process.env.APP_VERSION || '') {
  const send = async () => {
    const cur = sampleCPU();
    const idleDiff = cur.idle - prev.idle;
    const totalDiff = cur.total - prev.total;
    prev = cur;
    const cpuPct = totalDiff > 0 ? +((1 - idleDiff / totalDiff) * 100).toFixed(1) : 0;

    const totalMem = os.totalmem();
    const usedMem = totalMem - os.freemem();
    const ramPct = +((usedMem / totalMem) * 100).toFixed(1);

    try {
      await fetch(UPDOG_URL, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          cpu_pct: cpuPct,
          ram_pct: ramPct,
          ram_used_mb: +(usedMem / 1048576).toFixed(1),
          ram_total_mb: +(totalMem / 1048576).toFixed(1),
          load_avg: +os.loadavg()[0].toFixed(2),
          uptime_sec: Math.floor(process.uptime()),
          version,
        }),
      });
    } catch {
      // Never crash host app if Updog is unreachable
    }
  };
  send();
  return setInterval(send, INTERVAL_MS);
}
`, endpoint, interval)

	case "python":
		return "python", fmt.Sprintf(`# Updog Push Heartbeat + Telemetry (Zero dependencies, Python 3)
# Call start_updog_tracker() when your app starts.
import json, os, shutil, threading, time, urllib.request

UPDOG_URL = %q
INTERVAL_SEC = %d

def _read_linux_metrics():
    metrics = {}
    try:
        with open("/proc/meminfo") as f:
            info = dict(line.split(":", 1) for line in f)
        total_kb = int(info["MemTotal"].split()[0])
        avail_kb = int(info["MemAvailable"].split()[0])
        used_kb = total_kb - avail_kb
        metrics["ram_pct"] = round(used_kb / total_kb * 100, 1)
        metrics["ram_used_mb"] = round(used_kb / 1024, 1)
        metrics["ram_total_mb"] = round(total_kb / 1024, 1)
    except Exception:
        pass
    try:
        disk = shutil.disk_usage("/")
        metrics["disk_pct"] = round(disk.used / disk.total * 100, 1)
    except Exception:
        pass
    try:
        metrics["load_avg"] = round(os.getloadavg()[0], 2)
    except Exception:
        pass
    return metrics

def start_updog_tracker(version=""):
    start_ts = time.time()
    def loop():
        while True:
            payload = _read_linux_metrics()
            payload["uptime_sec"] = int(time.time() - start_ts)
            if version:
                payload["version"] = version
            req = urllib.request.Request(
                UPDOG_URL,
                data=json.dumps(payload).encode(),
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            try:
                urllib.request.urlopen(req, timeout=5).read()
            except Exception:
                pass
            time.sleep(INTERVAL_SEC)
    t = threading.Thread(target=loop, daemon=True)
    t.start()
    return t
`, endpoint, interval)

	case "go":
		return "go", fmt.Sprintf(`// Updog Push Heartbeat + Telemetry (Go stdlib only)
// Call go StartUpdogTracker(ctx, "1.0.0") in main().
package updogtracker

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

const (
	updogURL      = %q
	updogInterval = %d * time.Second
)

func StartUpdogTracker(ctx context.Context, version string) {
	client := &http.Client{Timeout: 5 * time.Second}
	started := time.Now()

	send := func() {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		payload := map[string]any{
			"ram_used_mb": float64(ms.Alloc) / 1048576.0,
			"uptime_sec":  int64(time.Since(started).Seconds()),
			"version":     version,
			"meta": map[string]any{
				"goroutines": runtime.NumGoroutine(),
			},
		}
		b, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, updogURL, bytes.NewReader(b))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
		}
	}

	send()
	ticker := time.NewTicker(updogInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			send()
		}
	}
}
`, endpoint, interval)

	default: // bash
		return "bash", fmt.Sprintf(`#!/usr/bin/env bash
# Updog Push Heartbeat + Telemetry (Linux Bash + curl)
# Run in cron (e.g. every minute) or inside a while loop:
#   while true; do ./updog-ping.sh; sleep %d; done

UPDOG_URL=%q

CPU=$(awk -v a="$(awk '/^cpu /{print $2+$4,$2+$4+$5}' /proc/stat; sleep 0.5)" \
  '/^cpu /{split(a,x); printf "%%.1f", (($2+$4-x[1])/($2+$4+$5-x[2]))*100}' /proc/stat 2>/dev/null || echo 0)
read -r RAM_USED RAM_TOTAL RAM_PCT <<< "$(free -m | awk '/Mem:/{printf "%%.1f %%.1f %%.1f", $3, $2, ($3/$2)*100}')"
DISK_PCT=$(df -P / | awk 'NR==2{gsub("%%",""); print $5}')
LOAD=$(awk '{print $1}' /proc/loadavg)
UPTIME=$(awk '{print int($1)}' /proc/uptime)

curl -fsS -m 5 -X POST "$UPDOG_URL" \
  -H "Content-Type: application/json" \
  -d "{\"cpu_pct\":${CPU:-0},\"ram_pct\":${RAM_PCT:-0},\"ram_used_mb\":${RAM_USED:-0},\"ram_total_mb\":${RAM_TOTAL:-0},\"disk_pct\":${DISK_PCT:-0},\"load_avg\":${LOAD:-0},\"uptime_sec\":${UPTIME:-0}}"
`, interval, endpoint)
	}
}
