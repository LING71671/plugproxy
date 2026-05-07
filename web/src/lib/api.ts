import type { Metrics } from './metrics';

export async function fetchMetrics(): Promise<Metrics> {
  const response = await fetch('/metrics.json', { headers: { Accept: 'application/json' } });
  if (!response.ok) {
    throw new Error(`metrics request failed: ${response.status}`);
  }
  return response.json() as Promise<Metrics>;
}

export async function triggerRefresh(): Promise<Record<string, unknown>> {
  const response = await fetch('/refresh', { method: 'POST', headers: { Accept: 'application/json' } });
  if (!response.ok) {
    throw new Error(`refresh request failed: ${response.status}`);
  }
  return response.json() as Promise<Record<string, unknown>>;
}

export async function cancelRefresh(): Promise<Record<string, unknown>> {
  const response = await fetch('/refresh/cancel', { method: 'POST', headers: { Accept: 'application/json' } });
  if (!response.ok) {
    throw new Error(`cancel request failed: ${response.status}`);
  }
  return response.json() as Promise<Record<string, unknown>>;
}
