<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { retargetTween, tweenValue, type TweenState } from '../lib/tween';

  export let label: string;
  export let value = 0;
  export let unit = '';
  export let precision = 0;
  export let tone: 'good' | 'info' | 'warn' | 'bad' | 'neutral' = 'neutral';

  let displayed = 0;
  let previousTarget = Number.NaN;
  let tween: TweenState | undefined;
  let delta = 0;
  let deltaVisible = false;
  let frame = 0;
  let mounted = false;
  let deltaTimer: number | undefined;

  $: if (mounted && value !== previousTarget) {
    const now = performance.now();
    const initial = Number.isNaN(previousTarget);
    delta = initial ? value : value - previousTarget;
    deltaVisible = !initial && delta !== 0;
    previousTarget = value;
    tween = retargetTween(tween, value, now, initial);
    scheduleFrame();
    if (deltaTimer) window.clearTimeout(deltaTimer);
    deltaTimer = window.setTimeout(() => (deltaVisible = false), 3000);
  }

  onMount(() => {
    mounted = true;
    previousTarget = Number.NaN;
    tween = retargetTween(undefined, value, performance.now(), true);
    scheduleFrame();
  });

  onDestroy(() => {
    if (frame) cancelAnimationFrame(frame);
    if (deltaTimer) window.clearTimeout(deltaTimer);
  });

  function scheduleFrame() {
    if (!frame) frame = requestAnimationFrame(tick);
  }

  function tick(now: number) {
    frame = 0;
    if (!tween) return;
    displayed = tweenValue(tween, now);
    if (now - tween.startedAt < tween.duration) scheduleFrame();
    else displayed = tween.target;
  }

  function formatNumber(input: number) {
    return input.toLocaleString(undefined, {
      minimumFractionDigits: precision,
      maximumFractionDigits: precision
    });
  }
</script>

<section class={`metric ${tone}`}>
  <div class="metric-label">{label}</div>
  <div class="metric-value">
    {formatNumber(displayed)}<span>{unit}</span>
  </div>
  <div class:visible={deltaVisible} class={`delta ${delta > 0 ? 'up' : 'down'}`}>
    {delta > 0 ? '+' : ''}{formatNumber(delta)}{unit}
  </div>
</section>
