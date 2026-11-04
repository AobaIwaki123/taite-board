import {
  CrossTrackEvent,
  evaluateSessions,
  getGroupTheme,
} from '../../utils/crossTrack';
import { Icon } from '../Icon';
import { MapPin } from '../../utils/icons';

interface Props {
  event: CrossTrackEvent;
  currentMinutes: number;
}

export function PatternLocationView({ event, currentMinutes }: Props) {
  const liveSessions = event.sessions.filter((s) => s.type === 'live');
  const benefitSessions = event.sessions.filter((s) => s.type === 'benefit');

  const liveEval = evaluateSessions(liveSessions, currentMinutes);
  const benefitEval = evaluateSessions(benefitSessions, currentMinutes);

  return (
    <div class="flex-1 flex flex-col gap-2.5 min-h-0 py-2">
      {/* 1. STAGE Section */}
      <div class="rounded-2xl border border-white/10 bg-stone-900/60 p-3.5 backdrop-blur-sm shrink-0 flex flex-col gap-2.5">
        {/* Header */}
        <div class="flex items-center justify-between border-b border-white/5 pb-2">
          <div class="flex items-center gap-2">
            <span
              class={`w-2 h-2 rounded-full ${
                liveEval.active.length > 0 ? 'bg-emerald-400 animate-pulse' : 'bg-stone-600'
              }`}
            />
            <span class="text-xs font-mono font-bold tracking-wider text-stone-200 uppercase">
              ステージ (STAGE)
            </span>
          </div>
          <span class="text-[11px] text-stone-400 font-mono">メインフロア</span>
        </div>

        {/* Stage Active Content */}
        {liveEval.active.length > 0 ? (
          (() => {
            const item = liveEval.active[0];
            const theme = getGroupTheme(item.session.groupId || item.session.groupName);

            return (
              <div
                class={`relative rounded-xl border p-3 flex flex-col gap-1.5 overflow-hidden shadow-md ${theme.border} bg-gradient-to-br ${theme.bgGlow}`}
              >
                <div class="flex items-center justify-between">
                  <span class="text-[9px] font-mono font-extrabold uppercase px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/40">
                    NOW ON AIR
                  </span>
                  <span class="text-xs font-mono font-bold text-stone-200">
                    {item.session.startTime} - {item.session.endTime}
                  </span>
                </div>

                <div class="text-lg font-extrabold text-white tracking-tight mt-0.5">
                  {item.session.groupName}
                </div>

                {item.session.detail && (
                  <div class="text-[11px] text-stone-300 line-clamp-1 bg-white/5 px-2 py-1 rounded">
                    {item.session.detail}
                  </div>
                )}
              </div>
            );
          })()
        ) : (
          <div class="py-4 text-center text-xs text-stone-400 font-mono">
            現在ステージの演目はありません
          </div>
        )}

        {/* Stage NEXT */}
        <div class="pt-1.5 border-t border-white/5 flex items-center justify-between text-xs">
          <span class="text-[10px] font-mono font-bold text-stone-400 uppercase tracking-wider">
            NEXT
          </span>
          {liveEval.next ? (
            <div class="flex items-center gap-2 text-stone-300 font-mono text-[11px] truncate">
              <span class="text-stone-200 font-bold shrink-0">{liveEval.next.session.startTime}〜</span>
              <span class="text-white truncate font-medium">{liveEval.next.session.groupShortName}</span>
            </div>
          ) : (
            <span class="text-stone-400 text-[11px] font-mono">本日のステージは終了</span>
          )}
        </div>
      </div>

      {/* 2. BENEFIT Section (Takes flexible remaining space with smooth scrolling) */}
      <div class="rounded-2xl border border-white/10 bg-stone-900/60 p-3.5 backdrop-blur-sm flex-1 min-h-0 flex flex-col gap-2.5">
        {/* Header */}
        <div class="flex items-center justify-between border-b border-white/5 pb-2 shrink-0">
          <div class="flex items-center gap-2">
            <span
              class={`w-2 h-2 rounded-full ${
                benefitEval.active.length > 0 ? 'bg-emerald-400 animate-pulse' : 'bg-stone-600'
              }`}
            />
            <span class="text-xs font-mono font-bold tracking-wider text-stone-200 uppercase">
              特典会 (BENEFIT)
            </span>
          </div>
          <div class="flex items-center gap-2 text-[11px]">
            <span class="text-stone-400 font-mono">並行物販エリア</span>
            {benefitEval.active.length > 0 && (
              <span class="font-mono text-emerald-400 font-bold">
                {benefitEval.active.length}組開催中
              </span>
            )}
          </div>
        </div>

        {/* Benefit Active Cards (Scrollable if multiple concurrent groups) */}
        <div class="flex-1 overflow-y-auto no-scrollbar flex flex-col gap-2 min-h-0 py-0.5">
          {benefitEval.active.length > 0 ? (
            benefitEval.active.map((item, idx) => {
              const theme = getGroupTheme(item.session.groupId || item.session.groupName);

              return (
                <div
                  key={idx}
                  class={`relative rounded-xl border p-3 flex flex-col gap-1.5 overflow-hidden shadow-md shrink-0 ${theme.border} bg-gradient-to-br ${theme.bgGlow}`}
                >
                  <div class="flex items-center justify-between">
                    <span class="text-[9px] font-mono font-extrabold uppercase px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/40">
                      NOW ON AIR
                    </span>
                    <span class="text-xs font-mono font-bold text-stone-200">
                      {item.session.startTime} - {item.session.endTime}
                    </span>
                  </div>

                  <div class="flex items-baseline justify-between mt-0.5">
                    <div class="text-base font-extrabold text-white tracking-tight truncate">
                      {item.session.groupName}
                    </div>
                    {item.session.location && (
                      <div class="flex items-center gap-1 text-xs text-amber-300 font-medium shrink-0 ml-2">
                        <Icon icon={MapPin} size={11} />
                        <span>{item.session.location}</span>
                      </div>
                    )}
                  </div>
                </div>
              );
            })
          ) : (
            <div class="flex-1 flex flex-col items-center justify-center py-4 text-center">
              <span class="text-xs text-stone-400 font-mono">
                現在進行中の特典会はありません
              </span>
            </div>
          )}
        </div>

        {/* Benefit NEXT */}
        <div class="pt-1.5 border-t border-white/5 flex items-center justify-between text-xs shrink-0">
          <span class="text-[10px] font-mono font-bold text-stone-400 uppercase tracking-wider">
            NEXT
          </span>
          {benefitEval.next ? (
            <div class="flex items-center gap-2 text-stone-300 font-mono text-[11px] truncate">
              <span class="text-stone-200 font-bold shrink-0">{benefitEval.next.session.startTime}〜</span>
              <span class="text-white truncate font-medium">{benefitEval.next.session.groupShortName}</span>
              {benefitEval.next.session.location && (
                <span class="text-amber-300 text-[10px] shrink-0">({benefitEval.next.session.location})</span>
              )}
            </div>
          ) : (
            <span class="text-stone-400 text-[11px] font-mono">本日の特典会は終了</span>
          )}
        </div>
      </div>
    </div>
  );
}
