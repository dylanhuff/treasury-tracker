import type { HistoricalDataPoint } from '../services/api';

type YieldKey = '10Y' | '5Y' | '2Y';
const YIELD_KEYS: YieldKey[] = ['10Y', '5Y', '2Y'];

/**
 * Resamples historical yield data to evenly-spaced time points using linear interpolation.
 * Fixes non-linear x-axis caused by variable data density (daily recent vs monthly old data).
 */
export function resampleTimeSeries(
  data: HistoricalDataPoint[],
  targetPoints: number = 300
): HistoricalDataPoint[] {
  if (!data || data.length < 2) return data || [];

  const sorted = [...data].sort(
    (a, b) => new Date(a.date).getTime() - new Date(b.date).getTime()
  );

  const startTime = new Date(sorted[0].date).getTime();
  const endTime = new Date(sorted[sorted.length - 1].date).getTime();
  if (endTime === startTime) return data;

  // Don't upsample - if we have fewer points than target, return sorted original
  if (sorted.length <= targetPoints) return sorted;

  const step = (endTime - startTime) / (targetPoints - 1);
  const resampled: HistoricalDataPoint[] = [];
  let srcIdx = 0;

  for (let i = 0; i < targetPoints; i++) {
    const targetTime = startTime + i * step;

    // Advance source index to bracket targetTime
    while (
      srcIdx < sorted.length - 2 &&
      new Date(sorted[srcIdx + 1].date).getTime() < targetTime
    ) {
      srcIdx++;
    }

    const p1 = sorted[srcIdx];
    const p2 = sorted[srcIdx + 1];

    if (!p2) {
      resampled.push({ ...p1, date: toDateStr(targetTime) });
      continue;
    }

    const t1 = new Date(p1.date).getTime();
    const t2 = new Date(p2.date).getTime();
    const factor = t2 === t1 ? 0 : (targetTime - t1) / (t2 - t1);

    const point: HistoricalDataPoint = {
      date: toDateStr(targetTime),
      '10Y': 0,
      '5Y': 0,
      '2Y': 0,
    };

    for (const key of YIELD_KEYS) {
      const v1 = p1[key];
      const v2 = p2[key];
      point[key] = v1 + (v2 - v1) * factor;
    }

    resampled.push(point);
  }

  return resampled;
}

function toDateStr(timestamp: number): string {
  return new Date(timestamp).toISOString().split('T')[0];
}
