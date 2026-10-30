/**
 * Views Carousel and Navigation Synchronization
 */

export type TabName = 'next' | 'week' | 'month';

export const tabCycle: TabName[] = ['next', 'week', 'month'];
let currentTabName: TabName = 'next';

export function getCurrentTab(): TabName {
  return currentTabName;
}

export function setTab(tab: TabName): void {
  currentTabName = tab;
  const carousel = document.getElementById('views-carousel');
  const idx = tabCycle.indexOf(tab);
  if (carousel && idx !== -1) {
    const slideWidth = carousel.clientWidth;
    carousel.scrollTo({
      left: idx * slideWidth,
      behavior: 'smooth'
    });
  }

  // カレンダー移動時はトップが見えるよう保護
  const targetSlide = document.getElementById(`tab-content-${tab}`);
  if (tab === 'month' && targetSlide && targetSlide.scrollTop > 0) {
    targetSlide.scrollTo({ top: 0, behavior: 'auto' });
  }

  updateNavHighlight(tab);
}

export function updateNavHighlight(activeTab: TabName): void {
  tabCycle.forEach(t => {
    const nav = document.getElementById(`nav-${t}`);
    if (!nav) return;
    if (t === activeTab) {
      nav.className = 'relative z-10 flex-1 py-2.5 rounded-xl text-center font-bold text-stone-950 transition-colors flex items-center justify-center gap-1.5';
    } else {
      nav.className = 'relative z-10 flex-1 py-2.5 rounded-xl text-center font-bold text-stone-400 hover:text-stone-200 transition-colors flex items-center justify-center gap-1.5';
    }
  });
}

export function initCarouselSync(): void {
  const carousel = document.getElementById('views-carousel');
  const indicator = document.getElementById('nav-indicator');
  if (!carousel) return;

  carousel.addEventListener('scroll', () => {
    const slideWidth = carousel.clientWidth;
    if (!slideWidth) return;

    // 横スクロール進行度 (0.0: 直近ライブ, 1.0: 全日程, 2.0: カレンダー)
    const progress = carousel.scrollLeft / slideWidth;
    
    // 1. フッターのピルインジケーターをスワイプ量に完全同期させて移動
    if (indicator) {
      indicator.style.transform = `translateX(${progress * 100}%)`;
    }

    // 2. 最も近いタブの文字色ハイライトをリアルタイム更新
    const nearestIdx = Math.max(0, Math.min(tabCycle.length - 1, Math.round(progress)));
    const detectedTab = tabCycle[nearestIdx];
    if (currentTabName !== detectedTab) {
      currentTabName = detectedTab;
      updateNavHighlight(detectedTab);
    }
  }, { passive: true });
}
