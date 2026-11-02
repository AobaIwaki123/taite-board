import { useState, useMemo } from 'preact/hooks';
import { GroupConfig, REGISTERED_GROUPS, DEFAULT_GROUP_ID } from '../config/group';
import { Icon } from './Icon';
import { Crown, ChevronRight, ExternalLink } from '../utils/icons';

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

// グループごとのテーマグラデーション設定
const GROUP_THEME_GRADIENTS: Record<string, { border: string; glow: string; badge: string; bar: string }> = {
  ribonclock: {
    border: 'border-rose-500/30 hover:border-rose-500/60',
    glow: 'from-rose-500/10 via-rose-500/5 to-transparent',
    badge: 'bg-rose-500/15 text-rose-300 border-rose-500/30',
    bar: 'from-rose-500 to-rose-400',
  },
  dollykiss: {
    border: 'border-pink-500/30 hover:border-pink-500/60',
    glow: 'from-pink-500/10 via-purple-500/5 to-transparent',
    badge: 'bg-pink-500/15 text-pink-300 border-pink-500/30',
    bar: 'from-pink-500 to-purple-500',
  },
};

const DEFAULT_THEME_GRADIENT = {
  border: 'border-stone-700/50 hover:border-stone-600',
  glow: 'from-stone-700/10 to-transparent',
  badge: 'bg-stone-800 text-stone-300 border-stone-700',
  bar: 'from-stone-500 to-stone-400',
};

// 次回ライブ情報（画像不要・テキストサマリー）
const NEXT_EVENT_SUMMARY: Record<string, string> = {
  ribonclock: '次回: 11/3(祝) アイドルマリアージュ @ ベルサール虎ノ門',
  dollykiss: '次回: 11/3(祝) アイドルマリアージュ / @JAM×アイドル横丁',
};

// 公式Xリンク
const OFFICIAL_X_LINKS: Record<string, string> = {
  ribonclock: 'https://x.com/Ribbon_Clock',
  dollykiss: 'https://x.com/DollyKiss_staff',
};

export function PortalView() {
  const realGroups = useMemo(() => {
    return Object.values(REGISTERED_GROUPS).filter((g) => g.id !== 'sample');
  }, []);

  const [topGroupId, setTopGroupId] = useState<string>(() => {
    if (typeof window !== 'undefined') {
      try {
        const stored = localStorage.getItem('taite_top_group');
        if (stored && REGISTERED_GROUPS[stored] && stored !== 'sample') {
          return stored;
        }
      } catch {
        // Fallback if localStorage access fails
      }
    }
    return DEFAULT_GROUP_ID;
  });

  const orderedGroups = useMemo(() => {
    const top = realGroups.find((g) => g.id === topGroupId) || realGroups[0];
    const others = realGroups.filter((g) => g.id !== top.id);
    return [top, ...others];
  }, [realGroups, topGroupId]);

  const handlePinTop = (groupId: string, e: MouseEvent) => {
    e.stopPropagation();
    try {
      localStorage.setItem('taite_top_group', groupId);
    } catch {
      // Ignored
    }
    setTopGroupId(groupId);
  };

  return (
    <div class="h-[100dvh] w-full max-w-sm mx-auto flex flex-col justify-between overflow-hidden relative bg-stone-950 text-stone-100 select-none px-3.5 pt-3 pb-6">
      {/* Top minimal header */}
      <header class="flex items-center justify-between px-1 pt-1 pb-3 shrink-0 border-b border-white/5">
        <div class="flex items-center gap-2">
          <span class="w-2 h-2 rounded-full bg-rose-500 animate-pulse" />
          <span class="text-xs font-bold tracking-widest text-stone-200 uppercase">Taite Board</span>
        </div>
        <div class="text-[11px] text-stone-400 font-medium">
          アイドルグループ選択
        </div>
      </header>

      {/* Ticket List Area */}
      <main class="flex-1 min-h-0 py-4 flex flex-col gap-3.5 overflow-y-auto no-scrollbar">
        {orderedGroups.map((group: GroupConfig) => {
          const isTop = topGroupId === group.id;
          const themeStyle = GROUP_THEME_GRADIENTS[group.id] || DEFAULT_THEME_GRADIENT;
          const nextEvent = NEXT_EVENT_SUMMARY[group.id] || 'スケジュール登録中';
          const xLink = OFFICIAL_X_LINKS[group.id];

          return (
            <div
              key={group.id}
              onClick={() => navigateToGroup(group.id)}
              class={`group relative rounded-2xl p-4 transition-all duration-200 ease-out cursor-pointer active:scale-[0.98] border ${themeStyle.border} bg-stone-900/80 shadow-lg hover:shadow-xl backdrop-blur-sm overflow-hidden flex flex-col justify-between min-h-[110px]`}
            >
              {/* Left Accent Bar */}
              <div
                class={`absolute left-0 top-0 bottom-0 w-1.5 bg-gradient-to-b ${themeStyle.bar}`}
              />

              {/* Background Ambient Glow */}
              <div
                class={`absolute inset-0 bg-gradient-to-br ${themeStyle.glow} pointer-events-none opacity-40 group-hover:opacity-70 transition-opacity duration-300`}
              />

              {/* Upper: Group Info & Top Pin / Official X */}
              <div class="relative z-10 flex items-start justify-between gap-2 pl-1">
                <div class="flex items-center gap-2.5">
                  <span class="text-2xl shrink-0 filter drop-shadow">
                    {group.logoText || '🎀'}
                  </span>
                  <div>
                    <div class="flex items-center gap-2">
                      <h2 class="text-base font-extrabold text-white tracking-wide group-hover:text-rose-200 transition-colors">
                        {group.name}
                      </h2>
                      {isTop && (
                        <span class="text-[10px] font-bold px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-300 border border-amber-500/30 flex items-center gap-1">
                          <Icon icon={Crown} size={10} class="text-amber-400" />
                          推し
                        </span>
                      )}
                    </div>
                    <div class="text-[11px] text-stone-400 font-medium">
                      {group.shortName}
                    </div>
                  </div>
                </div>

                {/* Actions: Pin & Official X */}
                <div class="flex items-center gap-1 shrink-0" onClick={(e) => e.stopPropagation()}>
                  {xLink && (
                    <a
                      href={xLink}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="p-1.5 rounded-lg text-stone-400 hover:text-white hover:bg-white/10 transition-colors"
                      title="公式Xを開く"
                      aria-label="公式X"
                    >
                      <Icon icon={ExternalLink} size={14} />
                    </a>
                  )}
                  <button
                    onClick={(e) => handlePinTop(group.id, e)}
                    class={`p-1.5 rounded-lg transition-colors ${
                      isTop
                        ? 'text-amber-400 bg-amber-500/10'
                        : 'text-stone-500 hover:text-stone-300 hover:bg-white/10'
                    }`}
                    title={isTop ? '最推しグループに設定中' : '最推しに設定する'}
                    aria-label="推し設定"
                  >
                    <Icon icon={Crown} size={14} />
                  </button>
                </div>
              </div>

              {/* Lower: Next Live Summary & Arrow */}
              <div class="relative z-10 pt-2.5 mt-2 border-t border-white/5 flex items-center justify-between pl-1">
                <div class="text-[11px] text-stone-300 font-medium truncate pr-2">
                  {nextEvent}
                </div>
                <div class="flex items-center text-xs font-bold text-stone-400 group-hover:text-white group-hover:translate-x-0.5 transition-all shrink-0">
                  <Icon icon={ChevronRight} size={16} />
                </div>
              </div>
            </div>
          );
        })}
      </main>

      {/* Footer minimal info */}
      <footer class="pt-2 shrink-0 border-t border-white/5 flex items-center justify-between text-[11px] text-stone-500 px-1">
        <span>タップしてタイテを開く</span>
        <span>taite-board v1.4.3</span>
      </footer>
    </div>
  );
}
