export type ProtocolStats = Record<string, number>;
export type StatusStats = Record<string, number>;

export interface ProxyStats {
  total: number;
  healthy: number;
  degraded: number;
  dead: number;
  unchecked: number;
  protocols: ProtocolStats;
  statuses: StatusStats;
  sources: Record<string, number>;
}

export interface SourceReport {
  name: string;
  status: string;
  count: number;
  error?: string;
  error_type?: string;
  duration_ms: number;
  consecutive_failures?: number;
  cooldown_until?: string;
}

export interface FetchReport {
  total_sources: number;
  successful_sources: number;
  failed_sources: number;
  skipped_sources: number;
  fetched: number;
  added: number;
  duplicates: number;
  cache_count?: number;
  cache_error?: string;
  sources: SourceReport[];
}

export interface CheckStats {
  total: number;
  scheduled: number;
  skipped_recent: number;
  skipped_limit: number;
  skipped_unsupported: number;
  skipped_backoff: number;
  healthy: number;
  degraded: number;
  dead: number;
  unsupported: number;
  failed: number;
  error_types?: Record<string, number>;
  by_protocol?: Record<string, {
    total: number;
    selected: number;
    skipped_recent: number;
    skipped_limit: number;
    skipped_unsupported: number;
    skipped_backoff: number;
  }>;
}

export interface RefreshStatus {
  status: string;
  running: boolean;
  phase?: string;
  progress?: {
    total_sources: number;
    completed_sources: number;
    successful_sources: number;
    failed_sources: number;
    skipped_sources: number;
    fetched: number;
    added: number;
    duplicates: number;
    scheduled_checks: number;
    completed_checks: number;
    failed_checks: number;
    unsupported_checks: number;
  };
  next_at?: string;
  last_reason?: string;
  skipped_reason?: string;
  cancelled?: boolean;
}

export interface RuntimeMetrics {
  goroutines: number;
  alloc: number;
  heap_alloc: number;
  sys: number;
  num_gc: number;
}

export interface MetricsConfig {
  source_workers: number;
  per_host_workers: number;
  check_workers: number;
  max_checks: number;
  check_profile: string;
  skip_unsupported: boolean;
  protocol_fair: boolean;
  cache_path: string;
  refresh_policy?: Record<string, unknown>;
}

export interface Metrics {
  generated_at: string;
  uptime_ms: number;
  pool: ProxyStats;
  fetch: FetchReport;
  check: CheckStats;
  refresh: RefreshStatus;
  runtime: RuntimeMetrics;
  config: MetricsConfig;
}

export const emptyMetrics: Metrics = {
  generated_at: new Date(0).toISOString(),
  uptime_ms: 0,
  pool: {
    total: 0,
    healthy: 0,
    degraded: 0,
    dead: 0,
    unchecked: 0,
    protocols: {},
    statuses: {},
    sources: {}
  },
  fetch: {
    total_sources: 0,
    successful_sources: 0,
    failed_sources: 0,
    skipped_sources: 0,
    fetched: 0,
    added: 0,
    duplicates: 0,
    sources: []
  },
  check: {
    total: 0,
    scheduled: 0,
    skipped_recent: 0,
    skipped_limit: 0,
    skipped_unsupported: 0,
    skipped_backoff: 0,
    healthy: 0,
    degraded: 0,
    dead: 0,
    unsupported: 0,
    failed: 0
  },
  refresh: { status: 'idle', running: false },
  runtime: { goroutines: 0, alloc: 0, heap_alloc: 0, sys: 0, num_gc: 0 },
  config: {
    source_workers: 0,
    per_host_workers: 0,
    check_workers: 0,
    max_checks: 0,
    check_profile: '',
    skip_unsupported: false,
    protocol_fair: false,
    cache_path: ''
  }
};

export function metricDelta(current: Metrics, previous: Metrics | null) {
  const currentFetched = liveFetched(current);
  const currentChecked = liveCompletedChecks(current);
  const currentFailed = liveFailedChecks(current);
  const currentSkipped = current.check.skipped_limit + current.check.skipped_recent + current.check.skipped_unsupported + current.check.skipped_backoff;
  if (!previous) {
    return { fetched: currentFetched, checked: currentChecked, failed: currentFailed, skipped: currentSkipped };
  }
  const previousSkipped = previous.check.skipped_limit + previous.check.skipped_recent + previous.check.skipped_unsupported + previous.check.skipped_backoff;
  return {
    fetched: Math.max(0, currentFetched - liveFetched(previous)),
    checked: Math.max(0, currentChecked - liveCompletedChecks(previous)),
    failed: Math.max(0, currentFailed - liveFailedChecks(previous)),
    skipped: Math.max(0, currentSkipped - previousSkipped)
  };
}

export function liveFetched(metrics: Metrics): number {
  return metrics.refresh.running ? metrics.refresh.progress?.fetched ?? metrics.fetch.fetched : metrics.fetch.fetched;
}

export function liveAdded(metrics: Metrics): number {
  return metrics.refresh.running ? metrics.refresh.progress?.added ?? metrics.fetch.added : metrics.fetch.added;
}

export function liveScheduledChecks(metrics: Metrics): number {
  return metrics.refresh.running ? metrics.refresh.progress?.scheduled_checks ?? metrics.check.scheduled : metrics.check.scheduled;
}

export function liveCompletedChecks(metrics: Metrics): number {
  return metrics.refresh.running ? metrics.refresh.progress?.completed_checks ?? 0 : metrics.check.healthy + metrics.check.degraded + metrics.check.dead;
}

export function liveFailedChecks(metrics: Metrics): number {
  return metrics.refresh.running ? metrics.refresh.progress?.failed_checks ?? metrics.check.failed : metrics.check.failed;
}
