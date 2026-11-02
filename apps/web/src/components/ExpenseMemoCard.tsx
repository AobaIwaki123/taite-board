import { useState, useEffect } from 'preact/hooks';
import type { Session } from '../types/schedule';
import {
  CHEKI_SIGNED_PRICE,
  CHEKI_REGULAR_PRICE,
  DRINK_FEE_OPTIONS,
  type DrinkFeeOption,
  type SessionExpense,
  defaultSessionExpense,
  calculateExpenseTotal,
  getSessionExpense,
  saveSessionExpense,
  deleteSessionExpense,
  parseTicketFee,
  isIncrementalFee,
} from '../utils/expense';
import { Icon } from './Icon';
import { Wallet, ChevronDown, ChevronUp, Minus, Plus, Trash2 } from '../utils/icons';

interface ExpenseMemoCardProps {
  session: Session;
  isAttended: boolean;
}

export function ExpenseMemoCard({ session, isAttended }: ExpenseMemoCardProps) {
  const [expense, setExpense] = useState<SessionExpense>(() => {
    return getSessionExpense(session.id) || defaultSessionExpense();
  });

  const total = calculateExpenseTotal(expense);
  const hasData = total > 0 || (expense.memo && expense.memo.trim() !== '');

  // 初期状態: データがあれば開く、または参戦中のときは開く
  const [isOpen, setIsOpen] = useState<boolean>(() => hasData || isAttended);

  // セッション切り替え時に状態を再読み込み
  useEffect(() => {
    const loaded = getSessionExpense(session.id) || defaultSessionExpense();
    setExpense(loaded);
    const loadedHasData = calculateExpenseTotal(loaded) > 0 || (loaded.memo && loaded.memo.trim() !== '');
    if (loadedHasData) {
      setIsOpen(true);
    }
  }, [session.id]);

  const updateExpense = (updater: (prev: SessionExpense) => SessionExpense) => {
    setExpense((prev) => {
      const next = updater(prev);
      saveSessionExpense(session.id, next);
      return next;
    });
  };

  const handleReset = () => {
    if (window.confirm('この公演の現場メモ・出費をクリアしますか？')) {
      deleteSessionExpense(session.id);
      setExpense(defaultSessionExpense());
    }
  };

  const ticketPrices = (session.ticket?.prices || [])
    .filter((p) => !isIncrementalFee(p.price))
    .map((p) => ({
      label: p.tier || 'チケット',
      fee: parseTicketFee(p.price),
    }))
    .filter((p) => p.fee > 0);

  const signedTotal = expense.chekiSignedCount * CHEKI_SIGNED_PRICE;
  const regularTotal = expense.chekiRegularCount * CHEKI_REGULAR_PRICE;

  return (
    <div class="soft-card rounded-2xl p-3.5 space-y-3 transition-all border border-white/10 shadow-md">
      {/* ヘッダー・折りたたみトグル */}
      <div
        onClick={() => setIsOpen(!isOpen)}
        class="flex items-center justify-between cursor-pointer select-none group"
      >
        <div class="flex items-center gap-2">
          <div class="w-7 h-7 rounded-lg bg-sky-950/80 border border-sky-500/30 flex items-center justify-center text-sky-400">
            <Icon icon={Wallet} class="w-4 h-4" />
          </div>
          <div>
            <div class="text-xs font-bold text-stone-200 flex items-center gap-1.5">
              <span>現場メモ・簡易家計簿</span>
              {hasData && (
                <span class="w-1.5 h-1.5 rounded-full bg-sky-400" />
              )}
            </div>
            <div class="text-[10px] text-stone-400">自分用端末メモ（非公開）</div>
          </div>
        </div>

        <div class="flex items-center gap-2">
          {total > 0 && (
            <span class="text-xs font-display font-black text-sky-300 bg-sky-950/80 px-2 py-0.5 rounded border border-sky-500/30">
              ¥{total.toLocaleString()}
            </span>
          )}
          <div class="text-stone-400 group-hover:text-stone-200 transition-colors">
            <Icon icon={isOpen ? ChevronUp : ChevronDown} class="w-4 h-4" />
          </div>
        </div>
      </div>

      {/* 展開時フォーム */}
      {isOpen && (
        <div class="space-y-3 pt-2 border-t border-white/5 animate-fade-in text-xs">
          {/* チケット代 */}
          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-stone-400">
              <span class="font-bold">チケット代</span>
              <span class="font-mono text-stone-200">
                {expense.ticketFee > 0 ? `¥${expense.ticketFee.toLocaleString()}` : '¥0'}
              </span>
            </div>

            {/* クイック選択候補 */}
            <div class="flex flex-wrap gap-1.5">
              {ticketPrices.map((tp, idx) => (
                <button
                  key={idx}
                  type="button"
                  onClick={() =>
                    updateExpense((prev) => ({
                      ...prev,
                      ticketFee: tp.fee,
                      ticketLabel: tp.label,
                    }))
                  }
                  class={`px-2 py-1 rounded-lg text-xs font-bold transition-all border ${
                    expense.ticketFee === tp.fee && expense.ticketLabel === tp.label
                      ? 'bg-sky-500 text-stone-950 border-sky-400 font-black shadow-sm'
                      : 'bg-stone-900/80 border-white/10 text-stone-300 hover:text-white'
                  }`}
                >
                  {tp.label} ¥{tp.fee.toLocaleString()}
                </button>
              ))}

              <button
                type="button"
                onClick={() =>
                  updateExpense((prev) => ({
                    ...prev,
                    ticketFee: 0,
                    ticketLabel: '無料',
                  }))
                }
                class={`px-2 py-1 rounded-lg text-xs font-bold transition-all border ${
                  expense.ticketFee === 0 && expense.ticketLabel === '無料'
                    ? 'bg-sky-500 text-stone-950 border-sky-400 font-black shadow-sm'
                    : 'bg-stone-900/80 border-white/10 text-stone-400 hover:text-stone-200'
                }`}
              >
                無料 / 招待
              </button>

              <div class="flex items-center gap-1 ml-auto">
                <span class="text-stone-500 text-[11px]">手入力:</span>
                <input
                  type="number"
                  min="0"
                  step="500"
                  placeholder="0"
                  value={expense.ticketFee || ''}
                  onInput={(e) => {
                    const val = parseInt((e.target as HTMLInputElement).value, 10) || 0;
                    updateExpense((prev) => ({
                      ...prev,
                      ticketFee: Math.max(0, val),
                      ticketLabel: '手入力',
                    }));
                  }}
                  class="w-20 px-2 py-0.5 rounded bg-stone-900 border border-white/10 text-right text-stone-200 font-mono focus:outline-none focus:border-sky-500"
                />
                <span class="text-stone-500 text-[11px]">円</span>
              </div>
            </div>
          </div>

          {/* ドリンク代 (D代: 0 / 500 / 600 / 700) */}
          <div class="space-y-1.5 pt-1 border-t border-white/5">
            <div class="flex items-center justify-between text-stone-400">
              <span class="font-bold">ドリンク代 (D代)</span>
              <span class="font-mono text-stone-200">
                {expense.drinkFee > 0 ? `¥${expense.drinkFee.toLocaleString()}` : '0円 (込)'}
              </span>
            </div>
            <div class="grid grid-cols-4 gap-1.5">
              {DRINK_FEE_OPTIONS.map((fee) => (
                <button
                  key={fee}
                  type="button"
                  onClick={() =>
                    updateExpense((prev) => ({
                      ...prev,
                      drinkFee: fee as DrinkFeeOption,
                    }))
                  }
                  class={`py-1 rounded-lg text-xs font-bold transition-all border text-center ${
                    expense.drinkFee === fee
                      ? 'bg-sky-500 text-stone-950 border-sky-400 font-black shadow-sm'
                      : 'bg-stone-900/80 border-white/10 text-stone-400 hover:text-stone-200'
                  }`}
                >
                  {fee === 0 ? '0円 (込)' : `¥${fee}`}
                </button>
              ))}
            </div>
          </div>

          {/* 特典会・チェキ */}
          <div class="space-y-2 pt-1 border-t border-white/5">
            <div class="flex items-center justify-between text-stone-400">
              <span class="font-bold">チェキ</span>
              <span class="font-mono text-stone-200">
                ¥{(signedTotal + regularTotal).toLocaleString()}
              </span>
            </div>

            {/* サイン有り (¥2,000) */}
            <div class="flex items-center justify-between p-2 rounded-xl bg-stone-900/50 border border-white/5">
              <div>
                <div class="font-semibold text-stone-200">サイン有</div>
                <div class="text-[10px] text-stone-500">¥2,000 / 枚</div>
              </div>
              <div class="flex items-center gap-3">
                <span class="font-mono text-xs text-stone-300 w-14 text-right">
                  ¥{signedTotal.toLocaleString()}
                </span>
                <div class="flex items-center gap-1.5 bg-stone-950 p-0.5 rounded-lg border border-white/10">
                  <button
                    type="button"
                    onClick={() =>
                      updateExpense((prev) => ({
                        ...prev,
                        chekiSignedCount: Math.max(0, prev.chekiSignedCount - 1),
                      }))
                    }
                    disabled={expense.chekiSignedCount <= 0}
                    class="w-6 h-6 flex items-center justify-center rounded text-stone-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed"
                  >
                    <Icon icon={Minus} class="w-3.5 h-3.5" />
                  </button>
                  <span class="w-5 text-center font-mono font-bold text-white text-xs">
                    {expense.chekiSignedCount}
                  </span>
                  <button
                    type="button"
                    onClick={() =>
                      updateExpense((prev) => ({
                        ...prev,
                        chekiSignedCount: prev.chekiSignedCount + 1,
                      }))
                    }
                    class="w-6 h-6 flex items-center justify-center rounded text-sky-400 hover:text-sky-300 hover:bg-sky-950/50"
                  >
                    <Icon icon={Plus} class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            </div>

            {/* サイン無し (¥1,000) */}
            <div class="flex items-center justify-between p-2 rounded-xl bg-stone-900/50 border border-white/5">
              <div>
                <div class="font-semibold text-stone-200">サイン無</div>
                <div class="text-[10px] text-stone-500">¥1,000 / 枚</div>
              </div>
              <div class="flex items-center gap-3">
                <span class="font-mono text-xs text-stone-300 w-14 text-right">
                  ¥{regularTotal.toLocaleString()}
                </span>
                <div class="flex items-center gap-1.5 bg-stone-950 p-0.5 rounded-lg border border-white/10">
                  <button
                    type="button"
                    onClick={() =>
                      updateExpense((prev) => ({
                        ...prev,
                        chekiRegularCount: Math.max(0, prev.chekiRegularCount - 1),
                      }))
                    }
                    disabled={expense.chekiRegularCount <= 0}
                    class="w-6 h-6 flex items-center justify-center rounded text-stone-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed"
                  >
                    <Icon icon={Minus} class="w-3.5 h-3.5" />
                  </button>
                  <span class="w-5 text-center font-mono font-bold text-white text-xs">
                    {expense.chekiRegularCount}
                  </span>
                  <button
                    type="button"
                    onClick={() =>
                      updateExpense((prev) => ({
                        ...prev,
                        chekiRegularCount: prev.chekiRegularCount + 1,
                      }))
                    }
                    class="w-6 h-6 flex items-center justify-center rounded text-sky-400 hover:text-sky-300 hover:bg-sky-950/50"
                  >
                    <Icon icon={Plus} class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            </div>
          </div>

          {/* 自由メモ */}
          <div class="space-y-1 pt-1 border-t border-white/5">
            <span class="font-bold text-stone-400">現場メモ</span>
            <input
              type="text"
              placeholder="例: 特典動画券使用、チェキ2ループ、整理番号A50等"
              value={expense.memo}
              onInput={(e) => {
                const val = (e.target as HTMLInputElement).value;
                updateExpense((prev) => ({ ...prev, memo: val }));
              }}
              class="w-full px-2.5 py-1.5 rounded-xl bg-stone-900 border border-white/10 text-stone-200 text-xs focus:outline-none focus:border-sky-500 placeholder:text-stone-600"
            />
          </div>

          {/* 合計サマリーバー */}
          <div class="pt-2 border-t border-white/10 flex items-center justify-between">
            <div class="flex items-center gap-2">
              <span class="text-stone-400 font-bold">公演合計出費:</span>
              <span class="text-base font-display font-black text-sky-400">
                ¥{total.toLocaleString()}
              </span>
            </div>

            {hasData && (
              <button
                type="button"
                onClick={handleReset}
                class="flex items-center gap-1 text-[11px] text-stone-500 hover:text-rose-400 transition-colors"
              >
                <Icon icon={Trash2} class="w-3 h-3" />
                <span>クリア</span>
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
