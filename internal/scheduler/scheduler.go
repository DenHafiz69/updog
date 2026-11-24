package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/DenHafiz69/updog/internal/alerts"
	"github.com/DenHafiz69/updog/internal/checker"
	"github.com/DenHafiz69/updog/internal/model"
	"github.com/DenHafiz69/updog/internal/sse"
	"github.com/DenHafiz69/updog/internal/store"
)

type Scheduler struct {
	store         *store.Store
	checker       *checker.Checker
	alerts        *alerts.Dispatcher
	hub           *sse.Hub
	retentionDays int

	mu       sync.Mutex
	inFlight map[string]bool
	sem      chan struct{}
}

func New(s *store.Store, c *checker.Checker, a *alerts.Dispatcher, h *sse.Hub, retentionDays int) *Scheduler {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	return &Scheduler{
		store:         s,
		checker:       c,
		alerts:        a,
		hub:           h,
		retentionDays: retentionDays,
		inFlight:      make(map[string]bool),
		sem:           make(chan struct{}, 32),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	pruneTick := time.NewTicker(1 * time.Hour)
	defer tick.Stop()
	defer pruneTick.Stop()

	// Run one pass immediately on startup
	s.runPass(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.runPass(ctx)
		case <-pruneTick.C:
			if deleted, err := s.store.PruneOldChecks(ctx, s.retentionDays); err == nil && deleted > 0 {
				log.Printf("pruned %d old check results (>%dd)", deleted, s.retentionDays)
			}
		}
	}
}

func (s *Scheduler) runPass(ctx context.Context) {
	monitors, err := s.store.ListActiveMonitors(ctx)
	if err != nil {
		log.Printf("scheduler list monitors error: %v", err)
		return
	}

	now := time.Now().UTC()
	for _, m := range monitors {
		switch m.Type {
		case model.MonitorTypePing:
			if s.isDueForPing(m, now) {
				s.dispatchPing(ctx, m)
			}
		case model.MonitorTypePush:
			s.checkPushWatchdog(ctx, m, now)
		}
	}
}

func (s *Scheduler) isDueForPing(m model.Monitor, now time.Time) bool {
	if m.LastCheckedAt == nil {
		return true
	}
	return now.Sub(*m.LastCheckedAt) >= time.Duration(m.IntervalSeconds)*time.Second
}

func (s *Scheduler) dispatchPing(ctx context.Context, m model.Monitor) {
	s.mu.Lock()
	if s.inFlight[m.ID] {
		s.mu.Unlock()
		return
	}
	s.inFlight[m.ID] = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.inFlight, m.ID)
			s.mu.Unlock()
		}()

		select {
		case s.sem <- struct{}{}:
			defer func() { <-s.sem }()
		case <-ctx.Done():
			return
		}

		s.RunPingNow(ctx, m)
	}()
}

func (s *Scheduler) RunPingNow(ctx context.Context, m model.Monitor) model.CheckResult {
	res := s.checker.ExecutePing(ctx, m)
	if err := s.store.RecordCheck(ctx, &res); err != nil {
		log.Printf("record ping check error (%s): %v", m.Name, err)
		return res
	}

	s.HandleTransition(ctx, m, m.Status, res)
	s.hub.Broadcast("check", map[string]any{
		"monitor_id": m.ID,
		"result":     res,
	})
	return res
}

func (s *Scheduler) checkPushWatchdog(ctx context.Context, m model.Monitor, now time.Time) {
	if m.LastCheckedAt == nil {
		// Never received a heartbeat yet; stay pending until first heartbeat
		return
	}
	deadline := time.Duration(m.IntervalSeconds+m.GraceSeconds) * time.Second
	if now.Sub(*m.LastCheckedAt) <= deadline {
		return
	}
	if m.Status == model.StatusDown {
		return
	}

	res := model.CheckResult{
		MonitorID:    m.ID,
		Status:       model.StatusDown,
		ErrorMessage: fmt.Sprintf("missed heartbeat (no signal for %ds; limit %ds)", int(now.Sub(*m.LastCheckedAt).Seconds()), int(deadline.Seconds())),
		CheckedAt:    now,
	}
	if err := s.store.RecordCheck(ctx, &res); err != nil {
		log.Printf("record missed heartbeat error (%s): %v", m.Name, err)
		return
	}

	s.HandleTransition(ctx, m, m.Status, res)
	s.hub.Broadcast("check", map[string]any{
		"monitor_id": m.ID,
		"result":     res,
	})
}

func (s *Scheduler) EvaluateThresholds(m model.Monitor, res *model.CheckResult) []string {
	var breaches []string
	if m.CPUThreshold > 0 && res.CPUPct != nil && *res.CPUPct >= m.CPUThreshold {
		breaches = append(breaches, fmt.Sprintf("CPU %.1f%% >= %.0f%%", *res.CPUPct, m.CPUThreshold))
	}
	if m.RAMThreshold > 0 && res.RAMPct != nil && *res.RAMPct >= m.RAMThreshold {
		breaches = append(breaches, fmt.Sprintf("RAM %.1f%% >= %.0f%%", *res.RAMPct, m.RAMThreshold))
	}
	if m.DiskThreshold > 0 && res.DiskPct != nil && *res.DiskPct >= m.DiskThreshold {
		breaches = append(breaches, fmt.Sprintf("Disk %.1f%% >= %.0f%%", *res.DiskPct, m.DiskThreshold))
	}
	return breaches
}

func (s *Scheduler) HandleTransition(ctx context.Context, m model.Monitor, prevStatus string, res model.CheckResult) {
	if res.Status == model.StatusDown && prevStatus != model.StatusDown {
		msg := res.ErrorMessage
		if msg == "" {
			msg = "Service is unreachable"
		}
		s.alerts.Notify(ctx, m, model.EventDown, msg)
		return
	}

	if res.Status == model.StatusUp && prevStatus == model.StatusDown {
		msg := fmt.Sprintf("Service is back UP (latency %dms)", res.ResponseTimeMs)
		s.alerts.Notify(ctx, m, model.EventUp, msg)
		return
	}

	if res.Status == model.StatusDegraded {
		breaches := s.EvaluateThresholds(m, &res)
		if len(breaches) > 0 {
			s.alerts.Notify(ctx, m, model.EventThreshold, "Resource threshold exceeded: "+strings.Join(breaches, ", "))
		}
	}
}
