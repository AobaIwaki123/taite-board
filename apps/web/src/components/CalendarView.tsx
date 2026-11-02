import { useState } from 'preact/hooks';
import { Session } from '../types/schedule';
import { isSessionPast } from '../utils/date';
import {
  calculateMonthExpenseSummary,
  calculateExpenseTotal,
  getSessionExpense,
} from '../utils/expense';
import { Icon } from './Icon';
import { ChevronLeft, ChevronRight, ChevronDown, ChevronUp, Wallet } from '../utils/icons';

interface CalendarViewProps {
  sessions: Session[];
  attendedSessionIds?: string[];
  onSelectSession: (id: string) => void;
}

export function CalendarView({ sessions, attendedSessionIds = [], onSelectSession }: CalendarViewProps) {
  const today = new Date();
  const [year, setYear] = useState(today.getFullYear());
  const [month, setMonth] = useState(today.getMonth() + 1); // 1-12
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const [showWeeklyDetail, setShowWeeklyDetail] = useState(false);

  const handlePrevMonth = () => {
    if (month === 1) {
      setYear((y) => y - 1);
      setMonth(12);
    } else {
      setMonth((m) => m - 1);
    }
    setSelectedDate(null);
  };

  const handleNextMonth = () => {
    if (month === 12) {
      setYear((y) => y + 1);
      setMonth(1);
    } else {
      setMonth((m) => m + 1);
    }
    setSelectedDate(null);
  };

  // 月の日数と1日の曜日インデックス (0 = Sun)
  const firstDayIndex = new Date(year, month - 1, 1).getDay();
  const daysInMonth = new Date(year, month, 0).getDate();

  // 月間・週間出費集計
  const monthSummary = calculateMonthExpenseSummary(year, month, sessions);

  // 選択日の公演
  const selectedDaySessions = selectedDate
    ? sessions.filter((s) => s.date === selectedDate)
    : [];

  return (
    <div class="h-full overflow-y-auto no-scrollbar tab-slide space-y-3.5 pb-6">
      {/* 月送りナビゲーション */}
      <div class="flex items-center justify-between px-1">
        <button
          onClick={handlePrevMonth}
          class="flex items-center gap-1 px-3 py-1.5 rounded-xl bg-stone-900 border border-white/10 text-stone-300 hover:text-white text-xs font-bold transition-colors"
        >
          <Icon icon={ChevronLeft} class="w-3.5 h-3.5" />
          <span>{month === 1 ? 12 : month - 1}月</span>
        </button>

        <h2 class="text-sm font-black text-white font-display">
          {year}年 {month}月
        </h2>

        <button
          onClick={handleNextMonth}
          class="flex items-center gap-1 px-3 py-1.5 rounded-xl bg-stone-900 border border-white/10 text-stone-300 hover:text-white text-xs font-bold transition-colors"
        >
          <span>{month === 12 ? 1 : month + 1}月</span>
          <Icon icon={ChevronRight} class="w-3.5 h-3.5" />
        </button>
      </div>

      {/* 月間・週間出費サマリーカード */}
      {monthSummary.total > 0 && (
        <div class="soft-card rounded-2xl p-3 space-y-2 border border-sky-500/20 bg-stone-900/60 shadow-md">
          <div
            onClick={() => setShowWeeklyDetail(!showWeeklyDetail)}
            class="flex items-center justify-between cursor-pointer select-none"
          >
            <div class="flex items-center gap-1.5 text-xs text-stone-300 font-bold">
              <Icon icon={Wallet} class="w-3.5 h-3.5 text-sky-400" />
              <span>{month}月の推し活出費</span>
              <span class="text-[10px] text-stone-400 font-normal">
                ({monthSummary.recordedSessionCount}公演記録)
              </span>
            </div>
            <div class="flex items-center gap-1.5">
              <span class="text-sm font-display font-black text-sky-300">
                ¥{monthSummary.total.toLocaleString()}
              </span>
              <Icon
                icon={showWeeklyDetail ? ChevronUp : ChevronDown}
                class="w-3.5 h-3.5 text-stone-500"
              />
            </div>
          </div>

          {/* 内訳チップス */}
          <div class="grid grid-cols-3 gap-1 pt-1 border-t border-white/5 text-[11px]">
            <div class="p-1.5 rounded-lg bg-stone-950/60 border border-white/5">
              <div class="text-[10px] text-stone-400">チケット</div>
              <div class="font-mono font-bold text-stone-200">
                ¥{monthSummary.ticketTotal.toLocaleString()}
              </div>
            </div>
            <div class="p-1.5 rounded-lg bg-stone-950/60 border border-white/5">
              <div class="text-[10px] text-stone-400">ドリンク (D代)</div>
              <div class="font-mono font-bold text-stone-200">
                ¥{monthSummary.drinkTotal.toLocaleString()}
              </div>
            </div>
            <div class="p-1.5 rounded-lg bg-stone-950/60 border border-white/5">
              <div class="text-[10px] text-stone-400">
                チェキ ({monthSummary.chekiSignedCount + monthSummary.chekiRegularCount}枚)
              </div>
              <div class="font-mono font-bold text-stone-200">
                ¥{monthSummary.chekiTotal.toLocaleString()}
              </div>
            </div>
          </div>

          {/* 週単位の内訳（トグル展開） */}
          {showWeeklyDetail && (
            <div class="pt-2 border-t border-white/5 space-y-1.5 text-xs animate-fade-in">
              <div class="text-[10px] font-bold text-stone-400 uppercase tracking-wider">
                週ごとの小計
              </div>
              <div class="space-y-1">
                {monthSummary.weeks.map((w, idx) => (
                  <div
                    key={idx}
                    class="flex items-center justify-between px-2 py-1 rounded bg-stone-950/40 text-[11px]"
                  >
                    <span class="text-stone-400">
                      第{idx + 1}週 ({month}/{w.startDay}〜{w.endDay}日)
                    </span>
                    <span
                      class={`font-mono font-bold ${
                        w.total > 0 ? 'text-sky-300' : 'text-stone-500'
                      }`}
                    >
                      ¥{w.total.toLocaleString()}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* ローカル保存の消失注意 */}
          <div class="text-[10px] text-stone-300 leading-normal pt-1.5 border-t border-white/5">
            ※ 参戦記録および収支データはお使いの端末（ブラウザ）にのみ保存されます。ブラウザ履歴の消去や機種変更で消失する可能性があります。
          </div>
        </div>
      )}

      {/* 曜日ヘッダー */}
      <div class="grid grid-cols-7 gap-1 text-center text-xs font-bold text-stone-400 py-1.5 border-b border-white/5">
        <span class="text-rose-400">日</span>
        <span>月</span>
        <span>火</span>
        <span>水</span>
        <span>木</span>
        <span>金</span>
        <span class="text-blue-400">土</span>
      </div>

      {/* カレンダーグリッド */}
      <div class="grid grid-cols-7 gap-1.5">
        {/* 前月の空白枠 */}
        {Array.from({ length: firstDayIndex }).map((_, i) => (
          <div key={`blank-${i}`} class="py-2 text-stone-800" />
        ))}

        {/* 日付セル */}
        {Array.from({ length: daysInMonth }).map((_, i) => {
          const day = i + 1;
          const monthStr = String(month).padStart(2, '0');
          const dayStr = String(day).padStart(2, '0');
          const dateStr = `${year}-${monthStr}-${dayStr}`;
          const daySessions = sessions.filter((s) => s.date === dateStr);
          const hasLive = daySessions.length > 0;
          const isAttended = daySessions.some((s) => attendedSessionIds.includes(s.id));
          const hasFutureAttended = daySessions.some(
            (s) => attendedSessionIds.includes(s.id) && !isSessionPast(s)
          );
          const hasExpense = daySessions.some((s) => {
            const exp = getSessionExpense(s.id);
            return exp ? calculateExpenseTotal(exp) > 0 : false;
          });
          const isSelected = selectedDate === dateStr;

          return (
            <button
              key={dateStr}
              type="button"
              onClick={() => {
                if (hasLive) {
                  setSelectedDate(isSelected ? null : dateStr);
                }
              }}
              aria-disabled={!hasLive}
              class={`py-1.5 rounded-xl flex flex-col items-center justify-center transition-all select-none min-h-[44px] ${
                isSelected
                  ? 'bg-rose-500 text-stone-950 font-black shadow-md scale-105 cursor-pointer'
                  : hasLive
                  ? isAttended
                    ? hasFutureAttended
                      ? 'bg-sky-950/50 border border-sky-500/60 text-white font-bold hover:bg-sky-900/60 cursor-pointer'
                      : 'bg-emerald-950/50 border border-emerald-500/60 text-white font-bold hover:bg-emerald-900/60 cursor-pointer'
                    : 'bg-rose-950/50 border border-rose-500/60 text-white font-bold hover:bg-rose-900/60 cursor-pointer'
                  : 'bg-stone-900/30 text-stone-600 cursor-default'
              }`}
            >
              <span class="text-xs font-display">{day}</span>
              {hasLive ? (
                <div class="flex items-center justify-center gap-0.5 mt-0.5">
                  <span
                    class={`w-1.5 h-1.5 rounded-full ${
                      isSelected
                        ? 'bg-stone-950'
                        : isAttended
                        ? hasFutureAttended
                          ? 'bg-sky-400'
                          : 'bg-emerald-400'
                        : 'bg-rose-400 animate-pulse'
                    }`}
                  />
                  {hasExpense && (
                    <span
                      class={`text-[9px] font-bold font-mono leading-none ${
                        isSelected ? 'text-stone-950 font-black' : 'text-sky-300'
                      }`}
                    >
                      ¥
                    </span>
                  )}
                </div>
              ) : (
                <span class="h-1.5 mt-1" />
              )}
            </button>
          );
        })}
      </div>

      {/* 選択日の公演詳細 */}
      <div class="space-y-2 pt-2 border-t border-white/5">
        {selectedDate ? (
          selectedDaySessions.length > 0 ? (
            selectedDaySessions.map((s) => {
              const attended = attendedSessionIds.includes(s.id);
              const isPast = isSessionPast(s);
              const exp = getSessionExpense(s.id);
              const expTotal = exp ? calculateExpenseTotal(exp) : 0;
              return (
                <div
                  key={s.id}
                  onClick={() => onSelectSession(s.id)}
                  class={`soft-card rounded-xl p-3 space-y-1.5 border-l-4 cursor-pointer hover:border-rose-400/40 transition-all active:scale-[0.99] ${
                    attended
                      ? isPast
                        ? 'border-l-emerald-500'
                        : 'border-l-sky-500'
                      : 'border-l-rose-500'
                  }`}
                >
                  {/* ヘッダー行: 日付・部数 と ステータスバッジ（参戦状況・金額） */}
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-xs font-display font-bold text-rose-300">
                      {s.date} {s.session_name ? `[${s.session_name}]` : ''}
                    </span>
                    <div class="flex items-center gap-1.5 flex-shrink-0">
                      {expTotal > 0 && (
                        <span class="px-1.5 py-0.5 rounded text-[10px] font-mono font-bold bg-sky-950/80 text-sky-300 border border-sky-500/30">
                          ¥{expTotal.toLocaleString()}
                        </span>
                      )}
                      {attended && (
                        <span
                          class={`px-1.5 py-0.5 rounded text-[10px] font-bold border ${
                            isPast
                              ? 'bg-emerald-950/80 text-emerald-400 border-emerald-500/30'
                              : 'bg-sky-950/80 text-sky-400 border-sky-500/30'
                          }`}
                        >
                          {isPast ? '参戦済' : '参戦予定'}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* タイトル行 */}
                  <div class="text-sm font-bold text-white pt-0.5 leading-snug">{s.title}</div>

                  {/* 会場名行 */}
                  <div class="text-xs text-stone-300 flex items-center gap-1">
                    <span class="text-stone-500">会場:</span>
                    <span class="truncate">{s.venue.name}</span>
                  </div>
                  <div class="text-xs font-display text-stone-300 flex justify-between pt-1.5 border-t border-white/5">
                    <span>
                      出演: <strong class="text-white font-bold">{s.live.start_time}〜</strong>
                    </span>
                    <span>
                      特典会: <strong class="text-white font-bold">{s.benefit.start_time}〜</strong> (
                      {s.benefit.type})
                    </span>
                  </div>
                </div>
              );
            })
          ) : (
            <div class="text-xs text-stone-500 text-center py-3">この日の公演はありません</div>
          )
        ) : (
          <div class="text-xs text-stone-500 text-center py-3">日付をタップすると詳細が表示されます</div>
        )}
      </div>
    </div>
  );
}
