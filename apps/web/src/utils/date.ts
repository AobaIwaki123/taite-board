/**
 * Date and Time utilities for Ribbon Clock
 */

export const TODAY_STR = "2026-10-30"; // 基準日

export const DAY_NAMES = ['日', '月', '火', '水', '木', '金', '土'] as const;

export function getDayOfWeekStr(dateStr: string): string {
  const [y, m, d] = dateStr.split('-').map(Number);
  const date = new Date(y, m - 1, d);
  return DAY_NAMES[date.getDay()];
}

export function parseMinutes(timeStr?: string): number | null {
  if (!timeStr) return null;
  const parts = timeStr.split(':').map(Number);
  if (parts.length < 2 || isNaN(parts[0]) || isNaN(parts[1])) return null;
  return parts[0] * 60 + parts[1];
}

export interface ProgressStatus {
  status: 'upcoming' | 'live' | 'benefit' | 'ended';
  label: string;
  badgeClass: string;
}

export function getSessionProgress(date: string, liveStart?: string, liveEnd?: string, benefitStart?: string, benefitEnd?: string): ProgressStatus {
  // 日付の比較
  if (date > TODAY_STR) {
    return { status: 'upcoming', label: '開催予定', badgeClass: 'bg-emerald-500/20 text-emerald-400 border-emerald-500/30' };
  }
  if (date < TODAY_STR) {
    return { status: 'ended', label: '終了', badgeClass: 'bg-zinc-800 text-zinc-400 border-zinc-700' };
  }

  // 当日の時間比較 (現状は時間進行の動的計算または開催中判定)
  const now = new Date();
  const currentMinutes = now.getHours() * 60 + now.getMinutes();

  const lStart = parseMinutes(liveStart);
  const lEnd = parseMinutes(liveEnd);
  const bStart = parseMinutes(benefitStart);
  const bEnd = parseMinutes(benefitEnd);

  if (lStart !== null && lEnd !== null && currentMinutes >= lStart && currentMinutes <= lEnd) {
    return { status: 'live', label: 'ライブ中', badgeClass: 'bg-rose-500 text-white animate-pulse font-black' };
  }

  if (bStart !== null && bEnd !== null && currentMinutes >= bStart && currentMinutes <= bEnd) {
    return { status: 'benefit', label: '特典会中', badgeClass: 'bg-purple-600 text-white animate-pulse font-black' };
  }

  if (bEnd !== null && currentMinutes > bEnd) {
    return { status: 'ended', label: '本日終了', badgeClass: 'bg-zinc-800 text-zinc-400 border-zinc-700' };
  }

  return { status: 'upcoming', label: '本日開催', badgeClass: 'bg-rose-500/20 text-rose-300 border-rose-500/30' };
}
