import type { HistoricalDataPoint } from '../services/api';

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

  const yieldKeys = Object.keys(sorted[0]).filter(
    (k) => k !== 'date' && typeof (sorted[0] as Record<string, unknown>)[k] === 'number'
  );

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

    const point: Record<string, unknown> = { date: toDateStr(targetTime) };
    for (const key of yieldKeys) {
      const v1 = (p1 as Record<string, unknown>)[key] as number;
      const v2 = (p2 as Record<string, unknown>)[key] as number;
      if (typeof v1 === 'number' && typeof v2 === 'number') {
        point[key] = v1 + (v2 - v1) * factor;
      } else {
        point[key] = typeof v1 === 'number' ? v1 : 0;
      }
    }

    resampled.push(point as HistoricalDataPoint);
  }

  return resampled;
}

function toDateStr(timestamp: number): string {
  return new Date(timestamp).toISOString().split('T')[0];
}
