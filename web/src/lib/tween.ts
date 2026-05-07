export interface TweenState {
  value: number;
  from: number;
  target: number;
  startedAt: number;
  duration: number;
}

export function easeOutCubic(t: number): number {
  const clamped = Math.min(1, Math.max(0, t));
  return 1 - Math.pow(1 - clamped, 3);
}

export function durationForDelta(delta: number, initial = false): number {
  if (initial) {
    return Math.min(1200, Math.max(900, Math.log10(Math.abs(delta) + 10) * 520));
  }
  const abs = Math.abs(delta);
  if (abs < 10) return 250;
  if (abs < 500) return 500;
  return 900;
}

export function retargetTween(state: TweenState | undefined, target: number, now: number, initial = false): TweenState {
  const current = state ? tweenValue(state, now) : 0;
  return {
    value: current,
    from: current,
    target,
    startedAt: now,
    duration: durationForDelta(target - current, initial)
  };
}

export function tweenValue(state: TweenState, now: number): number {
  if (state.duration <= 0) return state.target;
  const progress = (now - state.startedAt) / state.duration;
  const eased = easeOutCubic(progress);
  return state.from + (state.target - state.from) * eased;
}
