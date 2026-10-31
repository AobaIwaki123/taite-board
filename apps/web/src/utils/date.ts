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
}

export function getSessionProgress(
  date: string,
  liveStart?: string,
  liveEnd?: string,
  benefitStart?: string,
  benefitEnd?: string,
  nowDate = new Date()
): ProgressStatus {
  const todayStr = getTodayStr(nowDate);

  if (date > todayStr) {
    return {
      status: 'upcoming',
      label: '開催予定',
      badgeClass: 'bg-emerald-500/20 text-emerald-400 border-emerald-500/30',
    };
  }
  if (date < todayStr) {
    return {
      status: 'ended',
      label: '終了',
      badgeClass: 'bg-zinc-800 text-zinc-400 border-zinc-700',
    };
  }

  // 当日の時間比較
  const currentMinutes = nowDate.getHours() * 60 + nowDate.getMinutes();

  const lStart = parseMinutes(liveStart);
  const lEnd = parseMinutes(liveEnd);
  const bStart = parseMinutes(benefitStart);
  const bEnd = parseMinutes(benefitEnd);

  if (lStart !== null && lEnd !== null && currentMinutes >= lStart && currentMinutes <= lEnd) {
    return {
      status: 'live',
      label: 'ライブ中',
      badgeClass: 'bg-rose-500 text-white animate-pulse font-black',
    };
  }

  if (bStart !== null && bEnd !== null && currentMinutes >= bStart && currentMinutes <= bEnd) {
    return {
      status: 'benefit',
      label: '特典会中',
      badgeClass: 'bg-purple-600 text-white animate-pulse font-black',
    };
  }

  if (bEnd !== null && currentMinutes > bEnd) {
    return {
      status: 'ended',
      label: '本日終了',
      badgeClass: 'bg-zinc-800 text-zinc-400 border-zinc-700',
    };
  }

  return {
    status: 'upcoming',
    label: '本日開催',
    badgeClass: 'bg-rose-500/20 text-rose-300 border-rose-500/30',
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
 * 当日の場合は、特典会終了時刻（または出番終了時刻）から指定の猶予時間（graceHours、デフォルト2時間）が
 * 経過した時点で終了とみなす。
 */
export function isSessionPast(
  session: {
    date: string;
    live?: { start_time?: string; end_time?: string };
    benefit?: { start_time?: string; end_time?: string };
  },
  nowDate = new Date(),
  graceHours = 2
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

  const currentMinutes = nowDate.getHours() * 60 + nowDate.getMinutes();
  const graceMinutes = graceHours * 60;

  return currentMinutes > effectiveEnd + graceMinutes;
}

/**
 * 次回直近ライブセッションを決定する
 * 1. 指定された currentNextId があり、未終了ならそれを優先
 * 2. 未終了のセッションの中から最も近い日時・出番時刻のセッションを抽出
 * 3. 全て終了している場合は先頭のセッションをフォールバック
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
  graceHours = 2
): T | null {
  if (!sessions || sessions.length === 0) return null;

  if (currentNextId) {
    const current = sessions.find((s) => s.id === currentNextId);
    if (current && !isSessionPast(current, nowDate, graceHours)) {
      return current;
    }
  }

  const upcoming = sessions.filter((s) => !isSessionPast(s, nowDate, graceHours));
  if (upcoming.length > 0) {
    return upcoming.slice().sort((a, b) => {
      if (a.date !== b.date) return a.date.localeCompare(b.date);
      const aStart = parseMinutes(a.live?.start_time) ?? 9999;
      const bStart = parseMinutes(b.live?.start_time) ?? 9999;
      return aStart - bStart;
    })[0];
  }

  return sessions[0];
}
