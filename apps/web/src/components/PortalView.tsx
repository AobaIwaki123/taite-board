import { useState, useMemo } from 'preact/hooks';
import { GroupConfig, REGISTERED_GROUPS, DEFAULT_GROUP_ID } from '../config/group';
import { Icon } from './Icon';
import { Star, ChevronRight, ExternalLink } from '../utils/icons';

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
  const groups = useMemo(() => {
    return Object.values(REGISTERED_GROUPS).filter((g) => g.id !== 'sample');
  }, []);

  // お気に入りグループ ID（null は未設定）
  const [favoriteGroupId, setFavoriteGroupId] = useState<string | null>(() => {
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

  // 直前にタップしたボタングループ（マイクロアニメーション用）
  const [animatingGroupId, setAnimatingGroupId] = useState<string | null>(null);

  // お気に入りトグル処理（位置の入れ替えテレポートはせず、その場で点灯/消灯する直感的なUX）
  const handleToggleFavorite = (groupId: string, e: MouseEvent) => {
    e.stopPropagation();
    const isCurrentFav = favoriteGroupId === groupId;
    const newFav = isCurrentFav ? null : groupId;

    try {
      if (newFav) {
        localStorage.setItem('taite_top_group', newFav);
      } else {
        localStorage.removeItem('taite_top_group');
      }
    } catch {
      // Ignored
    }

    setFavoriteGroupId(newFav);
    setAnimatingGroupId(groupId);
    setTimeout(() => {
      setAnimatingGroupId(null);
    }, 400);
  };

  return (
    <div class="h-[100dvh] w-full max-w-sm mx-auto flex flex-col justify-between overflow-hidden relative bg-stone-950 text-stone-100 select-none px-3.5 pt-3 pb-6">
      {/* Header: Pure Minimal (ロゴのみで一切の説明テキストを排除) */}
      <header class="flex items-center justify-between px-1 pt-1 pb-2.5 shrink-0 border-b border-white/5">
        <div class="flex items-center gap-2">
          <span class="w-2 h-2 rounded-full bg-rose-500 animate-pulse" />
          <span class="text-xs font-bold tracking-widest text-stone-200 uppercase">Taite Board</span>
        </div>
      </header>

      {/* Ticket List Area */}
      <main class="flex-1 min-h-0 py-3 flex flex-col gap-3 overflow-y-auto no-scrollbar">
        {groups.map((group: GroupConfig, index: number) => {
          const isFav = favoriteGroupId === group.id;
          const isAnimating = animatingGroupId === group.id;
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
              onClick={() => navigateToGroup(group.id)}
              class={`group relative rounded-xl transition-all duration-300 ease-out cursor-pointer active:scale-[0.98] border ${
                isFav
                  ? 'border-amber-400/60 shadow-[0_0_18px_rgba(251,191,36,0.18)] bg-stone-900/95 ring-1 ring-amber-400/30'
                  : `${theme.border} bg-stone-900/90 shadow-md`
              } hover:shadow-lg flex items-stretch overflow-hidden`}
            >
              {/* Background Ambient Glow */}
              <div
                class={`absolute inset-0 bg-gradient-to-r ${
                  isFav
                    ? 'from-amber-500/10 via-amber-500/5 to-transparent opacity-80'
                    : `${theme.glow} opacity-40 group-hover:opacity-80`
                } pointer-events-none transition-opacity duration-300`}
              />

              {/* Left Ticket Stub (半券エリア) */}
              <div
                class={`w-14 shrink-0 ${theme.stubBg} border-r border-dashed border-stone-700/60 flex flex-col items-center justify-between py-2.5 px-1 relative z-10 transition-colors duration-300`}
              >
                {/* Ticket Number */}
                <span class="text-[9px] font-mono tracking-wider text-stone-500 uppercase">
                  #{String(index + 1).padStart(2, '0')}
                </span>

                {/* Group Short Name */}
                <div class="text-[11px] font-bold tracking-wider text-stone-300 group-hover:text-white transition-colors text-center leading-tight">
                  {group.shortName}
                </div>

                {/* Status Indicator (お気に入りはゴールド、通常はテーマカラー) */}
                <span
                  class={`w-1.5 h-1.5 rounded-full transition-all duration-300 ${
                    isFav
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
                      {isFav && (
                        <span class="text-[9px] font-bold px-1.5 py-0.2 rounded bg-amber-500/20 text-amber-300 border border-amber-500/40 shrink-0 flex items-center gap-0.5">
                          推し
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Right Actions: Official X & Star Toggle */}
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
                      onClick={(e) => handleToggleFavorite(group.id, e)}
                      class={`p-1.5 rounded-lg transition-all duration-200 ${
                        isAnimating ? 'scale-125' : 'scale-100 active:scale-90'
                      } ${
                        isFav
                          ? 'text-amber-400 bg-amber-400/10'
                          : 'text-stone-600 hover:text-stone-400 hover:bg-white/5'
                      }`}
                      title={isFav ? 'お気に入りを解除' : 'お気に入りに登録'}
                      aria-label="お気に入り登録"
                    >
                      <Icon
                        icon={Star}
                        size={15}
                        class={`transition-all duration-200 ${
                          isFav
                            ? 'fill-amber-400 text-amber-400 filter drop-shadow-[0_0_6px_rgba(251,191,36,0.6)]'
                            : 'stroke-[1.5]'
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

                  <div class="flex items-center text-stone-500 group-hover:text-stone-300 group-hover:translate-x-0.5 transition-all shrink-0">
                    <Icon icon={ChevronRight} size={15} />
                  </div>
                </div>
              </div>
            </div>
          );
        })}
      </main>

      {/* Footer Info */}
      <footer class="pt-2 shrink-0 border-t border-white/5 flex items-center justify-between text-[11px] text-stone-500 px-1 font-mono">
        <span>タップしてタイテを開く</span>
        <span>v1.4.3</span>
      </footer>
    </div>
  );
}
