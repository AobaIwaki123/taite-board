/**
 * Application Data Fetching and Local Cache Management
 */

import { ScheduleData } from './types/schedule';
import { resolveCurrentGroup, GroupConfig } from './config/group';

export interface LoadScheduleResult {
  data: ScheduleData;
  isFallback: boolean;
  group: GroupConfig;
}

export async function loadScheduleData(customGroup?: GroupConfig): Promise<LoadScheduleResult> {
  const group = customGroup || resolveCurrentGroup();
  const cacheKey = `${group.storagePrefix}cached_schedule`;

  // Purge any residual telemetry keys for privacy governance (Zero Telemetry)
  try {
    localStorage.removeItem('taite_visitor_id');
    localStorage.removeItem('taite_visit_count');
    localStorage.removeItem('taite_telemetry_logs');
    localStorage.removeItem('taite_telemetry_endpoint');
  } catch {
    // ignore
  }

  let data: ScheduleData | null = null;
  let isFallback = false;

  try {
    // group.dataUrl を優先取得 (フォールバックとして group.fallbackDataUrl も試行)
    let res = await fetch(`${group.dataUrl}?t=${Date.now()}`).catch(() => null);
    if (!res || !res.ok) {
      res = await fetch(`${group.fallbackDataUrl}?t=${Date.now()}`).catch(() => null);
    }
    if (res && res.ok) {
      data = (await res.json()) as ScheduleData;
      try {
        localStorage.setItem(cacheKey, JSON.stringify(data));
      } catch {
        // ignore
      }
    } else {
      throw new Error('Network response not ok');
    }
  } catch (e) {
    console.warn('Network load failed, falling back to local cache', e);
    isFallback = true;
    try {
      const cached = localStorage.getItem(cacheKey);
      if (cached) {
        data = JSON.parse(cached) as ScheduleData;
      }
    } catch {
      // ignore
    }
  }

  if (!data) {
    throw new Error('Fatal: No schedule data available');
  }

  return { data, isFallback, group };
}
