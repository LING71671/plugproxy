package scheduler

import (
	"testing"
	"time"

	"github.com/LING71671/plugproxy/pkg/model"
)

func TestScheduleChecksSelectsUnchecked(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	proxies := []model.Proxy{{
		ID:            "http://fresh:1",
		Address:       "fresh:1",
		Protocol:      model.ProtocolHTTP,
		CheckCount:    0,
		LastCheckedAt: now.Add(-time.Minute),
	}}

	schedule := ScheduleChecks(proxies, CheckOptions{Now: now, CheckTTL: time.Hour})
	if schedule.Stats.Selected != 1 || schedule.Stats.SkippedRecent != 0 {
		t.Fatalf("expected unchecked proxy selected, got %#v", schedule.Stats)
	}
}

func TestScheduleChecksSkipsRecentWithinTTL(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	proxies := []model.Proxy{checkedProxy("http://recent:1", model.HealthHealthy, now.Add(-time.Minute), 90)}

	schedule := ScheduleChecks(proxies, CheckOptions{Now: now, CheckTTL: 30 * time.Minute})
	if schedule.Stats.Selected != 0 || schedule.Stats.SkippedRecent != 1 {
		t.Fatalf("expected recent proxy skipped, got %#v", schedule.Stats)
	}
}

func TestScheduleChecksSelectsExpiredTTL(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	proxies := []model.Proxy{checkedProxy("http://old:1", model.HealthHealthy, now.Add(-time.Hour), 90)}

	schedule := ScheduleChecks(proxies, CheckOptions{Now: now, CheckTTL: 30 * time.Minute})
	if schedule.Stats.Selected != 1 || schedule.Stats.SkippedRecent != 0 {
		t.Fatalf("expected expired proxy selected, got %#v", schedule.Stats)
	}
}

func TestScheduleChecksSortsByStatusPriority(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	proxies := []model.Proxy{
		checkedProxy("http://dead:1", model.HealthDead, now.Add(-time.Hour), 10),
		checkedProxy("http://degraded:1", model.HealthDegraded, now.Add(-time.Hour), 50),
		{ID: "http://unchecked:1", Address: "unchecked:1", Protocol: model.ProtocolHTTP},
		checkedProxy("http://healthy:1", model.HealthHealthy, now.Add(-time.Hour), 90),
	}

	schedule := ScheduleChecks(proxies, CheckOptions{Now: now})
	got := ids(schedule.Selected)
	want := []string{"http://unchecked:1", "http://healthy:1", "http://degraded:1", "http://dead:1"}
	assertIDs(t, got, want)
}

func TestScheduleChecksSortsSameStatusByOldestThenScore(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	proxies := []model.Proxy{
		checkedProxy("http://newer:1", model.HealthHealthy, now.Add(-time.Minute), 100),
		checkedProxy("http://older-low-score:1", model.HealthHealthy, now.Add(-time.Hour), 10),
		checkedProxy("http://same-time-high-score:1", model.HealthHealthy, now.Add(-time.Minute), 80),
	}

	schedule := ScheduleChecks(proxies, CheckOptions{Now: now})
	got := ids(schedule.Selected)
	want := []string{"http://older-low-score:1", "http://newer:1", "http://same-time-high-score:1"}
	assertIDs(t, got, want)
}

func TestScheduleChecksMaxChecksLimitsSelection(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	proxies := []model.Proxy{
		{ID: "http://one:1", Address: "one:1", Protocol: model.ProtocolHTTP},
		{ID: "http://two:1", Address: "two:1", Protocol: model.ProtocolHTTP},
		{ID: "http://three:1", Address: "three:1", Protocol: model.ProtocolHTTP},
	}

	schedule := ScheduleChecks(proxies, CheckOptions{Now: now, MaxChecks: 2})
	if schedule.Stats.Selected != 2 || schedule.Stats.SkippedLimit != 1 {
		t.Fatalf("expected max checks limit, got %#v", schedule.Stats)
	}
}

func TestScheduleChecksZeroMaxChecksDoesNotLimit(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	proxies := []model.Proxy{
		{ID: "http://one:1", Address: "one:1", Protocol: model.ProtocolHTTP},
		{ID: "http://two:1", Address: "two:1", Protocol: model.ProtocolHTTP},
	}

	schedule := ScheduleChecks(proxies, CheckOptions{Now: now, MaxChecks: 0})
	if schedule.Stats.Selected != 2 || schedule.Stats.SkippedLimit != 0 {
		t.Fatalf("expected no limit, got %#v", schedule.Stats)
	}
}

func TestScheduleChecksZeroTTLDoesNotSkipRecent(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	proxies := []model.Proxy{checkedProxy("http://recent:1", model.HealthHealthy, now.Add(-time.Second), 90)}

	schedule := ScheduleChecks(proxies, CheckOptions{Now: now, CheckTTL: 0})
	if schedule.Stats.Selected != 1 || schedule.Stats.SkippedRecent != 0 {
		t.Fatalf("expected recent proxy selected with zero TTL, got %#v", schedule.Stats)
	}
}

func checkedProxy(id string, status model.HealthStatus, checkedAt time.Time, score int) model.Proxy {
	return model.Proxy{
		ID:            id,
		Address:       id,
		Protocol:      model.ProtocolHTTP,
		HealthStatus:  status,
		HealthScore:   score,
		CheckCount:    1,
		LastCheckedAt: checkedAt,
	}
}

func ids(proxies []model.Proxy) []string {
	result := make([]string, 0, len(proxies))
	for _, proxy := range proxies {
		result = append(result, proxy.ID)
	}
	return result
}

func assertIDs(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}
