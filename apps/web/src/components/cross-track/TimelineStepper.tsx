import { useState, useEffect, useMemo } from 'preact/hooks';
import { TimelineStep, extractTimelineSteps, CrossTrackSession, formatMinutes } from '../../utils/crossTrack';

interface Props {
  sessions: CrossTrackSession[];
  currentMinutes: number;
  onSelectMinutes: (minutes: number | ((prev: number) => number)) => void;
}

export function TimelineStepper({ sessions, currentMinutes, onSelectMinutes }: Props) {
  const steps: TimelineStep[] = useMemo(() => {
    return extractTimelineSteps(sessions);
  }, [sessions]);

  // Current step index based on currentMinutes
  const currentStepIndex = useMemo(() => {
    let closestIndex = 0;
    let minDiff = Infinity;
    steps.forEach((step, idx) => {
      const diff = Math.abs(step.minutes - currentMinutes);
      if (diff < minDiff) {
        minDiff = diff;
        closestIndex = idx;
      }
    });
    return closestIndex;
  }, [steps, currentMinutes]);

  const [isPlaying, setIsPlaying] = useState<boolean>(false);

  // Auto playback of steps
  useEffect(() => {
    if (!isPlaying) return;

    const timer = setInterval(() => {
      onSelectMinutes((prevMinutes) => {
        let idx = steps.findIndex((s) => s.minutes > prevMinutes);
        if (idx === -1) {
          idx = 0; // Loop back
        }
        return steps[idx].minutes;
      });
    }, 2000);

    return () => clearInterval(timer);
  }, [isPlaying, steps, onSelectMinutes]);

  const handlePrev = () => {
    setIsPlaying(false);
    if (currentStepIndex > 0) {
      onSelectMinutes(steps[currentStepIndex - 1].minutes);
    } else {
      onSelectMinutes(steps[steps.length - 1].minutes);
    }
  };

  const handleNext = () => {
    setIsPlaying(false);
    if (currentStepIndex < steps.length - 1) {
      onSelectMinutes(steps[currentStepIndex + 1].minutes);
    } else {
      onSelectMinutes(steps[0].minutes);
    }
  };

  const currentStep = steps[currentStepIndex] || steps[0];
  const minMinutes = steps[0]?.minutes || 11 * 60;
  const maxMinutes = steps[steps.length - 1]?.minutes || 17 * 60;

  return (
    <div class="flex flex-col gap-1.5 p-2 rounded-xl bg-stone-900/95 border border-white/10 font-mono shadow-md">
      {/* Top Row: Stepper Controls & Current Step Info */}
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <button
            onClick={handlePrev}
            class="px-2.5 py-1 rounded bg-white/10 hover:bg-white/20 active:bg-white/30 text-stone-200 text-meta font-bold transition-colors"
            title="前の変化点へ"
            aria-label="前の変化点"
          >
            ◀
          </button>

          <button
            onClick={() => setIsPlaying(!isPlaying)}
            class={`px-3 py-1 rounded text-meta font-bold transition-colors ${
              isPlaying
                ? 'bg-rose-500 text-white animate-pulse'
                : 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40 hover:bg-emerald-500/30'
            }`}
            title={isPlaying ? '停止' : '自動再生'}
            aria-label={isPlaying ? '停止' : '自動再生'}
          >
            {isPlaying ? '❚❚ 停止' : '▶ 再生'}
          </button>

          <button
            onClick={handleNext}
            class="px-2.5 py-1 rounded bg-white/10 hover:bg-white/20 active:bg-white/30 text-stone-200 text-meta font-bold transition-colors"
            title="次の変化点へ"
            aria-label="次の変化点"
          >
            ▶
          </button>
        </div>

        {/* Step Index & Time */}
        <div class="flex items-center gap-2 text-right">
          <span class="text-stone-400 text-floor">
            Step {currentStepIndex + 1}/{steps.length}
          </span>
          <span class="font-bold text-white text-meta px-2 py-0.5 rounded bg-white/10">
            {formatMinutes(currentMinutes)}
          </span>
        </div>
      </div>

      {/* Step Description */}
      <div class="text-floor text-amber-300 truncate px-0.5">
        {currentStep ? currentStep.description : 'タイテ境界'}
      </div>

      {/* Time Slider */}
      <div class="flex items-center gap-2 pt-0.5">
        <span class="text-floor text-stone-300 shrink-0">
          {formatMinutes(minMinutes)}
        </span>
        <input
          type="range"
          min={minMinutes}
          max={maxMinutes}
          step={5}
          value={currentMinutes}
          onInput={(e) => {
            setIsPlaying(false);
            onSelectMinutes(Number((e.target as HTMLInputElement).value));
          }}
          class="flex-1 h-1.5 bg-stone-700 rounded-lg appearance-none cursor-pointer accent-emerald-400"
          aria-label="タイムスライダー"
        />
        <span class="text-floor text-stone-300 shrink-0">
          {formatMinutes(maxMinutes)}
        </span>
      </div>
    </div>
  );
}
