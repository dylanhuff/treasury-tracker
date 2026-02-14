import { describe, it, expect } from 'vitest';
import { resampleTimeSeries } from '../chartUtils';
import type { HistoricalDataPoint } from '../../services/api';

function makePoint(date: string, value: number): HistoricalDataPoint {
  return { date, '10Y': value, '5Y': value - 0.5, '2Y': value - 1.0 };
}

describe('resampleTimeSeries', () => {
  it('returns empty array for null/undefined input', () => {
    expect(resampleTimeSeries(null as unknown as HistoricalDataPoint[])).toEqual([]);
    expect(resampleTimeSeries(undefined as unknown as HistoricalDataPoint[])).toEqual([]);
  });

  it('returns original data when fewer than 2 points', () => {
    const single = [makePoint('2025-01-01', 4.0)];
    expect(resampleTimeSeries(single)).toEqual(single);
  });

  it('returns sorted original when data length <= targetPoints', () => {
    const data = [
      makePoint('2025-01-03', 4.2),
      makePoint('2025-01-01', 4.0),
      makePoint('2025-01-02', 4.1),
    ];
    const result = resampleTimeSeries(data, 5);
    expect(result).toHaveLength(3);
    expect(result[0].date).toBe('2025-01-01');
    expect(result[2].date).toBe('2025-01-03');
  });

  it('downsamples to target number of points', () => {
    // Create 100 daily points
    const data: HistoricalDataPoint[] = [];
    for (let i = 0; i < 100; i++) {
      const d = new Date(2025, 0, 1 + i);
      data.push(makePoint(d.toISOString().split('T')[0], 4.0 + i * 0.01));
    }
    const result = resampleTimeSeries(data, 20);
    expect(result).toHaveLength(20);
  });

  it('produces linearly spaced dates', () => {
    // Create data with non-uniform density: 10 points in Jan, 1 in Feb
    const data: HistoricalDataPoint[] = [];
    for (let i = 1; i <= 10; i++) {
      data.push(makePoint(`2025-01-${String(i).padStart(2, '0')}`, 4.0));
    }
    data.push(makePoint('2025-02-10', 5.0));

    const result = resampleTimeSeries(data, 5);
    expect(result).toHaveLength(5);

    // Check dates are evenly spaced
    const times = result.map((p) => new Date(p.date).getTime());
    const gaps = [];
    for (let i = 1; i < times.length; i++) {
      gaps.push(times[i] - times[i - 1]);
    }
    // All gaps should be approximately equal
    const avgGap = gaps.reduce((a, b) => a + b, 0) / gaps.length;
    for (const gap of gaps) {
      expect(Math.abs(gap - avgGap) / avgGap).toBeLessThan(0.01);
    }
  });

  it('interpolates yield values linearly', () => {
    // Need more source points than target to trigger downsampling
    const data = [
      makePoint('2025-01-01', 4.0),
      makePoint('2025-01-04', 4.3),
      makePoint('2025-01-07', 4.6),
      makePoint('2025-01-10', 4.9),
      makePoint('2025-01-11', 5.0),
    ];
    // Downsample 5 points to 3
    const result = resampleTimeSeries(data, 3);
    expect(result).toHaveLength(3);

    // First point = start (4.0), last = end (5.0)
    expect(result[0]['10Y']).toBeCloseTo(4.0, 1);
    expect(result[2]['10Y']).toBeCloseTo(5.0, 1);
    // Midpoint should be interpolated to ~4.5
    expect(result[1]['10Y']).toBeCloseTo(4.5, 1);
  });

  it('first and last points match original date range', () => {
    const data = [
      makePoint('2025-01-01', 4.0),
      makePoint('2025-06-30', 4.5),
      makePoint('2025-12-31', 5.0),
    ];
    const result = resampleTimeSeries(data, 3);
    expect(result[0].date).toBe('2025-01-01');
    expect(result[result.length - 1].date).toBe('2025-12-31');
  });

  it('handles data with identical dates gracefully', () => {
    const data = [
      makePoint('2025-01-01', 4.0),
      makePoint('2025-01-01', 4.5),
      makePoint('2025-01-02', 5.0),
    ];
    // Should not throw or produce NaN
    const result = resampleTimeSeries(data, 3);
    expect(result).toHaveLength(3);
    for (const p of result) {
      expect(Number.isNaN(p['10Y'])).toBe(false);
    }
  });
});
