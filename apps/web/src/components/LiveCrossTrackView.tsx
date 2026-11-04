import { useState, useMemo, useEffect } from 'preact/hooks';
import {
  CrossTrackEvent,
  SAMPLE_CROSS_TRACK_EVENTS,
  parseMinutes,
  formatMinutes,
} from '../utils/crossTrack';
import { PatternLocationView } from './cross-track/PatternLocationView';
import { PatternStreamView } from './cross-track/PatternStreamView';
import { PatternTimerView } from './cross-track/PatternTimerView';
import { Icon } from './Icon';
import { ChevronLeft, ExternalLink } from '../utils/icons';

type PatternType = 'stream' | 'location' | 'timer';

interface Props {
  eventId?: string;
  onBack: () => void;
}

export function LiveCrossTrackView({ eventId = '2026-11-03/mariage', onBack }: Props) {
  const event: CrossTrackEvent = useMemo(() => {
    return SAMPLE_CROSS_TRACK_EVENTS[eventId] || SAMPLE_CROSS_TRACK_EVENTS['2026-11-03/mariage'];
  }, [eventId]);

  // Read initial pattern and sim time from URL search params
  const [pattern, setPattern] = useState<PatternType>(() => {
    const params = new URLSearchParams(window.location.search);
    const p = params.get('pattern');
    if (p === 'location' || p === 'timer') return p;
    return 'stream'; // Default: Pattern B (Minimal Stream)
  });

  // Current time state (defaults to 12:00 for demo if not specified, or real time)
  const [currentMinutes, setCurrentMinutes] = useState<number>(() => {
    const params = new URLSearchParams(window.location.search);
    const t = params.get('time');
    if (t) return parseMinutes(t);
    // Default demo time: 12:05 (RibonClock is on live stage)
    return parseMinutes('12:05');
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

  const handleSelectPattern = (newPattern: PatternType) => {
    setPattern(newPattern);
    const url = new URL(window.location.href);
    url.searchParams.set('pattern', newPattern);
    window.history.replaceState({}, '', url.toString());
  };

  const handleSelectTime = (timeStr: string) => {
    setIsRealTime(false);
    const mins = parseMinutes(timeStr);
    setCurrentMinutes(mins);
    const url = new URL(window.location.href);
    url.searchParams.set('time', timeStr);
    window.history.replaceState({}, '', url.toString());
  };

  return (
    <div class="h-[100dvh] w-full max-w-sm mx-auto flex flex-col justify-between overflow-hidden relative bg-stone-950 text-stone-100 select-none px-3.5 pt-3 pb-3">
      {/* Header: Pure Minimal (Navigation & Event Header) */}
      <header class="flex items-center justify-between px-1 pt-1 pb-2 shrink-0 border-b border-white/5">
        <button
          onClick={onBack}
          class="flex items-center gap-1 -ml-1.5 px-2 py-1 rounded-lg text-stone-400 hover:text-white hover:bg-white/5 transition-colors"
          aria-label="戻る"
        >
          <Icon icon={ChevronLeft} size={18} />
          <span class="text-xs font-mono text-stone-300">PORTAL</span>
        </button>

        <div class="flex items-center gap-2">
          <div class="flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-white/5 border border-white/10">
            <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
            <span class="text-[11px] font-mono text-emerald-300 font-bold">
              {formatMinutes(currentMinutes)}
            </span>
          </div>
        </div>
      </header>

      {/* Event Info Strip */}
      <div class="px-1 py-2 shrink-0 flex items-center justify-between text-xs border-b border-white/5">
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

      {/* Main Pattern Viewport */}
      <main class="flex-1 min-h-0 flex flex-col overflow-hidden relative">
        {pattern === 'stream' && (
          <PatternStreamView event={event} currentMinutes={currentMinutes} />
        )}
        {pattern === 'location' && (
          <PatternLocationView event={event} currentMinutes={currentMinutes} />
        )}
        {pattern === 'timer' && (
          <PatternTimerView event={event} currentMinutes={currentMinutes} />
        )}
      </main>

      {/* Bottom Prototype Compare Switcher (検証用バー) */}
      <footer class="pt-2 pb-1 border-t border-white/10 flex flex-col gap-1.5 shrink-0">
        {/* Pattern Switcher */}
        <div class="flex items-center justify-between text-[11px] font-mono bg-stone-900/90 rounded-lg p-1 border border-white/5">
          <button
            onClick={() => handleSelectPattern('stream')}
            class={`flex-1 py-1 rounded text-center transition-all ${
              pattern === 'stream'
                ? 'bg-white/15 text-white font-bold shadow-sm'
                : 'text-stone-400 hover:text-stone-200'
            }`}
          >
            B: NOW特化
          </button>
          <button
            onClick={() => handleSelectPattern('location')}
            class={`flex-1 py-1 rounded text-center transition-all ${
              pattern === 'location'
                ? 'bg-white/15 text-white font-bold shadow-sm'
                : 'text-stone-400 hover:text-stone-200'
            }`}
          >
            A: エリア別
          </button>
          <button
            onClick={() => handleSelectPattern('timer')}
            class={`flex-1 py-1 rounded text-center transition-all ${
              pattern === 'timer'
                ? 'bg-white/15 text-white font-bold shadow-sm'
                : 'text-stone-400 hover:text-stone-200'
            }`}
          >
            C: 残り時間
          </button>
        </div>

        {/* Time Simulator Presets */}
        <div class="flex items-center justify-between text-[10px] font-mono text-stone-400 px-1">
          <span class="text-stone-300">時間検証:</span>
          <button
            onClick={() => handleSelectTime('12:05')}
            class={`hover:text-white px-1.5 py-0.5 rounded ${
              currentMinutes === parseMinutes('12:05')
                ? 'text-emerald-400 font-bold bg-emerald-500/10'
                : ''
            }`}
          >
            12:05 (出番)
          </button>
          <button
            onClick={() => handleSelectTime('12:35')}
            class={`hover:text-white px-1.5 py-0.5 rounded ${
              currentMinutes === parseMinutes('12:35')
                ? 'text-emerald-400 font-bold bg-emerald-500/10'
                : ''
            }`}
          >
            12:35 (特典会)
          </button>
          <button
            onClick={() => handleSelectTime('13:45')}
            class={`hover:text-white px-1.5 py-0.5 rounded ${
              currentMinutes === parseMinutes('13:45')
                ? 'text-emerald-400 font-bold bg-emerald-500/10'
                : ''
            }`}
          >
            13:45 (待機中)
          </button>
          <button
            onClick={() => handleSelectTime('15:05')}
            class={`hover:text-white px-1.5 py-0.5 rounded ${
              currentMinutes === parseMinutes('15:05')
                ? 'text-emerald-400 font-bold bg-emerald-500/10'
                : ''
            }`}
          >
            15:05 (出番)
          </button>
        </div>
      </footer>
    </div>
  );
}
