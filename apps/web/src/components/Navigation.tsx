import { Icon } from './Icon';
import { Sparkles, Calendar, List } from '../utils/icons';

export type TabName = 'next' | 'week' | 'month';

interface NavigationProps {
  activeTab: TabName;
  scrollProgress?: number;
  isProgrammatic?: boolean;
  onSelectTab: (tab: TabName) => void;
  isViewingSelectedLive: boolean;
}

export function Navigation({
  activeTab,
  scrollProgress,
  isProgrammatic,
  onSelectTab,
  isViewingSelectedLive,
}: NavigationProps) {
  const tabs: TabName[] = ['next', 'week', 'month'];
  const activeIndex = tabs.indexOf(activeTab);
  const currentProgress = typeof scrollProgress === 'number' ? scrollProgress : activeIndex;

  // 各タブとの近接度に応じたアクティブ判定
  const isTabActive = (index: number) => Math.abs(currentProgress - index) < 0.5;

  return (
    <nav class="flex-shrink-0 pt-2 pb-2.5 border-t border-white/5 bg-stone-950/80 backdrop-blur-md">
      <div class="relative flex p-1 rounded-2xl bg-stone-900 border border-white/10 select-none">
        {/* スライディング・ピルインジケーター（スワイプにリアルタイム追従、クリック時は滑らかに遷移） */}
        <div
          class="absolute top-1 bottom-1 w-[calc(33.333%-2px)] rounded-xl bg-gradient-to-r from-rose-500 to-rose-400 shadow-md pointer-events-none"
          style={{
            transform: `translateX(${currentProgress * 100}%)`,
            transition: isProgrammatic
              ? 'transform 300ms cubic-bezier(0.2, 0.8, 0.2, 1)'
              : 'none',
          }}
        />

        {/* タブ 1: 直近ライブ / 公演詳細 */}
        <button
          onClick={() => onSelectTab('next')}
          class={`relative z-10 flex-1 py-2.5 rounded-xl text-center font-bold text-xs transition-colors duration-150 flex items-center justify-center gap-1.5 focus:outline-none ${
            isTabActive(0) ? 'text-stone-950' : 'text-stone-400 hover:text-stone-200'
          }`}
        >
          {isViewingSelectedLive ? (
            <>
              <Icon icon={Calendar} class="w-3.5 h-3.5" />
              <span>公演詳細</span>
            </>
          ) : (
            <>
              <Icon icon={Sparkles} class="w-3.5 h-3.5" />
              <span>直近ライブ</span>
            </>
          )}
        </button>

        {/* タブ 2: 全日程 */}
        <button
          onClick={() => onSelectTab('week')}
          class={`relative z-10 flex-1 py-2.5 rounded-xl text-center font-bold text-xs transition-colors duration-150 flex items-center justify-center gap-1.5 focus:outline-none ${
            isTabActive(1) ? 'text-stone-950' : 'text-stone-400 hover:text-stone-200'
          }`}
        >
          <Icon icon={List} class="w-3.5 h-3.5" />
          <span>全日程</span>
        </button>

        {/* タブ 3: カレンダー */}
        <button
          onClick={() => onSelectTab('month')}
          class={`relative z-10 flex-1 py-2.5 rounded-xl text-center font-bold text-xs transition-colors duration-150 flex items-center justify-center gap-1.5 focus:outline-none ${
            isTabActive(2) ? 'text-stone-950' : 'text-stone-400 hover:text-stone-200'
          }`}
        >
          <Icon icon={Calendar} class="w-3.5 h-3.5" />
          <span>カレンダー</span>
        </button>
      </div>
    </nav>
  );
}
