package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/DenHafiz69/updog/internal/model"
	"github.com/DenHafiz69/updog/internal/store"
)

type Dispatcher struct {
	store  *store.Store
	client *http.Client
}

func NewDispatcher(s *store.Store) *Dispatcher {
	return &Dispatcher{
		store: s,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (d *Dispatcher) Notify(ctx context.Context, m model.Monitor, eventType, message string) {
	channels, err := d.store.ListAlertChannels(ctx)
	if err != nil {
		return
	}

	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}

		// Cooldown check (skip cooldown for recovery "up" events)
		if eventType != model.EventUp && ch.CooldownSeconds > 0 {
			lastSent, err := d.store.LastAlertSentAt(ctx, m.ID, ch.ID, eventType)
			if err == nil && lastSent != nil && time.Since(*lastSent) < time.Duration(ch.CooldownSeconds)*time.Second {
				continue
			}
		}

		go func(channel model.AlertChannel) {
			sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 12*time.Second)
			defer cancel()

			err := d.send(sendCtx, channel, m, eventType, message)
			logEntry := &model.AlertLog{
				MonitorID: m.ID,
				ChannelID: channel.ID,
				EventType: eventType,
				Message:   message,
				Success:   err == nil,
				SentAt:    time.Now().UTC(),
			}
			if err != nil {
				logEntry.Error = err.Error()
			}
			_ = d.store.RecordAlertLog(sendCtx, logEntry)
		}(ch)
	}
}

func (d *Dispatcher) SendTest(ctx context.Context, ch model.AlertChannel) error {
	dummy := model.Monitor{
		ID:   "test",
		Name: "Updog Test Monitor",
		Type: model.MonitorTypePing,
		URL:  "https://example.com",
	}
	return d.send(ctx, ch, dummy, model.EventUp, "Test alert from Updog! Your alert channel is configured properly.")
}

func (d *Dispatcher) send(ctx context.Context, ch model.AlertChannel, m model.Monitor, eventType, message string) error {
	var endpoint string
	var body []byte
	var err error

	title, color := formatEventMeta(m.Name, eventType)

	switch ch.Type {
	case model.AlertTypeDiscord:
		endpoint = ch.TargetURL
		payload := map[string]any{
			"embeds": []map[string]any{
				{
					"title":       title,
					"description": message,
					"color":       color,
					"timestamp":   time.Now().UTC().Format(time.RFC3339),
					"footer":      map[string]string{"text": "Updog Monitor"},
				},
			},
		}
		body, err = json.Marshal(payload)

	case model.AlertTypeSlack:
		endpoint = ch.TargetURL
		hexColor := fmt.Sprintf("#%06x", color)
		payload := map[string]any{
			"attachments": []map[string]any{
				{
					"color": hexColor,
					"title": title,
					"text":  message,
					"ts":    time.Now().Unix(),
				},
			},
		}
		body, err = json.Marshal(payload)

	case model.AlertTypeTelegram:
		var cfg struct {
			ChatID string `json:"chat_id"`
		}
		_ = json.Unmarshal([]byte(ch.ConfigJSON), &cfg)
		if cfg.ChatID == "" {
			return fmt.Errorf("telegram config_json missing chat_id")
		}
		token := strings.TrimSpace(ch.TargetURL)
		if strings.HasPrefix(token, "http") {
			endpoint = token
		} else {
			endpoint = fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
		}
		payload := map[string]any{
			"chat_id":    cfg.ChatID,
			"text":       fmt.Sprintf("%s\n%s", title, message),
			"parse_mode": "HTML",
		}
		body, err = json.Marshal(payload)

	default: // generic webhook
		endpoint = ch.TargetURL
		payload := map[string]any{
			"source":     "updog",
			"event":      eventType,
			"title":      title,
			"message":    message,
			"monitor_id": m.ID,
			"monitor":    m.Name,
			"type":       m.Type,
			"url":        m.URL,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		}
		body, err = json.Marshal(payload)
	}

	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Updog-Alerts/1.0")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func formatEventMeta(name, eventType string) (string, int) {
	switch eventType {
	case model.EventDown:
		return fmt.Sprintf("🔴 DOWN: %s", name), 0xEF4444
	case model.EventThreshold:
		return fmt.Sprintf("🟡 THRESHOLD: %s", name), 0xF59E0B
	default:
		return fmt.Sprintf("🟢 RECOVERED: %s", name), 0x10B981
	}
}
