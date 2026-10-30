/**
 * Application State and Cache Management
 */

import { ScheduleData, Session } from './types/schedule.ts';
import { TODAY_STR } from './utils/date.ts';

export interface AppState {
  appData: ScheduleData | null;
  currentId: string | null;
  previousTab: 'week' | 'all' | 'cal';
  currentCalYear: number;
  currentCalMonth: number; // 1-12
  isFallback: boolean;
  selectedFlyerIdx: number;
}

export const state: AppState = {
  appData: null,
  currentId: null,
  previousTab: 'week',
  currentCalYear: 2026,
  currentCalMonth: 11,
  isFallback: false,
  selectedFlyerIdx: 0,
};

export async function loadScheduleData(): Promise<ScheduleData> {
  // Purge any residual telemetry keys for privacy governance (Zero Telemetry)
  try {
    localStorage.removeItem('taite_visitor_id');
    localStorage.removeItem('taite_visit_count');
    localStorage.removeItem('taite_telemetry_logs');
    localStorage.removeItem('taite_telemetry_endpoint');
  } catch {
    // ignore
  }

  let isFallback = false;
  let data: ScheduleData | null = null;

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
    try {
      const cached = localStorage.getItem('taite_cached_schedule');
      if (cached) {
        data = JSON.parse(cached) as ScheduleData;
        isFallback = true;
      }
    } catch {
      // ignore
    }
  }

  if (!data) {
    throw new Error('Fatal: No schedule data available');
  }

  state.appData = data;
  state.isFallback = isFallback;

  // 初期対象セッションIDの決定
  if (data.current_next_id) {
    state.currentId = data.current_next_id;
  } else if (data.sessions && data.sessions.length > 0) {
    // 本日以降の最初のセッション
    const upcoming = data.sessions.find(s => s.date >= TODAY_STR);
    state.currentId = upcoming ? upcoming.id : data.sessions[0].id;
  }

  return data;
}

export function getCurrentSession(): Session | null {
  if (!state.appData || !state.appData.sessions) return null;
  return state.appData.sessions.find(s => s.id === state.currentId) || state.appData.sessions[0] || null;
}

export function updateFreshnessIndicator(): void {
  const indicator = document.getElementById('freshness-indicator');
  const freshnessText = document.getElementById('freshness-text');
  if (!indicator || !freshnessText) return;

  if (state.isFallback) {
    indicator.classList.remove('hidden');
    indicator.classList.add('flex');
    freshnessText.textContent = 'オフライン保存版を表示中';
    return;
  }

  if (state.appData && state.appData.generated_at) {
    try {
      const genTime = new Date(state.appData.generated_at).getTime();
      const now = Date.now();
      const diffHours = (now - genTime) / (1000 * 60 * 60);

      // 24時間以上更新されていない場合
      if (diffHours >= 24) {
        const days = Math.floor(diffHours / 24);
        indicator.classList.remove('hidden');
        indicator.classList.add('flex');
        freshnessText.textContent = `${days}日前の情報です`;
      } else {
        indicator.classList.add('hidden');
        indicator.classList.remove('flex');
      }
    } catch {
      indicator.classList.add('hidden');
      indicator.classList.remove('flex');
    }
  }
}
