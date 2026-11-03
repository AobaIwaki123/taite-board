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

interface GroupTicketTheme {
  primary: string;
  glow: string;
  border: string;
  stubBg: string;
  badge: string;
  code: string;
}

const TICKET_THEMES: Record<string, GroupTicketTheme> = {
  ribonclock: {
    primary: 'text-rose-400',
    glow: 'from-rose-500/15 via-rose-500/5 to-transparent',
    border: 'border-rose-500/30 hover:border-rose-500/60',
    stubBg: 'bg-rose-950/40',
    badge: 'bg-rose-500/20 text-rose-300 border-rose-500/40',
    code: 'RBCK',
  },
  dollykiss: {
    primary: 'text-pink-400',
    glow: 'from-pink-500/15 via-purple-500/5 to-transparent',
    border: 'border-pink-500/30 hover:border-pink-500/60',
    stubBg: 'bg-pink-950/40',
    badge: 'bg-pink-500/20 text-pink-300 border-pink-500/40',
    code: 'DLKS',
  },
};

const DEFAULT_TICKET_THEME: GroupTicketTheme = {
  primary: 'text-stone-300',
  glow: 'from-stone-700/15 to-transparent',
  border: 'border-stone-700/50 hover:border-stone-600',
  stubBg: 'bg-stone-900/60',
  badge: 'bg-stone-800 text-stone-300 border-stone-700',
  code: 'LIVE',
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
    <div class="h-[100dvh] w-full max-w-sm mx-auto flex flex-col justify-between overflow-hidden relative bg-stone-950 text-stone-100 select-none px-3.5 pt-3 pb-6">
      {/* Minimal Header */}
      <header class="flex items-center justify-between px-1 pt-1 pb-3 shrink-0 border-b border-white/5">
        <div class="flex items-center gap-2">
          <span class="w-2 h-2 rounded-full bg-rose-500 animate-pulse" />
          <span class="text-xs font-bold tracking-widest text-stone-200 uppercase">Taite Board</span>
          <span class="text-[9px] font-mono px-1.5 py-0.5 rounded bg-white/5 text-stone-400 border border-white/10 uppercase">
            Pass
          </span>
        </div>
        <div class="text-[11px] text-stone-400 font-medium">
          エントランス
        </div>
      </header>

      {/* Ticket List Area */}
      <main class="flex-1 min-h-0 py-3 flex flex-col gap-3 overflow-y-auto no-scrollbar">
        {orderedGroups.map((group: GroupConfig, index: number) => {
          const isTop = topGroupId === group.id;
          const theme = TICKET_THEMES[group.id] || DEFAULT_TICKET_THEME;
          const nextEvent = NEXT_EVENT_DATA[group.id] || {
            date: 'TBD',
            title: 'スケジュール準備中',
            venue: '未定',
          };
          const xLink = OFFICIAL_X_LINKS[group.id];

          return (
            <div
              key={group.id}
              onClick={() => navigateToGroup(group.id)}
              class={`group relative rounded-xl transition-all duration-200 ease-out cursor-pointer active:scale-[0.98] border ${theme.border} bg-stone-900/90 shadow-md hover:shadow-lg flex items-stretch overflow-hidden`}
            >
              {/* Left Ambient Glow */}
              <div
                class={`absolute inset-0 bg-gradient-to-r ${theme.glow} pointer-events-none opacity-40 group-hover:opacity-80 transition-opacity duration-300`}
              />

              {/* Left Ticket Stub */}
              <div
                class={`w-14 shrink-0 ${theme.stubBg} border-r border-dashed border-stone-700/60 flex flex-col items-center justify-between py-3 relative z-10`}
              >
                <div class="text-[9px] font-mono tracking-widest text-stone-500 uppercase">
                  #{String(index + 1).padStart(2, '0')}
                </div>
                <div class="writing-vertical-rl font-mono text-xs font-extrabold tracking-widest uppercase text-stone-300 group-hover:text-white transition-colors">
                  {theme.code}
                </div>
                <div class="w-1.5 h-1.5 rounded-full bg-stone-700 group-hover:bg-rose-400 transition-colors" />

                {/* Ticket Notches (Top & Bottom Indent Metaphor) */}
                <div class="absolute -top-1.5 -right-1.5 w-3 h-3 rounded-full bg-stone-950 border-b border-stone-800" />
                <div class="absolute -bottom-1.5 -right-1.5 w-3 h-3 rounded-full bg-stone-950 border-t border-stone-800" />
              </div>

              {/* Main Ticket Body */}
              <div class="flex-1 min-w-0 p-3.5 flex flex-col justify-between relative z-10">
                {/* Upper Line: Group Name & Actions */}
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <div class="flex items-center gap-1.5">
                      <h2 class="text-sm font-bold text-white tracking-wide truncate group-hover:text-rose-200 transition-colors">
                        {group.name}
                      </h2>
                      {isTop && (
                        <span class="text-[9px] font-bold px-1 py-0.2 rounded bg-amber-500/20 text-amber-300 border border-amber-500/30 flex items-center gap-0.5 shrink-0">
                          <Icon icon={Crown} size={9} class="text-amber-400" />
                          推し
                        </span>
                      )}
                    </div>
                    <div class="text-[10px] font-mono text-stone-400 tracking-wider">
                      {group.shortName}
                    </div>
                  </div>

                  {/* Actions */}
                  <div class="flex items-center gap-0.5 shrink-0" onClick={(e) => e.stopPropagation()}>
                    {xLink && (
                      <a
                        href={xLink}
                        target="_blank"
                        rel="noopener noreferrer"
                        class="p-1 rounded text-stone-500 hover:text-white hover:bg-white/10 transition-colors"
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
                          ? 'text-amber-400 bg-amber-500/10'
                          : 'text-stone-600 hover:text-stone-300 hover:bg-white/10'
                      }`}
                      title={isTop ? '最推し解除' : '最推しに設定'}
                      aria-label="推し設定"
                    >
                      <Icon icon={Crown} size={13} />
                    </button>
                  </div>
                </div>

                {/* Lower Line: Next Live Pill & Entry CTA */}
                <div class="pt-2 mt-1.5 border-t border-white/5 flex items-center justify-between gap-2">
                  <div class="flex items-center gap-1.5 min-w-0">
                    <span class={`text-[9px] font-mono px-1.5 py-0.5 rounded border ${theme.badge} shrink-0`}>
                      {nextEvent.date}
                    </span>
                    <span class="text-[11px] text-stone-300 truncate">
                      {nextEvent.title}
                    </span>
                  </div>

                  <div class="flex items-center gap-0.5 text-[10px] font-mono font-bold text-stone-400 group-hover:text-white transition-colors shrink-0">
                    <span>ENTER</span>
                    <Icon icon={ChevronRight} size={13} class="group-hover:translate-x-0.5 transition-transform" />
                  </div>
                </div>
              </div>
            </div>
          );
        })}
      </main>

      {/* Footer Info */}
      <footer class="pt-2 shrink-0 border-t border-white/5 flex items-center justify-between text-[11px] text-stone-500 px-1 font-mono">
        <span>SELECT PASS TO ENTER</span>
        <span>v1.4.3</span>
      </footer>
    </div>
  );
}
