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

interface MinimalTheme {
  dot: string;
  glow: string;
  accent: string;
}

const MINIMAL_THEMES: Record<string, MinimalTheme> = {
  ribonclock: {
    dot: 'bg-rose-500 shadow-[0_0_8px_rgba(244,63,94,0.6)]',
    glow: 'hover:border-rose-500/40',
    accent: 'text-rose-400',
  },
  dollykiss: {
    dot: 'bg-pink-500 shadow-[0_0_8px_rgba(236,72,153,0.6)]',
    glow: 'hover:border-pink-500/40',
    accent: 'text-pink-400',
  },
};

const DEFAULT_MINIMAL_THEME: MinimalTheme = {
  dot: 'bg-stone-500',
  glow: 'hover:border-stone-500/40',
  accent: 'text-stone-300',
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
        // Fallback
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
    <div class="h-[100dvh] w-full max-w-sm mx-auto flex flex-col justify-between overflow-hidden relative bg-black text-stone-100 select-none px-4 pt-4 pb-6">
      {/* Header */}
      <header class="flex items-baseline justify-between px-1 pb-4 shrink-0 border-b border-stone-900">
        <div>
          <span class="text-xs font-mono font-bold tracking-widest text-stone-400 uppercase">
            Directory
          </span>
          <h1 class="text-lg font-extrabold text-white tracking-tight mt-0.5">
            Groups
          </h1>
        </div>
        <div class="text-[11px] font-mono text-stone-500">
          {orderedGroups.length} REGISTERED
        </div>
      </header>

      {/* List Area */}
      <main class="flex-1 min-h-0 py-3 flex flex-col gap-2 overflow-y-auto no-scrollbar">
        {orderedGroups.map((group: GroupConfig) => {
          const isTop = topGroupId === group.id;
          const theme = MINIMAL_THEMES[group.id] || DEFAULT_MINIMAL_THEME;
          const nextEvent = NEXT_EVENT_DATA[group.id] || {
            date: 'TBD',
            title: '未定',
            venue: '未定',
          };
          const xLink = OFFICIAL_X_LINKS[group.id];

          return (
            <div
              key={group.id}
              onClick={() => navigateToGroup(group.id)}
              class={`group relative rounded-xl p-3.5 transition-all duration-200 cursor-pointer active:scale-[0.99] border border-stone-900 bg-stone-950/80 hover:bg-stone-900/60 ${theme.glow} flex flex-col gap-2.5`}
            >
              {/* Row 1: Indicator, Group Name & Top Pin */}
              <div class="flex items-center justify-between gap-2">
                <div class="flex items-center gap-2.5 min-w-0">
                  <span class={`w-2 h-2 rounded-full shrink-0 ${theme.dot}`} />
                  <div class="min-w-0">
                    <div class="flex items-center gap-1.5">
                      <span class="text-sm font-bold text-white tracking-tight truncate group-hover:text-stone-200 transition-colors">
                        {group.name}
                      </span>
                      {isTop && (
                        <span class="text-[9px] font-mono font-semibold px-1 py-0.2 rounded bg-amber-500/15 text-amber-300 border border-amber-500/30 flex items-center gap-0.5 shrink-0">
                          <Icon icon={Crown} size={9} class="text-amber-400" />
                          TOP
                        </span>
                      )}
                    </div>
                  </div>
                </div>

                {/* Right Actions */}
                <div class="flex items-center gap-1 shrink-0" onClick={(e) => e.stopPropagation()}>
                  {xLink && (
                    <a
                      href={xLink}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="p-1 rounded text-stone-600 hover:text-stone-300 transition-colors"
                      title="公式X"
                      aria-label="公式X"
                    >
                      <Icon icon={ExternalLink} size={13} />
                    </a>
                  )}
                  <button
                    onClick={(e) => handlePinTop(group.id, e)}
                    class={`p-1 rounded transition-colors ${
                      isTop
                        ? 'text-amber-400'
                        : 'text-stone-700 hover:text-stone-400'
                    }`}
                    title={isTop ? '最推し解除' : '最推しに設定'}
                    aria-label="推し設定"
                  >
                    <Icon icon={Crown} size={13} />
                  </button>
                  <Icon icon={ChevronRight} size={15} class="text-stone-600 group-hover:text-stone-300 group-hover:translate-x-0.5 transition-all ml-0.5" />
                </div>
              </div>

              {/* Row 2: Live Info Pill */}
              <div class="flex items-center justify-between text-[11px] text-stone-400 pl-4.5 pt-1 border-t border-stone-900/60 font-sans">
                <span class="truncate pr-2">
                  <span class="font-mono text-stone-300 mr-1.5">{nextEvent.date}</span>
                  <span class="text-stone-300">{nextEvent.title}</span>
                  <span class="text-stone-500 text-[10px] ml-1.5">@{nextEvent.venue}</span>
                </span>
                <span class="text-[10px] font-mono text-stone-500 shrink-0">
                  {group.shortName}
                </span>
              </div>
            </div>
          );
        })}
      </main>

      {/* Footer Info */}
      <footer class="pt-3 shrink-0 border-t border-stone-900 flex items-center justify-between text-[11px] text-stone-600 px-1 font-mono">
        <span>TAITE BOARD</span>
        <span>v1.4.3</span>
      </footer>
    </div>
  );
}
