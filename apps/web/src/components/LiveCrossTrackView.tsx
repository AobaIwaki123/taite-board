import { useState, useMemo, useEffect } from 'preact/hooks';
import {
  CrossTrackEvent,
  SAMPLE_CROSS_TRACK_EVENTS,
  parseMinutes,
  formatMinutes,
} from '../utils/crossTrack';
import { PatternLocationView } from './cross-track/PatternLocationView';
import { TimelineStepper } from './cross-track/TimelineStepper';
import { Icon } from './Icon';
import { ChevronLeft, ExternalLink } from '../utils/icons';

interface Props {
  eventId?: string;
  onBack: () => void;
}

export function LiveCrossTrackView({ eventId = '2026-11-03/mariage', onBack }: Props) {
  const event: CrossTrackEvent = useMemo(() => {
    return SAMPLE_CROSS_TRACK_EVENTS[eventId] || SAMPLE_CROSS_TRACK_EVENTS['2026-11-03/mariage'];
  }, [eventId]);

  // Current simulated minutes
  const [currentMinutes, setCurrentMinutes] = useState<number>(() => {
    const params = new URLSearchParams(window.location.search);
    const t = params.get('time');
    if (t && t !== 'now') return parseMinutes(t);
    // Default demo time: 12:20 (SAI²Rium live / Ribon Clock & Milky benefit)
    return parseMinutes('12:20');
  });

  const [isRealTime, setIsRealTime] = useState<boolean>(() => {
    const params = new URLSearchParams(window.location.search);
    return params.get('time') === 'now';
  });

  // Update clock if in real-time mode
  useEffect(() => {
    if (!isRealTime) return;

    const updateClock = () => {
      const now = new Date();
      setCurrentMinutes(now.getHours() * 60 + now.getMinutes());
    };

    updateClock();
    const timer = setInterval(updateClock, 30000);
    return () => clearInterval(timer);
  }, [isRealTime]);

  const handleSelectMinutes = (mins: number | ((prev: number) => number)) => {
    setIsRealTime(false);
    setCurrentMinutes((prev) => {
      const nextMins = typeof mins === 'function' ? mins(prev) : mins;
      const url = new URL(window.location.href);
      url.searchParams.set('time', formatMinutes(nextMins));
      window.history.replaceState({}, '', url.toString());
      return nextMins;
    });
  };

  const handleSetRealTime = () => {
    setIsRealTime(true);
    const now = new Date();
    const mins = now.getHours() * 60 + now.getMinutes();
    setCurrentMinutes(mins);
    const url = new URL(window.location.href);
    url.searchParams.set('time', 'now');
    window.history.replaceState({}, '', url.toString());
  };

  return (
    <div class="h-[100dvh] w-full max-w-sm mx-auto flex flex-col justify-between overflow-hidden relative bg-stone-950 text-stone-100 select-none px-3.5 pt-2 pb-2">
      {/* Header: Pure Minimal */}
      <header class="flex items-center justify-between px-1 pt-1 pb-1.5 shrink-0 border-b border-white/5">
        <button
          onClick={onBack}
          class="flex items-center gap-1 -ml-1.5 px-2 py-1 rounded-lg text-stone-400 hover:text-white hover:bg-white/5 transition-colors"
          aria-label="戻る"
        >
          <Icon icon={ChevronLeft} size={18} />
          <span class="text-xs font-mono text-stone-300">PORTAL</span>
        </button>

        <div class="flex items-center gap-2">
          <button
            onClick={handleSetRealTime}
            class={`flex items-center gap-1.5 px-2 py-0.5 rounded-full border transition-colors ${
              isRealTime
                ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40 font-bold'
                : 'bg-white/5 text-stone-400 border-white/10 hover:text-white'
            }`}
            title="現在時刻に同期"
          >
            <span
              class={`w-1.5 h-1.5 rounded-full ${
                isRealTime ? 'bg-emerald-400 animate-pulse' : 'bg-stone-500'
              }`}
            />
            <span class="text-[11px] font-mono">
              {isRealTime ? 'LIVE NOW' : 'SIM'}: {formatMinutes(currentMinutes)}
            </span>
          </button>
        </div>
      </header>

      {/* Event Info Strip */}
      <div class="px-1 py-1.5 shrink-0 flex items-center justify-between text-xs border-b border-white/5">
        <div class="flex items-center gap-1.5 truncate">
          <span class="font-extrabold text-white truncate">{event.title}</span>
          <span class="text-[10px] text-stone-400 font-mono shrink-0">
            @{event.venueName}
          </span>
        </div>
        {event.ticketUrl && (
          <a
            href={event.ticketUrl}
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center gap-0.5 text-stone-400 hover:text-white text-[10px] font-mono shrink-0"
          >
            <span>TICKET</span>
            <Icon icon={ExternalLink} size={10} />
          </a>
        )}
      </div>

      {/* Main Single Board View (STAGE vs BENEFIT) */}
      <main class="flex-1 min-h-0 flex flex-col overflow-hidden relative">
        <PatternLocationView event={event} currentMinutes={currentMinutes} />
      </main>

      {/* Bottom Controls: Stepper Debugger */}
      <footer class="pt-1.5 flex flex-col gap-1 shrink-0 border-t border-white/10">
        <TimelineStepper
          sessions={event.sessions}
          currentMinutes={currentMinutes}
          onSelectMinutes={handleSelectMinutes}
        />
      </footer>
    </div>
  );
}
