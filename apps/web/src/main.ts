/**
 * Ribbon Clock (taite-board) - Main Application Bootstrap
 */

import './styles/main.css';
import { state, loadScheduleData, updateFreshnessIndicator } from './state.ts';
import { renderCockpit } from './components/cockpit.ts';
import { renderUpcomingList, openSessionDetail } from './components/scheduleList.ts';
import { renderCalendar, changeCalendarMonth } from './components/calendar.ts';
import { renderPromotion } from './components/promotion.ts';
import { setTab, initCarouselSync, getCurrentTab, TabName } from './components/carousel.ts';
import { openLightbox, closeLightbox, initLightboxEvents } from './components/lightbox.ts';
import { refreshLucideIcons } from './utils/icons.ts';

export function handleLogoClick(): void {
  // 1. リボンアイコンの弾けるマイクロアニメーション
  const icon = document.getElementById('logo-icon');
  if (icon) {
    icon.classList.remove('animate-ribbon-pop');
    void icon.offsetWidth;
    icon.classList.add('animate-ribbon-pop');
  }

  // 2. コックピットカードのリフレッシュ・アニメーション
  const cockpit = document.getElementById('tab-content-next');
  if (cockpit) {
    cockpit.classList.remove('animate-card-refresh');
    void cockpit.offsetWidth;
    cockpit.classList.add('animate-card-refresh');
  }

  // 3. 直近ライブへジャンプ
  goToNextLive();
}

export function goToNextLive(): void {
  if (!state.appData) return;
  const nextId = state.appData.current_next_id || (state.appData.sessions[0] && state.appData.sessions[0].id);
  if (!nextId) return;
  renderCockpit(nextId);
  const nextSlide = document.getElementById('tab-content-next');
  if (nextSlide) nextSlide.scrollTo({ top: 0, behavior: 'smooth' });
  setTab('next');
}

export function goBackToList(): void {
  const target = state.previousTab === 'cal' ? 'month' : 'week';
  setTab(target);
}

export function handleFirstTabClick(): void {
  if (!state.appData) return;
  const nextId = state.appData.current_next_id || (state.appData.sessions[0] && state.appData.sessions[0].id);
  const nextSlide = document.getElementById('tab-content-next');
  
  // すでに第1タブを表示中で、かつ別日程を閲覧中の場合のみ、タップで直近ライブへリセット
  if (getCurrentTab() === 'next' && state.currentId !== nextId) {
    if (nextId) renderCockpit(nextId);
    if (nextSlide) nextSlide.scrollTo({ top: 0, behavior: 'smooth' });
    return;
  }
  
  // 通常のタブ遷移時は、閲覧中の状態（選んだ公演）をそのまま維持してスムーズに移動
  setTab('next');
}

async function init(): Promise<void> {
  try {
    await loadScheduleData();
    updateFreshnessIndicator();

    if (state.currentId) {
      renderCockpit(state.currentId);
    }
    renderUpcomingList();
    renderCalendar(state.currentCalYear, state.currentCalMonth);
    renderPromotion('large');

    initCarouselSync();
    initLightboxEvents();
    refreshLucideIcons();

    // 初期タブの設定
    setTab('next');
  } catch (err) {
    console.error('Failed to initialize Ribbon Clock app', err);
  }
}

// Global window exposure for HTML events & inline handlers
declare global {
  interface Window {
    handleLogoClick: typeof handleLogoClick;
    goToNextLive: typeof goToNextLive;
    goBackToList: typeof goBackToList;
    handleFirstTabClick: typeof handleFirstTabClick;
    changeCalendarMonth: typeof changeCalendarMonth;
    setTab: (tab: TabName) => void;
    openLightbox: typeof openLightbox;
    closeLightbox: typeof closeLightbox;
    openSessionDetail: typeof openSessionDetail;
  }
}

window.handleLogoClick = handleLogoClick;
window.goToNextLive = goToNextLive;
window.goBackToList = goBackToList;
window.handleFirstTabClick = handleFirstTabClick;
window.changeCalendarMonth = changeCalendarMonth;
window.setTab = setTab;
window.openLightbox = openLightbox;
window.closeLightbox = closeLightbox;
window.openSessionDetail = openSessionDetail;

// DOM ready hook
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', () => {
    void init();
  });
} else {
  void init();
}
