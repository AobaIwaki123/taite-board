import { useState, useMemo, useRef, useLayoutEffect } from 'preact/hooks';
import { GroupConfig, REGISTERED_GROUPS, DEFAULT_GROUP_ID } from '../config/group';
import { Icon } from './Icon';
import { Pin, ChevronRight, ExternalLink } from '../utils/icons';

export function navigateToGroup(groupId: string) {
  if (typeof window === 'undefined') return;
  const hostname = window.location.hostname.toLowerCase();
  const isProd = hostname.endsWith('iorin.net') && !hostname.includes('localhost');
  if (isProd) {
    window.location.href = `https://${groupId}.iorin.net`;
  } else {
    const url = new URL(window.location.href);
    url.searchParams.set('group', groupId);
    url.searchParams.delete('view');
    window.location.href = url.toString();
  }
}

interface GroupTicketTheme {
  primary: string;
  glow: string;
  border: string;
  stubBg: string;
  badge: string;
  indicator: string;
}

const TICKET_THEMES: Record<string, GroupTicketTheme> = {
  ribonclock: {
    primary: 'text-rose-400',
    glow: 'from-rose-500/15 via-rose-500/5 to-transparent',
    border: 'border-rose-500/30 hover:border-rose-500/60',
    stubBg: 'bg-rose-950/40',
    badge: 'bg-rose-500/20 text-rose-300 border-rose-500/40',
    indicator: 'bg-rose-500',
  },
  dollykiss: {
    primary: 'text-pink-400',
    glow: 'from-pink-500/15 via-purple-500/5 to-transparent',
    border: 'border-pink-500/30 hover:border-pink-500/60',
    stubBg: 'bg-pink-950/40',
    badge: 'bg-pink-500/20 text-pink-300 border-pink-500/40',
    indicator: 'bg-pink-500',
  },
};

const DEFAULT_TICKET_THEME: GroupTicketTheme = {
  primary: 'text-stone-300',
  glow: 'from-stone-700/15 to-transparent',
  border: 'border-stone-700/50 hover:border-stone-600',
  stubBg: 'bg-stone-900/60',
  badge: 'bg-stone-800 text-stone-300 border-stone-700',
  indicator: 'bg-stone-500',
};

const NEXT_EVENT_DATA: Record<string, { date: string; title: string; venue: string }> = {
  ribonclock: {
    date: '11/3 (祝)',
    title: 'アイドルマリアージュ',
    venue: 'ベルサール虎ノ門',
  },
  dollykiss: {
    date: '11/3 (祝)',
    title: 'アイドルマリアージュ / 横丁',
    venue: '都内複数会場',
  },
};

const OFFICIAL_X_LINKS: Record<string, string> = {
  ribonclock: 'https://x.com/Ribbon_Clock',
  dollykiss: 'https://x.com/DollyKiss_staff',
};

export function PortalView() {
  const baseGroups = useMemo(() => {
    return Object.values(REGISTERED_GROUPS).filter((g) => g.id !== 'sample');
  }, []);

  // 各グループの固有固定ID（並び順が変わっても不変のチケットシリアル番号）
  const groupFixedIdMap = useMemo(() => {
    const map: Record<string, string> = {};
    baseGroups.forEach((g, idx) => {
      map[g.id] = `#${String(idx + 1).padStart(2, '0')}`;
    });
    return map;
  }, [baseGroups]);

  // ピン留めされたグループ ID（null はデフォルト状態）
  const [pinnedGroupId, setPinnedGroupId] = useState<string | null>(() => {
    if (typeof window !== 'undefined') {
      try {
        const stored = localStorage.getItem('taite_top_group');
        if (stored && REGISTERED_GROUPS[stored] && stored !== 'sample') {
          return stored;
        }
      } catch {
        // Fallback
      }
    }
    return DEFAULT_GROUP_ID;
  });

  // 物理アニメーション用の状態
  const [elevatingGroupId, setElevatingGroupId] = useState<string | null>(null);
  const cardElementsRef = useRef<Record<string, HTMLDivElement | null>>({});
  const prevPositionsRef = useRef<Record<string, number>>({});

  // ピン留めグループを最上位にした順序
  const orderedGroups = useMemo(() => {
    if (!pinnedGroupId) return baseGroups;
    const pinned = baseGroups.find((g) => g.id === pinnedGroupId);
    if (!pinned) return baseGroups;
    const others = baseGroups.filter((g) => g.id !== pinned.id);
    return [pinned, ...others];
  }, [baseGroups, pinnedGroupId]);

  // FLIP 物理スライドアニメーションの実行
  useLayoutEffect(() => {
    const prevPositions = prevPositionsRef.current;
    if (Object.keys(prevPositions).length === 0) return;

    for (const group of orderedGroups) {
      const el = cardElementsRef.current[group.id];
      const prevTop = prevPositions[group.id];
      if (el && prevTop !== undefined) {
        const currentTop = el.getBoundingClientRect().top;
        const deltaY = prevTop - currentTop;

        if (deltaY !== 0) {
          // 1. Invert: 移動前の座標へ瞬時に引き戻す
          el.style.transform = `translateY(${deltaY}px)`;
          el.style.transition = 'none';

          // 2. Play: 次フレームで滑らかにスライド移動
          requestAnimationFrame(() => {
            requestAnimationFrame(() => {
              el.style.transition = 'transform 380ms cubic-bezier(0.16, 1, 0.3, 1)';
              el.style.transform = '';
            });
          });
        }
      }
    }

    // 浮遊状態の着地タイマー
    const timer = setTimeout(() => {
      setElevatingGroupId(null);
    }, 400);

    return () => clearTimeout(timer);
  }, [orderedGroups]);

  // ピン留めトグル（ふわりと浮いてスライド移動）
  const handleTogglePin = (groupId: string, e: MouseEvent) => {
    e.stopPropagation();

    // 1. First: アニメーション前の全カードの Y 座標を記録
    const newPositions: Record<string, number> = {};
    for (const group of orderedGroups) {
      const el = cardElementsRef.current[group.id];
      if (el) {
        newPositions[group.id] = el.getBoundingClientRect().top;
      }
    }
    prevPositionsRef.current = newPositions;

    // タップされたカードを浮遊（elevation）状態にする
    setElevatingGroupId(groupId);

    const isCurrentPinned = pinnedGroupId === groupId;
    const newPinned = isCurrentPinned ? null : groupId;

    try {
      if (newPinned) {
        localStorage.setItem('taite_top_group', newPinned);
      } else {
        localStorage.removeItem('taite_top_group');
      }
    } catch {
      // Ignored
    }

    setPinnedGroupId(newPinned);
  };

  return (
    <div class="h-[100dvh] w-full max-w-sm mx-auto flex flex-col justify-between overflow-hidden relative bg-stone-950 text-stone-100 select-none px-3.5 pt-3 pb-6">
      {/* Header: Pure Minimal */}
      <header class="flex items-center justify-between px-1 pt-1 pb-2.5 shrink-0 border-b border-white/5">
        <div class="flex items-center gap-2">
          <span class="w-2 h-2 rounded-full bg-rose-500 animate-pulse" />
          <span class="text-xs font-bold tracking-widest text-stone-200 uppercase">Taite Board</span>
        </div>
      </header>

      {/* Ticket List Area */}
      <main class="flex-1 min-h-0 py-3 flex flex-col gap-3 overflow-y-auto no-scrollbar relative">
        {orderedGroups.map((group: GroupConfig) => {
          const isPinned = pinnedGroupId === group.id;
          const isElevating = elevatingGroupId === group.id;
          const theme = TICKET_THEMES[group.id] || DEFAULT_TICKET_THEME;
          const nextEvent = NEXT_EVENT_DATA[group.id] || {
            date: '未定',
            title: 'スケジュール準備中',
            venue: '未定',
          };
          const xLink = OFFICIAL_X_LINKS[group.id];

          return (
            <div
              key={group.id}
              ref={(el) => {
                cardElementsRef.current[group.id] = el;
              }}
              onClick={() => navigateToGroup(group.id)}
              class={`group relative rounded-xl transition-[border-color,box-shadow,opacity] duration-300 ease-out cursor-pointer active:scale-[0.98] border ${
                isPinned
                  ? 'border-amber-400/60 bg-stone-900/95 ring-1 ring-amber-400/30'
                  : `${theme.border} bg-stone-900/90 shadow-md`
              } ${
                isElevating
                  ? 'z-30 scale-[1.02] shadow-[0_20px_40px_rgba(0,0,0,0.9),0_0_24px_rgba(251,191,36,0.3)] transition-all duration-300'
                  : isPinned
                  ? 'shadow-[0_0_18px_rgba(251,191,36,0.18)] z-10'
                  : 'z-0'
              } flex items-stretch overflow-hidden`}
            >
              {/* Background Ambient Glow */}
              <div
                class={`absolute inset-0 bg-gradient-to-r ${
                  isPinned
                    ? 'from-amber-500/10 via-amber-500/5 to-transparent opacity-80'
                    : `${theme.glow} opacity-40 group-hover:opacity-80`
                } pointer-events-none transition-opacity duration-300`}
              />

              {/* Left Ticket Stub (半券エリア) */}
              <div
                class={`w-14 shrink-0 ${theme.stubBg} border-r border-dashed border-stone-700/60 flex flex-col items-center justify-between py-2.5 px-1 relative z-10 transition-colors duration-300`}
              >
                {/* Ticket Serial ID (グループ固有の不変シリアル番号) */}
                <span class="text-[9px] font-mono tracking-wider text-stone-500 uppercase">
                  {groupFixedIdMap[group.id] || '#00'}
                </span>

                {/* Group Short Name */}
                <div class="text-[11px] font-bold tracking-wider text-stone-300 group-hover:text-white transition-colors text-center leading-tight">
                  {group.shortName}
                </div>

                {/* Status Indicator */}
                <span
                  class={`w-1.5 h-1.5 rounded-full transition-all duration-300 ${
                    isPinned
                      ? 'bg-amber-400 shadow-[0_0_6px_rgba(251,191,36,0.8)]'
                      : theme.indicator
                  }`}
                />

                {/* Cutout Notches */}
                <div class="absolute -top-1.5 -right-1.5 w-3 h-3 rounded-full bg-stone-950 border-b border-stone-800" />
                <div class="absolute -bottom-1.5 -right-1.5 w-3 h-3 rounded-full bg-stone-950 border-t border-stone-800" />
              </div>

              {/* Main Ticket Body (本券エリア) */}
              <div class="flex-1 min-w-0 p-3 flex flex-col justify-between relative z-10">
                {/* Upper: Group Name & Actions */}
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <div class="flex items-center gap-2">
                      <h2 class="text-sm font-bold text-white tracking-wide truncate group-hover:text-rose-200 transition-colors">
                        {group.name}
                      </h2>
                      {isPinned && (
                        <span class="text-[9px] font-bold px-1.5 py-0.2 rounded bg-amber-500/20 text-amber-300 border border-amber-500/40 shrink-0 flex items-center gap-0.5">
                          推し
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Right Actions: Official X & Pin Toggle */}
                  <div class="flex items-center gap-1.5 shrink-0" onClick={(e) => e.stopPropagation()}>
                    {xLink && (
                      <a
                        href={xLink}
                        target="_blank"
                        rel="noopener noreferrer"
                        class="p-1 rounded text-stone-500 hover:text-white hover:bg-white/10 transition-colors"
                        title="公式Xを開く"
                        aria-label="公式X"
                      >
                        <Icon icon={ExternalLink} size={13} />
                      </a>
                    )}
                    <button
                      onClick={(e) => handleTogglePin(group.id, e)}
                      class={`p-1.5 rounded-lg transition-all duration-200 ${
                        isPinned
                          ? 'text-amber-400 bg-amber-400/10'
                          : 'text-stone-600 hover:text-stone-300 hover:bg-white/5'
                      } active:scale-90`}
                      title={isPinned ? 'ピン留めを解除' : '最上段にピン留め'}
                      aria-label="ピン留め設定"
                    >
                      <Icon
                        icon={Pin}
                        size={14}
                        class={`transition-all duration-300 ${
                          isPinned
                            ? 'fill-amber-400 rotate-[-45deg] filter drop-shadow-[0_0_6px_rgba(251,191,36,0.6)]'
                            : 'rotate-0'
                        }`}
                      />
                    </button>
                  </div>
                </div>

                {/* Lower: Next Event & Arrow */}
                <div class="pt-2 mt-1 border-t border-white/5 flex items-center justify-between gap-2">
                  <div class="flex items-center gap-1.5 min-w-0">
                    <span class={`text-[9px] font-mono px-1.5 py-0.5 rounded border ${theme.badge} shrink-0`}>
                      {nextEvent.date}
                    </span>
                    <span class="text-[11px] text-stone-300 truncate">
                      {nextEvent.title}
                    </span>
                    <span class="text-[10px] text-stone-500 truncate hidden sm:inline">
                      @{nextEvent.venue}
                    </span>
                  </div>

                  <div class="flex items-center gap-1.5 shrink-0">
                    <div class="flex items-center text-stone-500 group-hover:text-stone-300 group-hover:translate-x-0.5 transition-all">
                      <Icon icon={ChevronRight} size={15} />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          );
        })}
      </main>

      {/* Footer Info */}
      <footer class="pt-2 shrink-0 border-t border-white/5 flex items-center justify-between text-[11px] text-stone-500 px-1 font-mono">
        <span>推しグループを選択</span>
        <span>v1.4.3</span>
      </footer>
    </div>
  );
}
