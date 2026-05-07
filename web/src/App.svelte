<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { cancelRefresh, fetchMetrics, triggerRefresh } from './lib/api';
  import {
    emptyMetrics,
    liveAdded,
    liveCompletedChecks,
    liveFailedChecks,
    liveFetched,
    liveScheduledChecks,
    metricDelta,
    type Metrics
  } from './lib/metrics';
  import type { PulseInput } from './lib/pulse';
  import MetricNumber from './components/MetricNumber.svelte';
  import PipelineCanvas from './components/PipelineCanvas.svelte';

  let metrics: Metrics = emptyMetrics;
  let previous: Metrics | null = null;
  let pulses: PulseInput[] = [];
  let error = '';
  let busy = false;
  let timer: number | undefined;

  $: healthyRatio = metrics.pool.total > 0 ? (metrics.pool.healthy / metrics.pool.total) * 100 : 0;
  $: activeChecks = liveCompletedChecks(metrics);
  $: fetchedLive = liveFetched(metrics);
  $: addedLive = liveAdded(metrics);
  $: scheduledLive = liveScheduledChecks(metrics);
  $: failedLive = liveFailedChecks(metrics);
  $: protocolRows = Object.entries(metrics.pool.protocols ?? {}).sort((a, b) => b[1] - a[1]);
  $: errorRows = Object.entries(metrics.check.error_types ?? {}).sort((a, b) => b[1] - a[1]);
  $: sourceRows = [...(metrics.fetch.sources ?? [])].sort((a, b) => b.duration_ms - a.duration_ms).slice(0, 10);

  onMount(() => {
    void poll();
    timer = window.setInterval(poll, 1000);
  });

  onDestroy(() => {
    if (timer) window.clearInterval(timer);
  });

  async function poll() {
    try {
      const next = await fetchMetrics();
      const delta = metricDelta(next, previous);
      pulses = [
        { kind: 'success', count: delta.fetched },
        { kind: 'check', count: delta.checked },
        { kind: 'error', count: delta.failed },
        { kind: 'skip', count: delta.skipped }
      ];
      previous = metrics;
      metrics = next;
      error = '';
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    }
  }

  async function runRefresh(action: 'start' | 'cancel') {
    busy = true;
    try {
      if (action === 'start') await triggerRefresh();
      else await cancelRefresh();
      await poll();
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    } finally {
      busy = false;
    }
  }

  function formatDuration(ms: number) {
    const seconds = Math.floor(ms / 1000);
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = seconds % 60;
    return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  }

  function formatBytes(value: number) {
    if (value < 1024) return `${value} B`;
    if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`;
    return `${(value / 1024 / 1024).toFixed(1)} MiB`;
  }
</script>

<main class="shell">
  <aside class="rail">
    <div class="mark">PP</div>
    <a class="active" href="#overview">Overview</a>
    <a href="#pipeline">Pipeline</a>
    <a href="#sources">Sources</a>
    <a href="#checks">Checks</a>
    <a href="#pool">Pool</a>
    <a href="#events">Events</a>
  </aside>

  <section class="workspace">
    <header class="topbar">
      <div>
        <strong>plugproxy console</strong>
        <span>{formatDuration(metrics.uptime_ms)}</span>
      </div>
      <div class="status-strip">
        <span class={`phase ${metrics.refresh.running ? 'running' : ''}`}>{metrics.refresh.phase ?? metrics.refresh.status}</span>
        <span>healthy {healthyRatio.toFixed(1)}%</span>
        <span>checks {activeChecks}/{scheduledLive}</span>
        <span>{metrics.refresh.last_reason ?? 'idle'}</span>
      </div>
      <div class="actions">
        <button disabled={busy} on:click={() => runRefresh('start')}>Refresh</button>
        <button class="danger" disabled={busy} on:click={() => runRefresh('cancel')}>Cancel</button>
      </div>
    </header>

    {#if error}
      <div class="notice">{error}</div>
    {/if}

    <section id="overview" class="metric-grid">
      <MetricNumber label="pool total" value={metrics.pool.total} tone="info" />
      <MetricNumber label="healthy" value={metrics.pool.healthy} tone="good" />
      <MetricNumber label="degraded" value={metrics.pool.degraded} tone="warn" />
      <MetricNumber label="dead" value={metrics.pool.dead} tone="bad" />
      <MetricNumber label="fetched" value={fetchedLive} tone="info" />
      <MetricNumber label="added" value={addedLive} tone="good" />
      <MetricNumber label="checked" value={activeChecks} tone="info" />
      <MetricNumber label="timeouts/errors" value={failedLive} tone="bad" />
      <MetricNumber label="goroutines" value={metrics.runtime.goroutines} tone="neutral" />
    </section>

    <section id="pipeline" class="panel pipeline-panel">
      <div class="panel-head">
        <h2>Pipeline</h2>
        <span>source result level flow, sampled by metrics delta</span>
      </div>
      <PipelineCanvas
        {pulses}
        fetchWorkers={metrics.config.source_workers}
        checkWorkers={metrics.config.check_workers}
        phase={metrics.refresh.phase ?? metrics.refresh.status}
      />
    </section>

    <section class="split">
      <div id="sources" class="panel">
        <div class="panel-head">
          <h2>Sources</h2>
          <span>{metrics.fetch.successful_sources}/{metrics.fetch.total_sources} ok</span>
        </div>
        <table>
          <thead><tr><th>source</th><th>status</th><th>count</th><th>ms</th><th>error</th></tr></thead>
          <tbody>
            {#each sourceRows as source}
              <tr>
                <td>{source.name}</td>
                <td><span class={`badge ${source.status}`}>{source.status}</span></td>
                <td>{source.count}</td>
                <td>{source.duration_ms}</td>
                <td>{source.error_type ?? ''}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      <div id="checks" class="panel">
        <div class="panel-head">
          <h2>Checks</h2>
          <span>{metrics.config.check_profile || 'full'} profile</span>
        </div>
        <div class="bars">
          {#each protocolRows as [protocol, count]}
            <div class="bar-row">
              <span>{protocol}</span>
              <div><i style={`width:${metrics.pool.total ? (count / metrics.pool.total) * 100 : 0}%`}></i></div>
              <b>{count}</b>
            </div>
          {/each}
        </div>
        <div class="error-list">
          {#each errorRows as [kind, count]}
            <div><span>{kind}</span><b>{count}</b></div>
          {/each}
        </div>
      </div>
    </section>

    <section id="pool" class="panel footer-panel">
      <div>
        <h2>Runtime</h2>
        <p>alloc {formatBytes(metrics.runtime.alloc)} · heap {formatBytes(metrics.runtime.heap_alloc)} · sys {formatBytes(metrics.runtime.sys)} · gc {metrics.runtime.num_gc}</p>
      </div>
      <div>
        <h2>Config</h2>
        <p>source workers {metrics.config.source_workers} · per-host {metrics.config.per_host_workers} · check workers {metrics.config.check_workers} · max checks {metrics.config.max_checks}</p>
      </div>
    </section>
  </section>
</main>
