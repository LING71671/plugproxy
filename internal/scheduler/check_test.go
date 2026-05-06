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

func TestScheduleChecksSmartSkipsByStatusTTL(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	proxies := []model.Proxy{
		checkedProxy("http://healthy:1", model.HealthHealthy, now.Add(-time.Hour), 90),
		checkedProxy("http://degraded:1", model.HealthDegraded, now.Add(-time.Hour), 50),
	}

	schedule := ScheduleChecks(proxies, CheckOptions{
		Now:              now,
		Profile:          ProfileSmart,
		HealthyCheckTTL:  6 * time.Hour,
		DegradedCheckTTL: 30 * time.Minute,
	})
	if schedule.Stats.Selected != 1 || schedule.Stats.SkippedRecent != 1 {
		t.Fatalf("unexpected smart ttl stats %#v", schedule.Stats)
	}
	if schedule.Selected[0].ID != "http://degraded:1" {
		t.Fatalf("expected degraded proxy selected, got %#v", schedule.Selected)
	}
}

func TestScheduleChecksSmartDeadBackoff(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	dead := checkedProxy("http://dead:1", model.HealthDead, now.Add(-20*time.Hour), 10)
	dead.ConsecutiveFailures = 5

	schedule := ScheduleChecks([]model.Proxy{dead}, CheckOptions{
		Now:            now,
		Profile:        ProfileSmart,
		DeadCheckTTL:   12 * time.Hour,
		DeadBackoffMax: 72 * time.Hour,
	})
	if schedule.Stats.Selected != 0 || schedule.Stats.SkippedBackoff != 1 {
		t.Fatalf("expected dead proxy backed off, got %#v", schedule.Stats)
	}
}

func TestScheduleChecksSmartDeadBackoffMax(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	dead := checkedProxy("http://dead:1", model.HealthDead, now.Add(-73*time.Hour), 10)
	dead.ConsecutiveFailures = 10

	schedule := ScheduleChecks([]model.Proxy{dead}, CheckOptions{
		Now:            now,
		Profile:        ProfileSmart,
		DeadCheckTTL:   12 * time.Hour,
		DeadBackoffMax: 72 * time.Hour,
	})
	if schedule.Stats.Selected != 1 || schedule.Stats.SkippedBackoff != 0 {
		t.Fatalf("expected dead proxy selected after capped backoff, got %#v", schedule.Stats)
	}
}

func TestScheduleChecksSmartSkipsUnsupported(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	proxy := model.Proxy{ID: "socks4://one:1", Address: "one:1", Protocol: model.ProtocolSOCKS4}

	schedule := ScheduleChecks([]model.Proxy{proxy}, CheckOptions{Now: now, Profile: ProfileSmart, SkipUnsupported: true})
	if schedule.Stats.Selected != 0 || schedule.Stats.SkippedUnsupported != 1 {
		t.Fatalf("expected unsupported skip, got %#v", schedule.Stats)
	}
	if schedule.Stats.ByProtocol[model.ProtocolSOCKS4].SkippedUnsupported != 1 {
		t.Fatalf("expected protocol unsupported stats, got %#v", schedule.Stats.ByProtocol)
	}
}

func TestScheduleChecksProtocolFairLimit(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	proxies := []model.Proxy{
		uncheckedProxy("http://one:1", model.ProtocolHTTP),
		uncheckedProxy("http://two:1", model.ProtocolHTTP),
		uncheckedProxy("https://one:1", model.ProtocolHTTPS),
		uncheckedProxy("socks5://one:1", model.ProtocolSOCKS5),
	}

	schedule := ScheduleChecks(proxies, CheckOptions{Now: now, MaxChecks: 3, ProtocolFair: true})
	got := ids(schedule.Selected)
	want := []string{"http://one:1", "https://one:1", "socks5://one:1"}
	assertIDs(t, got, want)
	if schedule.Stats.SkippedLimit != 1 || schedule.Stats.ByProtocol[model.ProtocolHTTP].SkippedLimit != 1 {
		t.Fatalf("unexpected limit stats %#v", schedule.Stats)
	}
}

func TestScheduleChecksProtocolFairReusesUnusedBudget(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	proxies := []model.Proxy{
		uncheckedProxy("http://one:1", model.ProtocolHTTP),
		uncheckedProxy("http://two:1", model.ProtocolHTTP),
		uncheckedProxy("https://one:1", model.ProtocolHTTPS),
	}

	schedule := ScheduleChecks(proxies, CheckOptions{Now: now, MaxChecks: 3, ProtocolFair: true})
	if schedule.Stats.Selected != 3 || schedule.Stats.SkippedLimit != 0 {
		t.Fatalf("expected unused protocol budget to flow, got %#v", schedule.Stats)
	}
}

func TestScheduleChecksSortsSameStatusBySeenCount(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	low := checkedProxy("http://low:1", model.HealthHealthy, now.Add(-time.Hour), 90)
	high := checkedProxy("http://high:1", model.HealthHealthy, now.Add(-time.Hour), 90)
	low.SeenCount = 1
	high.SeenCount = 3

	schedule := ScheduleChecks([]model.Proxy{low, high}, CheckOptions{Now: now})
	assertIDs(t, ids(schedule.Selected), []string{"http://high:1", "http://low:1"})
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

func uncheckedProxy(id string, protocol model.Protocol) model.Proxy {
	return model.Proxy{ID: id, Address: id, Protocol: protocol}
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
