package checker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/DenHafiz69/updog/internal/model"
)

type Checker struct {
	client *http.Client
}

func New() *Checker {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     60 * time.Second,
	}
	return &Checker{
		client: &http.Client{
			Transport: transport,
		},
	}
}

func (c *Checker) ExecutePing(ctx context.Context, m model.Monitor) model.CheckResult {
	res := model.CheckResult{
		MonitorID: m.ID,
		CheckedAt: time.Now().UTC(),
	}

	timeout := time.Duration(m.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	method := m.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(reqCtx, method, m.URL, nil)
	if err != nil {
		res.Status = model.StatusDown
		res.ErrorMessage = fmt.Sprintf("invalid request: %v", err)
		return res
	}
	req.Header.Set("User-Agent", "Updog-Monitor/1.0")

	start := time.Now()
	resp, err := c.client.Do(req)
	res.ResponseTimeMs = time.Since(start).Milliseconds()

	if err != nil {
		res.Status = model.StatusDown
		res.ErrorMessage = err.Error()
		return res
	}
	defer resp.Body.Close()

	res.StatusCode = resp.StatusCode

	var bodyStr string
	if m.Keyword != "" {
		limited := io.LimitReader(resp.Body, 1<<20) // 1MB cap
		b, readErr := io.ReadAll(limited)
		_, _ = io.Copy(io.Discard, resp.Body)
		if readErr != nil {
			res.Status = model.StatusDown
			res.ErrorMessage = fmt.Sprintf("body read error: %v", readErr)
			return res
		}
		bodyStr = string(b)
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		res.Status = model.StatusDown
		res.ErrorMessage = fmt.Sprintf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		return res
	}

	if m.Keyword != "" && !strings.Contains(bodyStr, m.Keyword) {
		res.Status = model.StatusDown
		res.ErrorMessage = fmt.Sprintf("keyword %q not found in response", m.Keyword)
		return res
	}

	res.Status = model.StatusUp
	return res
}
