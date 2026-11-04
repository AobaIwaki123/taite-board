import { CrossTrackEvent, evaluateSessions } from '../../utils/crossTrack';
import { Icon } from '../Icon';
import { MapPin } from '../../utils/icons';

interface Props {
  event: CrossTrackEvent;
  currentMinutes: number;
}

export function PatternTimerView({ event, currentMinutes }: Props) {
  const { active, next } = evaluateSessions(event.sessions, currentMinutes);

  return (
    <div class="flex-1 flex flex-col justify-between py-2 gap-3 min-h-0">
      {/* Active Sessions with Remaining Timer & Progress Bar */}
      <div class="flex-1 flex flex-col gap-3 min-h-0 overflow-y-auto no-scrollbar">
        {active.length > 0 ? (
          active.map((item, idx) => {
            const isRibon = item.session.groupId === 'ribonclock';
            const progressPercent = Math.round(item.progress * 100);

            return (
              <div
                key={idx}
                class={`rounded-2xl border p-4 flex flex-col gap-3 shadow-lg relative overflow-hidden ${
                  isRibon
                    ? 'border-rose-500/40 bg-stone-900/90'
                    : 'border-pink-500/40 bg-stone-900/90'
                }`}
              >
                {/* Header: Label & Time */}
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-1.5">
                    <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
                    <span class="text-xs font-mono font-bold text-stone-300">
                      {item.session.label}
                    </span>
                  </div>
                  <span class="text-xs font-mono text-stone-300">
                    {item.session.startTime} - {item.session.endTime}
                  </span>
                </div>

                {/* Group Name & Location */}
                <div>
                  <h2 class="text-xl font-black text-white tracking-tight">
                    {item.session.groupName}
                  </h2>
                  {item.session.location && (
                    <div class="flex items-center gap-1 text-xs text-amber-300 mt-1 font-medium">
                      <Icon icon={MapPin} size={12} />
                      <span>エリア: {item.session.location}</span>
                    </div>
                  )}
                </div>

                {/* Remaining Time Highlight */}
                <div class="flex items-baseline justify-between pt-1">
                  <span class="text-xs text-stone-300 font-medium">終了まで</span>
                  <div class="flex items-baseline gap-1 font-mono">
                    <span class="text-2xl font-black text-emerald-400">
                      残り {item.remainingMinutes}
                    </span>
                    <span class="text-xs text-emerald-400 font-bold">分</span>
                  </div>
                </div>

                {/* Progress Bar */}
                <div class="w-full bg-white/10 rounded-full h-1.5 overflow-hidden">
                  <div
                    class={`h-full transition-all duration-500 ${
                      isRibon ? 'bg-rose-500' : 'bg-pink-500'
                    }`}
                    style={{ width: `${progressPercent}%` }}
                  />
                </div>
              </div>
            );
          })
        ) : (
          /* Idle / Break Time Focus */
          <div class="flex-1 flex flex-col items-center justify-center rounded-2xl border border-white/10 bg-stone-900/60 p-6 text-center">
            <span class="text-xs font-mono tracking-widest text-stone-300 uppercase">
              INTERVAL / WAITING
            </span>
            {next ? (
              <div class="flex flex-col items-center gap-1 mt-3">
                <span class="text-xs text-stone-300">次の出番まで</span>
                <div class="flex items-baseline gap-1 font-mono">
                  <span class="text-4xl font-black text-emerald-400">
                    {next.remainingMinutes}
                  </span>
                  <span class="text-sm font-bold text-emerald-400">分</span>
                </div>
                <span class="text-xs text-stone-300 mt-1">
                  ({next.session.startTime} {next.session.groupName} {next.session.label})
                </span>
              </div>
            ) : (
              <span class="text-sm text-stone-300 mt-2 font-mono">本日の演目は終了しました</span>
            )}
          </div>
        )}
      </div>

      {/* NEXT Preview Row */}
      {next && active.length > 0 && (
        <div class="rounded-xl border border-white/10 bg-stone-900/80 p-3 flex items-center justify-between shrink-0">
          <div class="flex items-center gap-2">
            <span class="text-[10px] font-mono font-bold text-stone-300 uppercase px-1.5 py-0.5 rounded bg-white/10">
              NEXT
            </span>
            <span class="text-xs font-mono font-bold text-white">
              {next.session.startTime}
            </span>
            <span class="text-xs font-bold text-stone-200">
              {next.session.groupShortName}
            </span>
            <span class="text-[11px] text-stone-300">({next.session.label})</span>
          </div>
          <span class="text-[11px] font-mono text-emerald-400 font-medium">
            あと{next.remainingMinutes}分
          </span>
        </div>
      )}
    </div>
  );
}
