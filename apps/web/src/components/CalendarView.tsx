import { useState } from 'preact/hooks';
import { Session } from '../types/schedule';
import { Icon } from './Icon';
import { ChevronLeft, ChevronRight } from '../utils/icons';

interface CalendarViewProps {
  sessions: Session[];
  onSelectSession: (id: string) => void;
}

export function CalendarView({ sessions, onSelectSession }: CalendarViewProps) {
  const [year, setYear] = useState(2026);
  const [month, setMonth] = useState(11); // 1-12
  const [selectedDate, setSelectedDate] = useState<string | null>(null);

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

      {/* 曜日ヘッダー */}
      <div class="grid grid-cols-7 gap-1 text-center text-[11px] font-bold text-stone-400 py-1 border-b border-white/5">
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
          const isSelected = selectedDate === dateStr;

          return (
            <button
              key={dateStr}
              onClick={() => {
                if (hasLive) setSelectedDate(dateStr);
              }}
              disabled={!hasLive}
              class={`py-2 rounded-xl flex flex-col items-center justify-center transition-all ${
                isSelected
                  ? 'bg-rose-500 text-stone-950 font-black shadow-md scale-105'
                  : hasLive
                  ? 'bg-rose-950/50 border border-rose-500/60 text-white font-bold hover:bg-rose-900/60'
                  : 'bg-stone-900/30 text-stone-600 cursor-default'
              }`}
            >
              <span class="text-xs font-display">{day}</span>
              {hasLive ? (
                <span
                  class={`w-1.5 h-1.5 rounded-full mt-1 ${
                    isSelected ? 'bg-stone-950' : 'bg-rose-400 animate-pulse'
                  }`}
                />
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
            selectedDaySessions.map((s) => (
              <div
                key={s.id}
                onClick={() => onSelectSession(s.id)}
                class="soft-card rounded-xl p-3 space-y-1.5 border-l-4 border-l-rose-500 cursor-pointer hover:border-rose-400/40 transition-all active:scale-[0.99]"
              >
                <div class="flex items-center justify-between text-xs">
                  <span class="font-display font-bold text-rose-300">
                    {s.date} {s.session_name ? `[${s.session_name}]` : ''}
                  </span>
                  <span class="text-stone-300 text-xs">{s.venue.name}</span>
                </div>
                <div class="text-sm font-bold text-white pt-0.5">{s.title}</div>
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
            ))
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
