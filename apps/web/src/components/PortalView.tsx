import { useState, useMemo } from 'preact/hooks';
import { GroupConfig, REGISTERED_GROUPS, DEFAULT_GROUP_ID } from '../config/group';
import { Icon } from './Icon';
import { Crown } from '../utils/icons';

const PEEK_OFFSET_PX = 84;

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

  const [activeGroupId, setActiveGroupId] = useState<string>(topGroupId);

  const orderedGroups = useMemo(() => {
    const top = realGroups.find((g) => g.id === topGroupId) || realGroups[0];
    const others = realGroups.filter((g) => g.id !== top.id);
    return [top, ...others];
  }, [realGroups, topGroupId]);

  const handleCardClick = (groupId: string, e: MouseEvent) => {
    e.stopPropagation();
    if (activeGroupId === groupId) {
      navigateToGroup(groupId);
    } else {
      setActiveGroupId(groupId);
    }
  };

  const handlePinTop = (groupId: string, e: MouseEvent) => {
    e.stopPropagation();
    try {
      localStorage.setItem('taite_top_group', groupId);
    } catch {
      // Ignored
    }
    setTopGroupId(groupId);
    setActiveGroupId(groupId);
  };

  const handleBackgroundClick = () => {
    if (activeGroupId !== topGroupId) {
      setActiveGroupId(topGroupId);
    }
  };

  const cardHeightStyle = `calc(100% - ${(orderedGroups.length - 1) * PEEK_OFFSET_PX}px)`;

  return (
    <div
      onClick={handleBackgroundClick}
      class="h-[100dvh] w-full max-w-sm mx-auto flex flex-col justify-between overflow-hidden relative bg-stone-950 text-stone-100 select-none px-3.5 pt-3 pb-4"
    >
      {/* Top minimal status bar */}
      <header class="flex items-center justify-between px-1 pt-0.5 pb-2.5 shrink-0">
        <div class="flex items-center gap-2">
          <span class="w-2 h-2 rounded-full bg-rose-500 animate-pulse" />
          <span class="text-xs font-bold tracking-widest text-stone-300 uppercase">Taite Board</span>
        </div>
        <div class="text-[11px] text-stone-500 font-medium">
          {orderedGroups.length} Groups
        </div>
      </header>

      {/* Wallet Card Stack Area */}
      <main class="flex-1 min-h-0 relative w-full">
        {orderedGroups.map((group: GroupConfig, index: number) => {
          const isActive = activeGroupId === group.id;
          const isTop = topGroupId === group.id;
          const defaultTranslateY = index * PEEK_OFFSET_PX;
          const translateY = isActive ? 0 : defaultTranslateY;
          const zIndex = isActive ? 30 : 10 - index;
          const displayName = group.id === 'rincent' ? 'RiNCENT#' : group.name;

          return (
            <div
              key={group.id}
              onClick={(e) => handleCardClick(group.id, e)}
              style={{
                height: cardHeightStyle,
                transform: `translate3d(0, ${translateY}px, 0) scale(${isActive ? 1.02 : 1})`,
                zIndex,
              }}
              class={`absolute inset-x-0 top-0 rounded-3xl overflow-hidden cursor-pointer transition-all duration-[220ms] ease-[cubic-bezier(0.16,1,0.3,1)] border border-white/10 ${
                isActive
                  ? 'shadow-[0_25px_50px_-12px_rgba(0,0,0,0.85)] active:scale-[0.99]'
                  : 'shadow-xl hover:brightness-105'
              }`}
            >
              {/* Card Hero Image */}
              {group.heroImage ? (
                <img
                  src={group.heroImage}
                  alt={displayName}
                  class="w-full h-full object-cover object-center"
                  loading="eager"
                />
              ) : (
                <div class="w-full h-full bg-gradient-to-br from-stone-800 to-stone-900" />
              )}

              {/* Shading gradient for readable typography & controls */}
              <div class="absolute inset-0 bg-gradient-to-t from-stone-950 via-stone-950/20 to-stone-950/40 pointer-events-none" />

              {/* Top Right Pin Button */}
              {isActive && (
                <button
                  type="button"
                  onClick={(e) => handlePinTop(group.id, e)}
                  aria-label="Set as top group"
                  class={`absolute top-4 right-4 p-2.5 rounded-full backdrop-blur-md transition-all active:scale-90 ${
                    isTop
                      ? 'bg-amber-400/20 text-amber-400 border border-amber-400/40 ring-2 ring-amber-400/30'
                      : 'bg-stone-900/60 text-stone-400 hover:text-amber-300 border border-white/10'
                  }`}
                >
                  <Icon icon={Crown} size={18} />
                </button>
              )}

              {/* Bottom banner area: 25% height typography */}
              <div
                style={{ height: `${PEEK_OFFSET_PX}px` }}
                class="absolute inset-x-0 bottom-0 px-5 flex items-center justify-between bg-gradient-to-t from-stone-950/95 via-stone-950/80 to-transparent pointer-events-none"
              >
                <div class="flex flex-col justify-center">
                  <span class="font-black text-2xl tracking-wider text-white drop-shadow-md">
                    {displayName}
                  </span>
                  <span class="text-[11px] font-semibold text-stone-400 tracking-wide">
                    {group.shortName}
                  </span>
                </div>
              </div>
            </div>
          );
        })}
      </main>
    </div>
  );
}
