/**
 * Monthly Calendar Component (Slide 3)
 */

import { state } from '../state.ts';
import { Session } from '../types/schedule.ts';
import { openSessionDetail } from './scheduleList.ts';
import { refreshLucideIcons } from '../utils/icons.ts';

export function changeCalendarMonth(delta: number): void {
  state.currentCalMonth += delta;
  if (state.currentCalMonth > 12) {
    state.currentCalMonth = 1;
    state.currentCalYear++;
  } else if (state.currentCalMonth < 1) {
    state.currentCalMonth = 12;
    state.currentCalYear--;
  }
  renderCalendar(state.currentCalYear, state.currentCalMonth);
}

export function renderCalendar(year: number, month: number): void {
  const headerTitle = document.getElementById('calendar-header-title');
  if (headerTitle) headerTitle.textContent = `${year}年 ${month}月`;

  const prevLabel = document.getElementById('prev-month-label');
  const nextLabel = document.getElementById('next-month-label');
  if (prevLabel) prevLabel.textContent = `${month === 1 ? 12 : month - 1}月`;
  if (nextLabel) nextLabel.textContent = `${month === 12 ? 1 : month + 1}月`;

  const grid = document.getElementById('calendar-cells');
  if (!grid) return;
  grid.innerHTML = '';

  // 月の日数と1日の曜日インデックスを取得 (0 = Sun)
  const firstDayIndex = new Date(year, month - 1, 1).getDay();
  const daysInMonth = new Date(year, month, 0).getDate();

  // 前月の空白埋め
  for (let i = 0; i < firstDayIndex; i++) {
    const blank = document.createElement('div');
    blank.className = 'py-2.5 text-stone-700';
    grid.appendChild(blank);
  }

  // 日付セルの描画
  for (let day = 1; day <= daysInMonth; day++) {
    const monthStr = String(month).padStart(2, '0');
    const dayStr = String(day).padStart(2, '0');
    const dateStr = `${year}-${monthStr}-${dayStr}`;
    const sessions = state.appData ? state.appData.sessions.filter(s => s.date === dateStr) : [];
    const hasLive = sessions.length > 0;

    const cell = document.createElement('button');
    cell.className = `py-2 rounded-xl flex flex-col items-center justify-center transition-all ${
      hasLive 
        ? 'bg-rose-950/50 border-2 border-rose-500/60 text-white font-extrabold shadow-sm scale-105' 
        : 'bg-stone-900/30 text-stone-500 hover:text-stone-400'
    }`;

    cell.innerHTML = `
      <span class="text-xs font-display">${day}</span>
      ${hasLive ? `<span class="w-1.5 h-1.5 rounded-full mt-1 bg-rose-400 animate-pulse"></span>` : '<span class="h-1.5 mt-1"></span>'}
    `;

    if (hasLive) {
      cell.onclick = () => renderCalendarDetail(dateStr, sessions);
    }
    grid.appendChild(cell);
  }

  // 詳細カードのクリア
  const detailContainer = document.getElementById('calendar-day-detail');
  if (detailContainer) {
    detailContainer.innerHTML = '<div class="text-xs text-stone-400 text-center py-2">日付をタップすると詳細が表示されます</div>';
  }

  refreshLucideIcons();
}

export function renderCalendarDetail(_dateStr: string, sessions: Session[]): void {
  const container = document.getElementById('calendar-day-detail');
  if (!container) return;
  container.innerHTML = '';

  sessions.forEach(s => {
    const div = document.createElement('div');
    div.className = 'soft-card rounded-xl p-3 space-y-1.5 border-l-4 border-l-rose-500 cursor-pointer hover:border-rose-400/40 transition-all';
    div.onclick = () => openSessionDetail(s.id, 'cal');

    const sessionTag = sessions.length > 1 ? ((s.session_name && s.session_name.includes('昼')) ? ' [昼公演]' : ' [夜公演]') : '';
    div.innerHTML = `
      <div class="flex items-center justify-between text-xs">
        <span class="font-display font-bold text-rose-300">${s.date}${sessionTag}</span>
        <span class="text-stone-300 text-xs">${s.venue.name}</span>
      </div>
      <div class="text-sm font-bold text-white pt-0.5">${s.title}</div>
      <div class="text-xs font-display text-stone-300 flex justify-between pt-1.5 border-t border-white/5">
        <span>出演: <strong class="text-white font-bold">${s.live.start_time}〜</strong></span>
        <span>物販: <strong class="text-white font-bold">${s.benefit.start_time}〜</strong> (${s.benefit.type})</span>
      </div>
    `;
    container.appendChild(div);
  });

  refreshLucideIcons();
}
