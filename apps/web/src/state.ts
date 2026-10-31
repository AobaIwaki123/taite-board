/**
 * Application Data Fetching and Local Cache Management
 */

import { ScheduleData } from './types/schedule';

export interface LoadScheduleResult {
  data: ScheduleData;
  isFallback: boolean;
}

export async function loadScheduleData(): Promise<LoadScheduleResult> {
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
    // schedule.json を優先取得 (フォールバックとして mock_schedule.json も試行)
    let res = await fetch('schedule.json?t=' + Date.now()).catch(() => null);
    if (!res || !res.ok) {
      res = await fetch('mock_schedule.json?t=' + Date.now()).catch(() => null);
    }
    if (res && res.ok) {
      data = (await res.json()) as ScheduleData;
      try {
        localStorage.setItem('taite_cached_schedule', JSON.stringify(data));
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
      const cached = localStorage.getItem('taite_cached_schedule');
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

  return { data, isFallback };
}
