<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { PulseInput } from '../lib/pulse';

  export let pulses: PulseInput[] = [];
  export let fetchWorkers = 0;
  export let checkWorkers = 0;
  export let phase = 'idle';

  let canvas: HTMLCanvasElement;
  let ctx: CanvasRenderingContext2D;
  let frame = 0;
  let lastPulseRef = pulses;
  let particles: Array<{ lane: number; x: number; y: number; speed: number; color: string; born: number }> = [];

  const labels = ['sources', 'fetch', 'scheduler', 'check', 'pool', 'cache'];
  const colors = {
    success: '#4ade80',
    check: '#38bdf8',
    error: '#ef4444',
    skip: '#94a3b8'
  };

  $: if (pulses !== lastPulseRef) {
    lastPulseRef = pulses;
    spawnPulses(pulses);
  }

  onMount(() => {
    ctx = canvas.getContext('2d') as CanvasRenderingContext2D;
    resize();
    window.addEventListener('resize', resize);
    frame = requestAnimationFrame(draw);
  });

  onDestroy(() => {
    window.removeEventListener('resize', resize);
    if (frame) cancelAnimationFrame(frame);
  });

  function resize() {
    const rect = canvas.getBoundingClientRect();
    const ratio = window.devicePixelRatio || 1;
    canvas.width = Math.max(1, Math.floor(rect.width * ratio));
    canvas.height = Math.max(1, Math.floor(rect.height * ratio));
    ctx?.setTransform(ratio, 0, 0, ratio, 0, 0);
  }

  function spawnPulses(input: PulseInput[]) {
    for (const item of input) {
      const total = Math.min(20, Math.max(0, item.count));
      for (let i = 0; i < total; i++) {
        const lane = item.kind === 'error' ? 2 : item.kind === 'skip' ? 1 : i % 3;
        particles.push({
          lane,
          x: 34,
          y: 76 + lane * 38,
          speed: 1.4 + Math.random() * 0.8,
          color: colors[item.kind],
          born: performance.now() + i * 25
        });
      }
    }
  }

  function draw(now: number) {
    const width = canvas.clientWidth;
    const height = canvas.clientHeight;
    ctx.clearRect(0, 0, width, height);
    drawGrid(width, height);
    drawNodes(width);
    drawWorkers(width);
    drawParticles(now, width);
    frame = requestAnimationFrame(draw);
  }

  function drawGrid(width: number, height: number) {
    ctx.strokeStyle = '#1c2833';
    ctx.lineWidth = 1;
    for (let y = 28; y < height; y += 24) {
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(width, y);
      ctx.stroke();
    }
  }

  function drawNodes(width: number) {
    const step = (width - 80) / (labels.length - 1);
    labels.forEach((label, index) => {
      const x = 40 + index * step;
      ctx.fillStyle = phase === label ? '#38bdf8' : '#18232d';
      ctx.strokeStyle = '#2b3a46';
      ctx.lineWidth = 1;
      roundRect(x - 36, 24, 72, 30, 5);
      ctx.fill();
      ctx.stroke();
      ctx.fillStyle = '#d7e0ea';
      ctx.font = '12px ui-monospace, Consolas, monospace';
      ctx.textAlign = 'center';
      ctx.fillText(label, x, 44);
      if (index < labels.length - 1) {
        ctx.strokeStyle = '#2f4352';
        ctx.beginPath();
        ctx.moveTo(x + 40, 39);
        ctx.lineTo(x + step - 40, 39);
        ctx.stroke();
      }
    });
  }

  function drawWorkers(width: number) {
    drawLane('fetch workers', fetchWorkers, 76, width);
    drawLane('check workers', checkWorkers, 114, width);
  }

  function drawLane(label: string, count: number, y: number, width: number) {
    ctx.fillStyle = '#7f8d9b';
    ctx.font = '11px ui-monospace, Consolas, monospace';
    ctx.textAlign = 'left';
    ctx.fillText(label, 14, y + 11);
    const visible = Math.min(48, Math.max(0, count));
    const startX = 120;
    const gap = Math.min(12, (width - startX - 20) / Math.max(visible, 1));
    for (let i = 0; i < visible; i++) {
      const active = phase === 'checking' ? i % 3 !== 0 : phase === 'fetching' ? i % 4 === 0 : false;
      ctx.fillStyle = active ? '#38bdf8' : '#1a2630';
      ctx.fillRect(startX + i * gap, y, Math.max(5, gap - 3), 14);
    }
  }

  function drawParticles(now: number, width: number) {
    particles = particles.filter((p) => p.x < width - 40 && now - p.born < 10000);
    for (const p of particles) {
      if (now < p.born) continue;
      p.x += p.speed;
      ctx.fillStyle = p.color;
      ctx.globalAlpha = 0.88;
      ctx.beginPath();
      ctx.arc(p.x, p.y, 3, 0, Math.PI * 2);
      ctx.fill();
      ctx.globalAlpha = 1;
    }
  }

  function roundRect(x: number, y: number, w: number, h: number, r: number) {
    ctx.beginPath();
    ctx.moveTo(x + r, y);
    ctx.arcTo(x + w, y, x + w, y + h, r);
    ctx.arcTo(x + w, y + h, x, y + h, r);
    ctx.arcTo(x, y + h, x, y, r);
    ctx.arcTo(x, y, x + w, y, r);
    ctx.closePath();
  }
</script>

<canvas bind:this={canvas} aria-label="plugproxy pipeline"></canvas>
