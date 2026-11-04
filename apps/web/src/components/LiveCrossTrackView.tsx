import { useState, useMemo, useEffect } from 'preact/hooks';
import {
  CrossTrackEvent,
  SAMPLE_CROSS_TRACK_EVENTS,
  parseMinutes,
  formatMinutes,
} from '../utils/crossTrack';
import { PatternSignboardClean } from './cross-track/PatternSignboardClean';
import { PatternSignboardDetailed } from './cross-track/PatternSignboardDetailed';
import { TimelineStepper } from './cross-track/TimelineStepper';
import { Icon } from './Icon';
import { ChevronLeft, ExternalLink, MapPin, SlidersHorizontal, Clock } from '../utils/icons';
import { REGISTERED_GROUPS } from '../config/group';

export type TimelineTheme = 'clean' | 'detailed';

interface Props {
  eventId?: string;
  fromGroupId?: string;
  onBack: () => void;
}

export function LiveCrossTrackView({
  eventId = '2026-11-03/mariage',
  fromGroupId,
  onBack,
}: Props) {
  const event: CrossTrackEvent = useMemo(() => {
    return SAMPLE_CROSS_TRACK_EVENTS[eventId] || SAMPLE_CROSS_TRACK_EVENTS['2026-11-03/mariage'];
  }, [eventId]);

  const fromGroupName = useMemo(() => {
    if (!fromGroupId) return null;
    return REGISTERED_GROUPS[fromGroupId]?.name || fromGroupId;
  }, [fromGroupId]);

  // デザイン案の切り替え状態 (プロ仕様: clean / ディテール案: detailed)
  const [theme, setTheme] = useState<TimelineTheme>(() => {
    const params = new URLSearchParams(window.location.search);
    const t = params.get('theme');
    return t === 'clean' ? 'clean' : 'detailed';
  });

  // タイムシミュレーター（デバッグステッパー）の開閉状態
  const [isSimulatorOpen, setIsSimulatorOpen] = useState<boolean>(() => {
    const params = new URLSearchParams(window.location.search);
    return params.get('debug') === '1' || params.get('debug') === 'true';
  });

  // 現在のシミュレーション時刻
  const [currentMinutes, setCurrentMinutes] = useState<number>(() => {
    const params = new URLSearchParams(window.location.search);
    const t = params.get('time');
    if (t && t !== 'now') return parseMinutes(t);
    // デフォルトデモ時刻: 12:20
    return parseMinutes('12:20');
  });

  const [isRealTime, setIsRealTime] = useState<boolean>(() => {
    const params = new URLSearchParams(window.location.search);
    return params.get('time') === 'now';
  });

  // 進行押し時間（常時描画・現場調整用: 0, 5, 10, 15, 20, 30分）
  const [delayMinutes, setDelayMinutes] = useState<number>(() => {
    const params = new URLSearchParams(window.location.search);
    const d = params.get('delay');
    return d ? parseInt(d, 10) : 0;
  });

  // 押し時間を考慮した実効時刻
  const effectiveMinutes = currentMinutes - delayMinutes;

  // リアルタイムモード時の自動時計更新
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

  const handleSelectTheme = (newTheme: TimelineTheme) => {
    setTheme(newTheme);
    const url = new URL(window.location.href);
    url.searchParams.set('theme', newTheme);
    window.history.replaceState({}, '', url.toString());
  };

  const handleToggleSimulator = () => {
    setIsSimulatorOpen((prev) => {
      const next = !prev;
      const url = new URL(window.location.href);
      if (next) {
        url.searchParams.set('debug', '1');
      } else {
        url.searchParams.delete('debug');
      }
      window.history.replaceState({}, '', url.toString());
      return next;
    });
  };

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

  const handleChangeDelay = (m: number) => {
    setDelayMinutes(m);
    const url = new URL(window.location.href);
    if (m > 0) {
      url.searchParams.set('delay', String(m));
    } else {
      url.searchParams.delete('delay');
    }
    window.history.replaceState({}, '', url.toString());
  };

  return (
    <div class="h-[100dvh] w-full max-w-sm mx-auto flex flex-col justify-between overflow-hidden relative bg-stone-950 text-stone-100 select-none px-3.5 pt-2 pb-2">
      {/* 1. Header Bar: Navigation & Real-time Clock */}
      <header class="flex items-center justify-between px-1 pt-1 pb-2 shrink-0 border-b border-stone-800">
        <button
          onClick={onBack}
          class="flex items-center gap-1 -ml-1.5 px-2 py-1 rounded-lg text-stone-400 hover:text-white hover:bg-stone-900 transition-colors group"
          aria-label="戻る"
        >
          <Icon icon={ChevronLeft} size={20} class="text-rose-400 group-hover:-translate-x-0.5 transition-transform" />
          <span class="text-meta font-medium text-stone-300 group-hover:text-white truncate max-w-[140px]">
            {fromGroupName ? fromGroupName : 'ポータル'}
          </span>
        </button>

        <div class="flex items-center gap-2">
          {delayMinutes > 0 && (
            <span class="text-floor font-mono font-bold px-2 py-0.5 rounded bg-amber-500/10 text-amber-300 border border-amber-500/30">
              +{delayMinutes}分押
            </span>
          )}

          <button
            onClick={handleSetRealTime}
            class={`flex items-center gap-1.5 px-2.5 py-1 rounded-full border transition-colors ${
              isRealTime
                ? 'bg-emerald-500/10 text-emerald-300 border-emerald-500/30 font-medium'
                : 'bg-stone-900 text-stone-400 border-stone-800 hover:text-white'
            }`}
            title="現在時刻に同期"
          >
            <span
              class={`w-2 h-2 rounded-full ${
                isRealTime ? 'bg-emerald-400' : 'bg-stone-600'
              }`}
            />
            <span class="text-meta font-mono font-medium">
              {isRealTime ? 'LIVE' : 'SIM'}: {formatMinutes(currentMinutes)}
            </span>
          </button>
        </div>
      </header>

      {/* 2. Event Title & Details (見切れのない自然な2段レイアウト) */}
      <div class="px-1 pt-2 pb-2 shrink-0 flex flex-col gap-1 border-b border-stone-800/80">
        <div class="text-subheading font-bold text-white leading-snug tracking-tight break-words">
          {event.title}
        </div>
        <div class="flex items-center justify-between text-meta text-stone-400 pt-0.5">
          <a
            href={event.venueUrl || '#'}
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center gap-1 hover:text-stone-200 transition-colors truncate max-w-[210px]"
          >
            <Icon icon={MapPin} size={14} class="text-stone-400 shrink-0" />
            <span class="truncate">{event.venueName}</span>
          </a>

          {event.ticketUrl && (
            <a
              href={event.ticketUrl}
              target="_blank"
              rel="noopener noreferrer"
              class="flex items-center gap-1 text-stone-400 hover:text-stone-200 text-floor font-mono px-2 py-0.5 rounded border border-stone-800 hover:border-stone-700 transition-colors shrink-0"
            >
              <span>チケット</span>
              <Icon icon={ExternalLink} size={11} />
            </a>
          )}
        </div>
      </div>

      {/* 3. Controls: 進行押し調整 & 案A1/案A2切り替えセレクター */}
      <div class="px-1 py-1.5 shrink-0 flex flex-col gap-1.5 border-b border-stone-800/80 bg-stone-900/50 rounded-lg my-1.5">
        {/* 進行押し調整 */}
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-1 text-floor font-mono text-stone-400 font-medium shrink-0">
            <Icon icon={Clock} size={12} class="text-stone-400" />
            <span>進行調整:</span>
          </div>
          <div class="flex items-center gap-1 font-mono">
            {[0, 5, 10, 15, 20, 30].map((m) => {
              const isSelected = delayMinutes === m;
              return (
                <button
                  key={m}
                  onClick={() => handleChangeDelay(m)}
                  class={`px-2 py-0.5 rounded text-floor font-medium transition-colors ${
                    isSelected
                      ? 'bg-amber-400/90 text-stone-950 font-bold'
                      : 'bg-stone-800/80 text-stone-400 hover:text-stone-200 hover:bg-stone-800'
                  }`}
                >
                  {m === 0 ? '定刻' : `+${m}m`}
                </button>
              );
            })}
          </div>
        </div>

        {/* 案A1（プロ仕様） / 案A2（ディテール案） 切り替えスイッチ */}
        <div class="flex items-center justify-between pt-1 border-t border-stone-800/60 text-floor">
          <span class="font-mono text-stone-400">SIGNBOARD DESIGN:</span>
          <div class="flex items-center p-0.5 rounded-md bg-stone-950 border border-stone-800">
            <button
              onClick={() => handleSelectTheme('clean')}
              class={`px-2.5 py-0.5 rounded text-floor font-medium transition-colors ${
                theme === 'clean'
                  ? 'bg-stone-800 text-white font-bold shadow-sm'
                  : 'text-stone-400 hover:text-stone-200'
              }`}
            >
              プロ仕様
            </button>
            <button
              onClick={() => handleSelectTheme('detailed')}
              class={`px-2.5 py-0.5 rounded text-floor font-medium transition-colors ${
                theme === 'detailed'
                  ? 'bg-stone-800 text-white font-bold shadow-sm'
                  : 'text-stone-400 hover:text-stone-200'
              }`}
            >
              ディテール
            </button>
          </div>
        </div>
      </div>

      {/* 4. Main Clean Board Viewport */}
      <main class="flex-1 min-h-0 flex flex-col overflow-hidden relative">
        {theme === 'detailed' ? (
          <PatternSignboardDetailed
            event={event}
            currentMinutes={effectiveMinutes}
          />
        ) : (
          <PatternSignboardClean
            event={event}
            currentMinutes={effectiveMinutes}
          />
        )}
      </main>

      {/* 5. Footer & Optional Simulator Drawer */}
      {isSimulatorOpen ? (
        <footer class="pt-1.5 flex flex-col gap-1 shrink-0 border-t border-stone-800 bg-stone-900/95 rounded-t-xl p-2 mt-1">
          <div class="flex items-center justify-between px-1 pb-1 text-floor font-mono text-stone-400 border-b border-stone-800">
            <span class="font-medium text-stone-300">タイムシミュレーター</span>
            <button
              onClick={handleToggleSimulator}
              class="text-stone-400 hover:text-white px-2 py-0.5 rounded hover:bg-stone-800 text-floor"
            >
              閉じる ✕
            </button>
          </div>
          <TimelineStepper
            sessions={event.sessions}
            currentMinutes={currentMinutes}
            onSelectMinutes={handleSelectMinutes}
          />
        </footer>
      ) : (
        <footer class="pt-2 pb-1 shrink-0 border-t border-stone-800 flex items-center justify-between text-floor font-mono text-stone-400 px-1">
          <span class="tracking-wider">当日のタイムテーブル</span>
          <button
            onClick={handleToggleSimulator}
            class="flex items-center gap-1 text-stone-400 hover:text-stone-200 px-2 py-0.5 rounded border border-stone-800 bg-stone-900/50 transition-colors"
            title="時間シミュレーターを開く"
          >
            <Icon icon={SlidersHorizontal} size={11} />
            <span>シミュレーター</span>
          </button>
        </footer>
      )}
    </div>
  );
}
