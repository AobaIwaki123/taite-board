/**
 * Date and Time utilities for Ribbon Clock
 */

export const DAY_NAMES = ['日', '月', '火', '水', '木', '金', '土'] as const;

/**
 * 実行環境の現在日付を YYYY-MM-DD 形式で取得する (JST)
 */
export function getTodayStr(nowDate = new Date()): string {
  const y = nowDate.getFullYear();
  const m = String(nowDate.getMonth() + 1).padStart(2, '0');
  const d = String(nowDate.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

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
  countdownMessage?: string;
  minutesRemaining?: number;
  phase: 'pre_live' | 'live' | 'intermission' | 'benefit' | 'ended';
}

function formatMinutesDuration(totalMinutes: number, targetAction: string): string {
  if (totalMinutes <= 0) {
    return `${targetAction}時刻`;
  }
  if (totalMinutes >= 60) {
    const hours = Math.floor(totalMinutes / 60);
    const mins = totalMinutes % 60;
    return mins > 0
      ? `${targetAction}まで あと${hours}時間${mins}分`
      : `${targetAction}まで あと${hours}時間`;
  }
  return `${targetAction}まで あと${totalMinutes}分`;
}

export function getSessionProgress(
  date: string,
  liveStart?: string,
  liveEnd?: string,
  benefitStart?: string,
  benefitEnd?: string,
  nowDate = new Date(),
  delayMinutes = 0
): ProgressStatus {
  const todayStr = getTodayStr(nowDate);

  if (date > todayStr) {
    return {
      status: 'upcoming',
      label: '開催予定',
      badgeClass: 'bg-emerald-500/20 text-emerald-400 border-emerald-500/30',
      phase: 'pre_live',
    };
  }
  if (date < todayStr) {
    return {
      status: 'ended',
      label: '終了',
      badgeClass: 'bg-zinc-800 text-zinc-400 border-zinc-700',
      countdownMessage: '公演終了',
      phase: 'ended',
    };
  }

  // 当日の時間比較（遅延オフセットを考慮：進行が押している場合は実質現在時刻から遅延分を引いて判定）
  const rawMinutes = nowDate.getHours() * 60 + nowDate.getMinutes();
  const currentMinutes = rawMinutes - delayMinutes;

  const lStart = parseMinutes(liveStart);
  const lEnd = parseMinutes(liveEnd);
  const bStart = parseMinutes(benefitStart);
  const bEnd = parseMinutes(benefitEnd);

  const delaySuffix = delayMinutes > 0 ? ` (+${delayMinutes}分押)` : '';

  // 1. 出番中
  if (lStart !== null && lEnd !== null && currentMinutes >= lStart && currentMinutes <= lEnd) {
    const remaining = lEnd - currentMinutes;
    return {
      status: 'live',
      label: `ライブ中${delaySuffix}`,
      badgeClass: 'bg-rose-500 text-white animate-pulse font-black',
      countdownMessage: formatMinutesDuration(remaining, '出番終了'),
      minutesRemaining: remaining,
      phase: 'live',
    };
  }

  // 2. 特典会中
  if (bStart !== null && bEnd !== null && currentMinutes >= bStart && currentMinutes <= bEnd) {
    const remaining = bEnd - currentMinutes;
    return {
      status: 'benefit',
      label: `特典会中${delaySuffix}`,
      badgeClass: 'bg-purple-600 text-white animate-pulse font-black',
      countdownMessage: formatMinutesDuration(remaining, '特典会終了'),
      minutesRemaining: remaining,
      phase: 'benefit',
    };
  }

  // 3. 特典会終了後（または出番終了後）
  if (bEnd !== null && currentMinutes > bEnd) {
    return {
      status: 'ended',
      label: '本日終了',
      badgeClass: 'bg-zinc-800 text-zinc-400 border-zinc-700',
      countdownMessage: '本日の全行程終了',
      phase: 'ended',
    };
  }
  if (bEnd === null && lEnd !== null && currentMinutes > lEnd) {
    return {
      status: 'ended',
      label: '本日終了',
      badgeClass: 'bg-zinc-800 text-zinc-400 border-zinc-700',
      countdownMessage: '出番終了',
      phase: 'ended',
    };
  }

  // 4. 出番終了〜特典会開始までの転換・待機時間
  if (lEnd !== null && bStart !== null && currentMinutes > lEnd && currentMinutes < bStart) {
    const remaining = bStart - currentMinutes;
    return {
      status: 'upcoming',
      label: `出番終了 / 特典会待ち${delaySuffix}`,
      badgeClass: 'bg-purple-900/60 text-purple-300 border-purple-500/40',
      countdownMessage: formatMinutesDuration(remaining, '特典会開始'),
      minutesRemaining: remaining,
      phase: 'intermission',
    };
  }

  // 5. 出番前（本日開催）
  if (lStart !== null && currentMinutes < lStart) {
    const remaining = lStart - currentMinutes;
    return {
      status: 'upcoming',
      label: delayMinutes > 0 ? `本日開催 (+${delayMinutes}分押)` : '本日開催',
      badgeClass: 'bg-rose-500/20 text-rose-300 border-rose-500/30',
      countdownMessage: formatMinutesDuration(remaining, '出番開始'),
      minutesRemaining: remaining,
      phase: 'pre_live',
    };
  }

  return {
    status: 'upcoming',
    label: delayMinutes > 0 ? `本日開催 (+${delayMinutes}分押)` : '本日開催',
    badgeClass: 'bg-rose-500/20 text-rose-300 border-rose-500/30',
    phase: 'pre_live',
  };
}

export interface DeadlineInfo {
  formattedDeadline: string; // e.g. "11/22(日)まで"
  daysLeft: number;
  countdownText: string;     // e.g. "あと23日", "本日締切", "受付終了"
  isExpired: boolean;
  isUrgent: boolean;
}

export function parseDeadlineDate(deadlineStr: string): { year: number; month: number; day: number } | null {
  if (!deadlineStr) return null;
  const matchYMD = deadlineStr.match(/(\d{4})[-/](\d{1,2})[-/](\d{1,2})/);
  if (matchYMD) {
    return {
      year: parseInt(matchYMD[1], 10),
      month: parseInt(matchYMD[2], 10),
      day: parseInt(matchYMD[3], 10),
    };
  }
  const matchMD = deadlineStr.match(/(\d{1,2})[-/](\d{1,2})/);
  if (matchMD) {
    const currentYear = new Date().getFullYear();
    return {
      year: currentYear,
      month: parseInt(matchMD[1], 10),
      day: parseInt(matchMD[2], 10),
    };
  }
  return null;
}

export interface DeadlineOptions {
  suffix?: 'まで' | '開催';
}

/**
 * 締め切りまたは開催日文字列（YYYY-MM-DD 等）から残日数を計算する
 */
export function getDeadlineInfo(
  deadlineStr?: string,
  nowDate = new Date(),
  options: DeadlineOptions = {}
): DeadlineInfo | null {
  if (!deadlineStr) return null;
  const parsed = parseDeadlineDate(deadlineStr);
  if (!parsed) {
    return {
      formattedDeadline: deadlineStr,
      daysLeft: 0,
      countdownText: '受付中',
      isExpired: false,
      isUrgent: false,
    };
  }

  const { year, month, day } = parsed;
  const targetUtc = Date.UTC(year, month - 1, day);
  const nowUtc = Date.UTC(nowDate.getFullYear(), nowDate.getMonth(), nowDate.getDate());
  const msPerDay = 24 * 60 * 60 * 1000;
  const daysLeft = Math.round((targetUtc - nowUtc) / msPerDay);

  const targetDateObj = new Date(year, month - 1, day);
  const dayOfWeek = DAY_NAMES[targetDateObj.getDay()];
  const isEvent = options.suffix === '開催';
  const suffix = options.suffix || 'まで';
  const formattedDeadline = `${month}/${day}(${dayOfWeek})${suffix}`;

  let countdownText = '';
  let isExpired = false;
  let isUrgent = false;

  if (daysLeft < 0) {
    countdownText = isEvent ? '終了' : '受付終了';
    isExpired = true;
  } else if (daysLeft === 0) {
    countdownText = isEvent ? '本日開催！' : '本日締切';
    isUrgent = true;
  } else if (daysLeft === 1) {
    countdownText = isEvent ? '明日開催！' : 'あと1日 (明日締切)';
    isUrgent = true;
  } else {
    countdownText = `あと${daysLeft}日`;
    if (daysLeft <= 3) {
      isUrgent = true;
    }
  }

  return {
    formattedDeadline,
    daysLeft,
    countdownText,
    isExpired,
    isUrgent,
  };
}

export interface FreshnessInfo {
  label: string;
  isStale: boolean;
}

/**
 * データ鮮度表示プロトコル (ADR-0008)
 * オンライン正常取得時は配信側の最新データであるため目障りな警告は表示しない。
 * オフライン・通信失敗によるフォールバック時のみ、データの古さを明示する。
 */
export function getFreshnessInfo(params: {
  isFallback: boolean;
  generatedAt?: string;
  nowDate?: Date;
}): FreshnessInfo | null {
  const { isFallback, generatedAt, nowDate = new Date() } = params;

  if (!isFallback) {
    return null;
  }

  if (!generatedAt) {
    return {
      label: 'オフライン保存版を表示中',
      isStale: false,
    };
  }

  try {
    const genTime = new Date(generatedAt).getTime();
    if (isNaN(genTime)) {
      return {
        label: 'オフライン保存版を表示中',
        isStale: false,
      };
    }

    const diffHours = (nowDate.getTime() - genTime) / (1000 * 60 * 60);
    if (diffHours >= 24) {
      const days = Math.floor(diffHours / 24);
      return {
        label: `オフライン保存版（${days}日前）`,
        isStale: true,
      };
    }

    return {
      label: 'オフライン保存版を表示中',
      isStale: false,
    };
  } catch {
    return {
      label: 'オフライン保存版を表示中',
      isStale: false,
    };
  }
}

/**
 * セッションが終了済みか判定する
 * 1. 過去日付なら終了 (true)
 * 2. 未来日付なら未終了 (false)
 * 3. 本日公演の場合：特典会終了（または出番終了）時刻を実質現在時刻（遅延オフセット適用後）が
 *    超えた時点で直ちに終了（past=true）と判定する。
 */
export function isSessionPast(
  session: {
    date: string;
    live?: { start_time?: string; end_time?: string };
    benefit?: { start_time?: string; end_time?: string };
  },
  nowDate = new Date(),
  graceHours = 0,
  delayMinutes = 0
): boolean {
  const todayStr = getTodayStr(nowDate);

  if (session.date < todayStr) {
    return true;
  }
  if (session.date > todayStr) {
    return false;
  }

  const bEnd = parseMinutes(session.benefit?.end_time);
  const lEnd = parseMinutes(session.live?.end_time);
  const effectiveEnd = bEnd !== null ? bEnd : lEnd;

  if (effectiveEnd === null) {
    return false;
  }

  const currentMinutes = nowDate.getHours() * 60 + nowDate.getMinutes() - delayMinutes;
  const graceMinutes = graceHours * 60;

  return currentMinutes > effectiveEnd + graceMinutes;
}

/**
 * 次回直近ライブセッションを決定する
 * 1. 指定された currentNextId があり、未終了ならそれを優先
 * 2. 当該セッションが終了（本日終了）した場合は即座に次回以降の未終了セッションへ自動遷移
 * 3. 未終了のセッションの中から最も近い日時・出番時刻のセッションを抽出
 * 4. 全て終了している場合は直近の最新セッションをフォールバック
 */
export function resolveNextSession<T extends {
  id: string;
  date: string;
  live?: { start_time?: string; end_time?: string };
  benefit?: { start_time?: string; end_time?: string };
}>(
  sessions: T[],
  currentNextId?: string,
  nowDate = new Date(),
  graceHours = 0,
  delayMap?: { [id: string]: number }
): T | null {
  if (!sessions || sessions.length === 0) return null;

  if (currentNextId) {
    const current = sessions.find((s) => s.id === currentNextId);
    const delay = delayMap?.[currentNextId] ?? 0;
    if (current && !isSessionPast(current, nowDate, graceHours, delay)) {
      return current;
    }
  }

  const upcoming = sessions.filter((s) => {
    const delay = delayMap?.[s.id] ?? 0;
    return !isSessionPast(s, nowDate, graceHours, delay);
  });

  if (upcoming.length > 0) {
    return upcoming.slice().sort((a, b) => {
      if (a.date !== b.date) return a.date.localeCompare(b.date);
      const aStart = parseMinutes(a.live?.start_time) ?? 9999;
      const bStart = parseMinutes(b.live?.start_time) ?? 9999;
      return aStart - bStart;
    })[0];
  }

  return sessions[sessions.length - 1] ?? sessions[0];
}
