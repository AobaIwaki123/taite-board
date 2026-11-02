export const CHEKI_SIGNED_PRICE = 2000;
export const CHEKI_REGULAR_PRICE = 1000;
export const DRINK_FEE_OPTIONS = [0, 500, 600, 700] as const;

export type DrinkFeeOption = (typeof DRINK_FEE_OPTIONS)[number];

export interface SessionExpense {
  ticketFee: number;
  ticketLabel?: string; // '前方' | '通常' | '手入力' | '無料' など
  drinkFee: DrinkFeeOption;
  chekiSignedCount: number; // サイン有り (¥2,000)
  chekiRegularCount: number; // サイン無し (¥1,000)
  memo: string;
  updatedAt?: string;
}

const STORAGE_PREFIX = 'ribonclock_expense_';

export function defaultSessionExpense(): SessionExpense {
  return {
    ticketFee: 0,
    ticketLabel: '',
    drinkFee: 600, // 標準的なライブハウスD代
    chekiSignedCount: 0,
    chekiRegularCount: 0,
    memo: '',
  };
}

export function calculateExpenseTotal(expense: Partial<SessionExpense>): number {
  const ticket = Number(expense.ticketFee) || 0;
  const drink = Number(expense.drinkFee) || 0;
  const signed = (Number(expense.chekiSignedCount) || 0) * CHEKI_SIGNED_PRICE;
  const regular = (Number(expense.chekiRegularCount) || 0) * CHEKI_REGULAR_PRICE;
  return ticket + drink + signed + regular;
}

/**
 * チケット価格文字列から正規の金額数値を抽出する。
 * 例: "¥3,200 (+1D)" -> 3200
 *     "¥4,500" -> 4500
 *     "各+¥500 (+1D)" -> 500
 *     "¥0 (無料)" -> 0
 */
export function parseTicketFee(fee?: string | number | null): number {
  if (fee === undefined || fee === null) return 0;
  if (typeof fee === 'number') return fee;
  // 1. 括弧注記（(+1D), (完売), (無料)など）を除去
  const clean = fee.replace(/\([^)]*\)/g, '').replace(/（[^）]*）/g, '');
  // 2. 金額数値を抽出
  const m = clean.match(/([0-9]{1,3}(?:,[0-9]{3})*|[0-9]+)/);
  if (!m) return 0;
  const num = parseInt(m[1].replace(/,/g, ''), 10);
  return isNaN(num) ? 0 : num;
}

/**
 * 当日券などの増分指定（各+¥500, +¥1,000等）かどうかを判定
 */
export function isIncrementalFee(fee?: string | null): boolean {
  if (!fee) return false;
  const trimmed = fee.trim();
  return trimmed.startsWith('+') || trimmed.includes('+¥') || trimmed.includes('各+');
}

export interface WeekExpenseSummary {
  weekIndex: number; // 0, 1, 2, 3, 4, 5 (カレンダー行)
  startDay: number;
  endDay: number;
  total: number;
}

export interface MonthExpenseSummary {
  year: number;
  month: number;
  total: number;
  ticketTotal: number;
  drinkTotal: number;
  chekiTotal: number;
  chekiSignedCount: number;
  chekiRegularCount: number;
  recordedSessionCount: number;
  weeks: WeekExpenseSummary[];
}

export function calculateMonthExpenseSummary(
  year: number,
  month: number,
  sessions: { id: string; date: string }[],
  getExpenseFn: (sessionId: string) => SessionExpense | null = getSessionExpense
): MonthExpenseSummary {
  const monthPrefix = `${year}-${String(month).padStart(2, '0')}`;
  const monthSessions = sessions.filter((s) => s.date.startsWith(monthPrefix));

  let ticketTotal = 0;
  let drinkTotal = 0;
  let chekiTotal = 0;
  let chekiSignedCount = 0;
  let chekiRegularCount = 0;
  let recordedSessionCount = 0;

  // 日付ごとの出費合計をキャッシュ
  const dayExpenseMap: Record<number, number> = {};

  for (const s of monthSessions) {
    const exp = getExpenseFn(s.id);
    if (!exp) continue;
    const sessionTotal = calculateExpenseTotal(exp);
    if (sessionTotal > 0 || exp.memo.trim() !== '') {
      recordedSessionCount++;
    }
    ticketTotal += Number(exp.ticketFee) || 0;
    drinkTotal += Number(exp.drinkFee) || 0;
    chekiSignedCount += Number(exp.chekiSignedCount) || 0;
    chekiRegularCount += Number(exp.chekiRegularCount) || 0;

    const dayNum = parseInt(s.date.split('-')[2], 10);
    dayExpenseMap[dayNum] = (dayExpenseMap[dayNum] || 0) + sessionTotal;
  }

  chekiTotal = chekiSignedCount * CHEKI_SIGNED_PRICE + chekiRegularCount * CHEKI_REGULAR_PRICE;
  const total = ticketTotal + drinkTotal + chekiTotal;

  // カレンダーの行（週）ごとに集計
  // 1日の曜日 (0: 日, 1: 月, ... 6: 土)
  const firstDayOfWeek = new Date(year, month - 1, 1).getDay();
  const daysInMonth = new Date(year, month, 0).getDate();

  const weeks: WeekExpenseSummary[] = [];
  let currentDay = 1;
  let weekIndex = 0;

  while (currentDay <= daysInMonth) {
    const startDay = currentDay;
    // その週の土曜日までの日数（初週は 7 - firstDayOfWeek 日間、以降は最大 7 日間）
    const daysRemainingInWeek = weekIndex === 0 ? 7 - firstDayOfWeek : 7;
    const endDay = Math.min(daysInMonth, startDay + daysRemainingInWeek - 1);

    let weekTotal = 0;
    for (let d = startDay; d <= endDay; d++) {
      weekTotal += dayExpenseMap[d] || 0;
    }

    weeks.push({
      weekIndex,
      startDay,
      endDay,
      total: weekTotal,
    });

    currentDay = endDay + 1;
    weekIndex++;
  }

  return {
    year,
    month,
    total,
    ticketTotal,
    drinkTotal,
    chekiTotal,
    chekiSignedCount,
    chekiRegularCount,
    recordedSessionCount,
    weeks,
  };
}

export function getSessionExpense(sessionId: string): SessionExpense | null {
  if (typeof window === 'undefined' || !window.localStorage) {
    return null;
  }
  try {
    const raw = localStorage.getItem(`${STORAGE_PREFIX}${sessionId}`);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    return {
      ticketFee: Number(parsed.ticketFee) || 0,
      ticketLabel: typeof parsed.ticketLabel === 'string' ? parsed.ticketLabel : '',
      drinkFee: DRINK_FEE_OPTIONS.includes(Number(parsed.drinkFee) as DrinkFeeOption)
        ? (Number(parsed.drinkFee) as DrinkFeeOption)
        : 600,
      chekiSignedCount: Math.max(0, Number(parsed.chekiSignedCount) || 0),
      chekiRegularCount: Math.max(0, Number(parsed.chekiRegularCount) || 0),
      memo: typeof parsed.memo === 'string' ? parsed.memo : '',
      updatedAt: parsed.updatedAt || undefined,
    };
  } catch {
    return null;
  }
}

export function saveSessionExpense(sessionId: string, expense: SessionExpense): void {
  if (typeof window === 'undefined' || !window.localStorage) {
    return;
  }
  try {
    const toSave: SessionExpense = {
      ...expense,
      updatedAt: new Date().toISOString(),
    };
    localStorage.setItem(`${STORAGE_PREFIX}${sessionId}`, JSON.stringify(toSave));
  } catch (e) {
    console.warn('Failed to save session expense to localStorage', e);
  }
}

export function deleteSessionExpense(sessionId: string): void {
  if (typeof window === 'undefined' || !window.localStorage) {
    return;
  }
  try {
    localStorage.removeItem(`${STORAGE_PREFIX}${sessionId}`);
  } catch (e) {
    console.warn('Failed to remove session expense from localStorage', e);
  }
}
