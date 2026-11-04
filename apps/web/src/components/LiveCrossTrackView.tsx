import { useMemo } from 'preact/hooks';
import { CrossTrackEvent, SAMPLE_CROSS_TRACK_EVENTS } from '../utils/crossTrack';
import { Icon } from './Icon';
import { ChevronLeft, MapPin, ExternalLink, Clock } from '../utils/icons';

interface Props {
  eventId?: string;
  onBack: () => void;
}

export function LiveCrossTrackView({ eventId = '2026-11-03/mariage', onBack }: Props) {
  const event: CrossTrackEvent = useMemo(() => {
    return SAMPLE_CROSS_TRACK_EVENTS[eventId] || SAMPLE_CROSS_TRACK_EVENTS['2026-11-03/mariage'];
  }, [eventId]);

  return (
    <div class="h-[100dvh] w-full max-w-sm mx-auto flex flex-col justify-between overflow-hidden relative bg-stone-950 text-stone-100 select-none px-3.5 pt-3 pb-6">
      {/* Header: Pure Minimal (Navigation & Festival Title) */}
      <header class="flex items-center justify-between px-1 pt-1 pb-3 shrink-0 border-b border-white/5">
        <button
          onClick={onBack}
          class="flex items-center gap-1 -ml-1.5 px-2 py-1 rounded-lg text-stone-400 hover:text-white hover:bg-white/5 transition-colors"
          aria-label="戻る"
        >
          <Icon icon={ChevronLeft} size={18} />
          <span class="text-xs font-mono text-stone-300">PORTAL</span>
        </button>

        <div class="flex items-center gap-1.5">
          <span class="w-2 h-2 rounded-full bg-rose-500 animate-pulse" />
          <span class="text-xs font-mono font-bold tracking-widest text-stone-300 uppercase">
            LIVE TRACK
          </span>
        </div>
      </header>

      {/* Event Header Banner (現場情報) */}
      <div class="px-1 py-3 shrink-0 border-b border-white/5 flex flex-col gap-1">
        <div class="flex items-center justify-between">
          <h1 class="text-base font-extrabold text-white tracking-tight">
            {event.title}
          </h1>
          <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-white/5 text-stone-400 border border-white/10">
            {event.date} ({event.dayOfWeek})
          </span>
        </div>

        <div class="flex items-center justify-between text-[11px] text-stone-400">
          <div class="flex items-center gap-1 truncate">
            <Icon icon={MapPin} size={12} class="text-stone-500 shrink-0" />
            <span class="truncate">{event.venueName}</span>
          </div>

          <div class="flex items-center gap-2 shrink-0">
            {event.ticketUrl && (
              <a
                href={event.ticketUrl}
                target="_blank"
                rel="noopener noreferrer"
                class="flex items-center gap-0.5 text-stone-400 hover:text-white text-[10px] font-mono transition-colors"
              >
                <span>TICKET</span>
                <Icon icon={ExternalLink} size={10} />
              </a>
            )}
          </div>
        </div>
      </div>

      {/* Main Timeline Stream (フィルター・トグルなしの純粋な時間軸) */}
      <main class="flex-1 min-h-0 py-3 overflow-y-auto no-scrollbar relative flex flex-col gap-3">
        {event.sessions.map((session, idx) => {
          const isRibon = session.groupId === 'ribonclock';
          const isLive = session.type === 'live';

          const themeStyles = isRibon
            ? {
                border: isLive ? 'border-rose-500/40' : 'border-rose-500/20',
                glow: isLive ? 'from-rose-500/15 via-rose-500/5 to-transparent' : 'from-rose-950/20 to-transparent',
                badge: 'bg-rose-500/20 text-rose-300 border-rose-500/40',
                bar: 'bg-rose-500',
              }
            : {
                border: isLive ? 'border-pink-500/40' : 'border-pink-500/20',
                glow: isLive ? 'from-pink-500/15 via-pink-500/5 to-transparent' : 'from-pink-950/20 to-transparent',
                badge: 'bg-pink-500/20 text-pink-300 border-pink-500/40',
                bar: 'bg-pink-500',
              };

          return (
            <div
              key={`${session.groupId}-${idx}`}
              class={`relative rounded-xl border ${themeStyles.border} bg-stone-900/80 shadow-md p-3 flex flex-col gap-2 overflow-hidden`}
            >
              {/* Background Ambient Glow */}
              <div
                class={`absolute inset-0 bg-gradient-to-r ${themeStyles.glow} pointer-events-none opacity-40`}
              />

              {/* Top Row: Group, Session Type & Time */}
              <div class="relative z-10 flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <span class={`w-1.5 h-3 rounded-full ${themeStyles.bar}`} />
                  <span class="text-xs font-bold text-white tracking-wide">
                    {session.groupName}
                  </span>
                  <span class={`text-[9px] font-mono px-1.5 py-0.2 rounded border ${themeStyles.badge}`}>
                    {session.label}
                  </span>
                </div>

                <div class="flex items-center gap-1 font-mono text-xs font-bold text-stone-200">
                  <Icon icon={Clock} size={11} class="text-stone-500" />
                  <span>
                    {session.startTime} - {session.endTime}
                  </span>
                </div>
              </div>

              {/* Detail row (場所や特典内容) */}
              {(session.location || session.detail) && (
                <div class="relative z-10 text-[11px] text-stone-400 pl-3.5 border-l border-white/5 flex flex-col gap-0.5">
                  {session.location && (
                    <div class="text-stone-300 font-medium">
                      場所: {session.location}
                    </div>
                  )}
                  {session.detail && (
                    <div class="text-[10px] text-stone-400 leading-relaxed">
                      {session.detail}
                    </div>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </main>

      {/* Footer Info */}
      <footer class="pt-2 shrink-0 border-t border-white/5 flex items-center justify-between text-[11px] text-stone-500 px-1 font-mono">
        <span>TIMELINE STREAM</span>
        <span>2 GROUPS</span>
      </footer>
    </div>
  );
}
