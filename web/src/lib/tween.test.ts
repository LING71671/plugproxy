import { describe, expect, it } from 'vitest';
import { retargetTween, tweenValue } from './tween';

describe('counter tween', () => {
  it('moves upward without jumping', () => {
    const tween = retargetTween(undefined, 1000, 0, true);
    const mid = tweenValue(tween, tween.duration / 2);
    expect(mid).toBeGreaterThan(0);
    expect(mid).toBeLessThan(1000);
    expect(tweenValue(tween, tween.duration + 1)).toBe(1000);
  });

  it('moves downward smoothly', () => {
    const first = retargetTween(undefined, 1000, 0, true);
    const second = retargetTween(first, 250, first.duration / 2);
    expect(second.from).toBeGreaterThan(250);
    expect(tweenValue(second, second.startedAt + second.duration + 1)).toBe(250);
  });

  it('retargets from the displayed value', () => {
    const first = retargetTween(undefined, 1000, 0, true);
    const current = tweenValue(first, 300);
    const second = retargetTween(first, 1200, 300);
    expect(second.from).toBeCloseTo(current);
    expect(second.target).toBe(1200);
  });
});
