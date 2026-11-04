import { CrossTrackEvent, evaluateSessions, getGroupTheme } from '../../utils/crossTrack';
import { Icon } from '../Icon';
import { MapPin } from '../../utils/icons';

interface Props {
  event: CrossTrackEvent;
  currentMinutes: number;
}

export function PatternStreamView({ event, currentMinutes }: Props) {
  const { active, next } = evaluateSessions(event.sessions, currentMinutes);

  return (
    <div class="flex-1 flex flex-col justify-between py-2 gap-3 min-h-0">
      {/* NOW Section (Hero Focus with Timer Integrated) */}
      <div class="flex-1 flex flex-col gap-2 min-h-0">
        <div class="flex items-center justify-between px-1">
          <div class="flex items-center gap-1.5">
            <span class="w-2.5 h-2.5 rounded-full bg-emerald-400 animate-pulse" />
            <span class="text-xs font-mono font-extrabold tracking-widest text-emerald-400 uppercase">
              NOW HAPPENING
            </span>
          </div>
          <span class="text-[11px] font-mono text-stone-300">
            {active.length} 件進行中
          </span>
        </div>

        <div class="flex-1 overflow-y-auto no-scrollbar flex flex-col gap-2.5 py-0.5">
          {active.length > 0 ? (
            active.map((item, idx) => {
              const theme = getGroupTheme(item.session.groupId || item.session.groupName);
              const isLive = item.session.type === 'live';
              const progressPercent = Math.round(item.progress * 100);

              return (
                <div
                  key={`${item.session.groupId}-${idx}`}
                  class={`relative rounded-2xl border p-3.5 flex flex-col gap-2 shadow-lg overflow-hidden ${theme.border} bg-gradient-to-br ${theme.bgGlow}`}
                >
                  {/* Top Line: Session Label, Range & Remaining Minutes */}
                  <div class="flex items-center justify-between">
                    <span
                      class={`text-[10px] font-mono font-extrabold uppercase px-2 py-0.5 rounded-full border ${
                        isLive
                          ? 'bg-rose-500/20 text-rose-300 border-rose-500/40'
                          : 'bg-amber-500/20 text-amber-300 border-amber-500/40'
                      }`}
                    >
                      {item.session.label}
                    </span>

                    <div class="flex items-center gap-2">
                      <span class="text-xs font-mono text-stone-300">
                        {item.session.startTime} - {item.session.endTime}
                      </span>
                      <span class="text-xs font-mono font-extrabold text-emerald-400 px-1.5 py-0.5 rounded bg-emerald-500/20 border border-emerald-500/40">
                        残り {item.remainingMinutes}分
                      </span>
                    </div>
                  </div>

                  {/* Group Name & Location */}
                  <div>
                    <div class="text-lg font-extrabold text-white tracking-tight">
                      {item.session.groupName}
                    </div>

                    {item.session.location && (
                      <div class="flex items-center gap-1.5 text-xs text-amber-300 font-medium mt-0.5">
                        <Icon icon={MapPin} size={12} />
                        <span>場所: {item.session.location}</span>
                      </div>
                    )}
                  </div>

                  {/* Optional Details */}
                  {item.session.detail && (
                    <div class="text-[11px] text-stone-300 bg-white/5 p-2 rounded-lg border border-white/5">
                      {item.session.detail}
                    </div>
                  )}

                  {/* Integrated Slim Progress Bar */}
                  <div class="w-full bg-white/10 rounded-full h-1 overflow-hidden mt-0.5">
                    <div
                      class={`h-full transition-all duration-300 ${theme.bar}`}
                      style={{ width: `${progressPercent}%` }}
                    />
                  </div>
                </div>
              );
            })
          ) : (
            <div class="flex-1 flex flex-col items-center justify-center rounded-2xl border border-dashed border-white/10 bg-stone-900/40 p-6 text-center">
              <span class="text-xs font-mono tracking-widest text-stone-400 uppercase">
                INTERVAL
              </span>
              <span class="text-sm text-stone-200 font-bold mt-1">
                現在進行中の演目・特典会はありません
              </span>
              {next && (
                <div class="mt-3 flex items-center gap-2 text-xs font-mono bg-white/5 px-3 py-1.5 rounded-full border border-white/10">
                  <span class="text-stone-300">次の出番まで</span>
                  <span class="text-emerald-400 font-bold">あと {next.remainingMinutes}分</span>
                  <span class="text-stone-400">({next.session.startTime} {next.session.groupShortName})</span>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* NEXT Section (Minimal Footer) */}
      <div class="rounded-xl border border-white/10 bg-stone-900/80 p-2.5 flex flex-col gap-1 shrink-0">
        <div class="flex items-center justify-between">
          <span class="text-[10px] font-mono font-extrabold text-stone-400 uppercase tracking-widest">
            UPCOMING NEXT
          </span>
          {next && (
            <span class="text-[11px] font-mono text-emerald-400">
              {next.remainingMinutes}分後開始
            </span>
          )}
        </div>

        {next ? (
          <div class="flex items-center justify-between pt-0.5">
            <div class="flex items-center gap-2 truncate">
              <span class="text-xs font-mono font-bold text-white shrink-0">
                {next.session.startTime}
              </span>
              <span class="text-[10px] px-1.5 py-0.5 rounded bg-white/10 text-stone-300 border border-white/10 shrink-0">
                {next.session.label}
              </span>
              <span class="text-xs font-extrabold text-stone-200 truncate">
                {next.session.groupShortName}
              </span>
            </div>
            {next.session.location && (
              <span class="text-xs text-amber-300 font-mono shrink-0 ml-2">
                {next.session.location}
              </span>
            )}
          </div>
        ) : (
          <div class="text-xs text-stone-400 py-1 font-mono">
            本日の予定はすべて終了しました
          </div>
        )}
      </div>
    </div>
  );
}
