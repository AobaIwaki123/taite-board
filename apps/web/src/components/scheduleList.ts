/**
 * Upcoming Schedule List Component (Slide 2)
 */

import { state } from '../state.ts';
import { TODAY_STR } from '../utils/date.ts';
import { renderCockpit } from './cockpit.ts';
import { setTab } from './carousel.ts';
import { refreshLucideIcons } from '../utils/icons.ts';

export function openSessionDetail(id: string, fromTab: 'week' | 'all' | 'cal' = 'week'): void {
  state.previousTab = fromTab;
  renderCockpit(id);
  setTab('next');
}

export function renderUpcomingList(): void {
  const container = document.getElementById('upcoming-list');
  if (!container || !state.appData) return;
  container.innerHTML = '';

  // 今後の公演のみ抽出
  const upcoming = state.appData.sessions.filter(s => s.date >= TODAY_STR);
  
  const countBadge = document.getElementById('upcoming-count-badge');
  if (countBadge) countBadge.textContent = `${upcoming.length}公演`;

  upcoming.forEach(s => {
    const card = document.createElement('div');
    card.className = 'soft-card rounded-2xl p-3 flex items-center gap-3.5 cursor-pointer hover:border-rose-400/40 transition-all shadow-md';
    card.onclick = () => openSessionDetail(s.id, 'week');

    // 日付・月・曜日の抽出
    const [, mStr, dStr] = s.date.split('-');
    const monthNum = parseInt(mStr, 10);
    const dayNum = parseInt(dStr, 10);
    
    // 曜日カラー
    let dayColor = 'text-stone-400';
    if (s.day_of_week === '土') dayColor = 'text-blue-400';
    if (s.day_of_week === '日' || (s.day_of_week && s.day_of_week.includes('祝'))) dayColor = 'text-rose-400';

    const sameDayCount = state.appData ? state.appData.sessions.filter(x => x.date === s.date).length : 1;
    const sessionTag = sameDayCount > 1 
      ? ((s.session_name && s.session_name.includes('昼')) ? ' [昼]' : ' [夜]') 
      : '';

    card.innerHTML = `
      <!-- カレンダー風・日付アンカーブロック -->
      <div class="w-[52px] h-[56px] rounded-xl bg-stone-950 border border-white/10 flex flex-col items-center justify-center shrink-0 shadow-inner">
        <span class="text-[10px] font-bold text-stone-400 leading-none">${monthNum}月</span>
        <span class="text-xl font-display font-black text-white leading-tight my-0.5">${dayNum}</span>
        <span class="text-[10px] font-bold ${dayColor} leading-none">${s.day_of_week || ''}</span>
      </div>

      <!-- イベント情報本体 -->
      <div class="flex-1 min-w-0 space-y-1">
        <div class="flex items-center justify-between text-xs">
          <span class="text-stone-300 font-semibold truncate flex items-center gap-1">
            <i data-lucide="map-pin" class="w-3 h-3 text-rose-400 shrink-0"></i>
            <span class="truncate">${s.venue.name}</span>
          </span>
          ${sessionTag ? `<span class="px-1.5 py-0.2 rounded bg-rose-500/20 text-rose-300 font-bold text-[10px] shrink-0">${sessionTag}</span>` : ''}
        </div>

        <div class="text-sm font-extrabold text-white leading-snug line-clamp-2">${s.title}</div>

        <div class="text-xs flex items-center justify-between pt-1 border-t border-white/5 text-stone-300">
          <span class="flex items-center gap-1">
            <span class="text-rose-400 font-bold">出番</span>
            <strong class="text-white font-display font-bold">${s.live.start_time}〜</strong>
          </span>
          <span class="flex items-center gap-1 text-stone-400">
            <span>特典会</span>
            <strong class="text-stone-200 font-display">${s.benefit.start_time}〜</strong>
          </span>
        </div>
      </div>
    `;

    container.appendChild(card);
  });

  refreshLucideIcons();
}
