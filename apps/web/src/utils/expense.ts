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
