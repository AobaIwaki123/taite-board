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
    <div class="flex-1 flex flex-col gap-2.5 min-h-0 py-1.5 w-full overflow-hidden">
      {/* 1. STAGE Section (Stable shrink-0 container) */}
      <section class="rounded-2xl border border-white/10 bg-stone-900/70 p-3.5 backdrop-blur-sm shrink-0 flex flex-col gap-2 shadow-sm">
        {/* Header */}
        <div class="flex items-center justify-between border-b border-white/5 pb-2 shrink-0">
          <div class="flex items-center gap-2">
            <span
              class={`w-2.5 h-2.5 rounded-full ${
                liveEval.active.length > 0 ? 'bg-emerald-400 animate-pulse' : 'bg-stone-600'
              }`}
            />
            <span class="text-xs font-mono font-extrabold tracking-wider text-stone-200 uppercase">
              ステージ (STAGE)
            </span>
          </div>
          <span class="text-xs text-stone-400 font-mono">メインフロア</span>
        </div>

        {/* Stage Active Content */}
        {liveEval.active.length > 0 ? (
          (() => {
            const item = liveEval.active[0];
            const theme = getGroupTheme(item.session.groupId || item.session.groupName);

            return (
              <div
                class={`relative rounded-xl border p-3 flex flex-col gap-2 shadow-md shrink-0 ${theme.border} bg-gradient-to-br ${theme.bgGlow}`}
              >
                {/* Time Range */}
                <div class="flex items-center justify-between">
                  <span class="text-xs font-mono font-bold text-stone-300 tracking-wider">
                    {item.session.startTime} - {item.session.endTime}
                  </span>
                </div>

                {/* Group Name (Large font, anti-breakage) */}
                <div class="text-xl font-black text-white tracking-tight leading-tight break-words">
                  {item.session.groupName}
                </div>

                {/* Optional Detail */}
                {item.session.detail && (
                  <div class="text-xs text-stone-300 line-clamp-2 bg-black/30 p-2 rounded-lg border border-white/5 mt-0.5 leading-relaxed">
                    {item.session.detail}
                  </div>
                )}
              </div>
            );
          })()
        ) : (
          <div class="py-5 text-center text-xs text-stone-400 font-mono">
            現在ステージの演目はありません
          </div>
        )}

        {/* Stage NEXT (Footer) */}
        <div class="pt-1.5 border-t border-white/5 flex items-center justify-between text-xs shrink-0">
          <span class="text-[11px] font-mono font-bold text-stone-400 uppercase tracking-wider shrink-0">
            NEXT
          </span>
          {liveEval.next ? (
            <div class="flex items-center gap-2 text-stone-300 font-mono text-xs truncate ml-2">
              <span class="text-emerald-400 font-bold shrink-0">{liveEval.next.session.startTime}〜</span>
              <span class="text-white font-bold truncate">{liveEval.next.session.groupShortName}</span>
            </div>
          ) : (
            <span class="text-stone-400 text-xs font-mono">本日のステージは終了</span>
          )}
        </div>
      </section>

      {/* 2. BENEFIT Section (Flexible height with independent scroll) */}
      <section class="rounded-2xl border border-white/10 bg-stone-900/70 p-3.5 backdrop-blur-sm flex-1 min-h-0 flex flex-col gap-2 shadow-sm">
        {/* Header */}
        <div class="flex items-center justify-between border-b border-white/5 pb-2 shrink-0">
          <div class="flex items-center gap-2">
            <span
              class={`w-2.5 h-2.5 rounded-full ${
                benefitEval.active.length > 0 ? 'bg-emerald-400 animate-pulse' : 'bg-stone-600'
              }`}
            />
            <span class="text-xs font-mono font-extrabold tracking-wider text-stone-200 uppercase">
              特典会 (BENEFIT)
            </span>
          </div>
          <div class="flex items-center gap-2 text-xs">
            <span class="text-stone-400 font-mono">並行物販エリア</span>
            {benefitEval.active.length > 0 && (
              <span class="font-mono text-emerald-400 font-bold">
                {benefitEval.active.length}組開催中
              </span>
            )}
          </div>
        </div>

        {/* Benefit Active Cards (Safe scrolling with min-h prevention) */}
        <div class="flex-1 overflow-y-auto no-scrollbar flex flex-col gap-2.5 min-h-0 py-0.5">
          {benefitEval.active.length > 0 ? (
            benefitEval.active.map((item, idx) => {
              const theme = getGroupTheme(item.session.groupId || item.session.groupName);

              return (
                <div
                  key={idx}
                  class={`relative rounded-xl border p-3 flex flex-col gap-2 shadow-md shrink-0 ${theme.border} bg-gradient-to-br ${theme.bgGlow}`}
                >
                  {/* Top Line: Time & Location Badge */}
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-mono font-bold text-stone-300 tracking-wider">
                      {item.session.startTime} - {item.session.endTime}
                    </span>

                    {item.session.location && (
                      <div class="flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-amber-400/20 text-amber-300 font-bold border border-amber-400/40 shrink-0">
                        <Icon icon={MapPin} size={11} />
                        <span>{item.session.location}</span>
                      </div>
                    )}
                  </div>

                  {/* Group Name (High visibility) */}
                  <div class="text-base font-extrabold text-white tracking-tight leading-snug break-words">
                    {item.session.groupName}
                  </div>
                </div>
              );
            })
          ) : (
            <div class="flex-1 flex flex-col items-center justify-center py-6 text-center">
              <span class="text-xs text-stone-400 font-mono">
                現在進行中の特典会はありません
              </span>
            </div>
          )}
        </div>

        {/* Benefit NEXT (Footer) */}
        <div class="pt-1.5 border-t border-white/5 flex items-center justify-between text-xs shrink-0">
          <span class="text-[11px] font-mono font-bold text-stone-400 uppercase tracking-wider shrink-0">
            NEXT
          </span>
          {benefitEval.next ? (
            <div class="flex items-center gap-2 text-stone-300 font-mono text-xs truncate ml-2">
              <span class="text-emerald-400 font-bold shrink-0">{benefitEval.next.session.startTime}〜</span>
              <span class="text-white font-bold truncate">{benefitEval.next.session.groupShortName}</span>
              {benefitEval.next.session.location && (
                <span class="text-amber-300 text-xs shrink-0">({benefitEval.next.session.location})</span>
              )}
            </div>
          ) : (
            <span class="text-stone-400 text-xs font-mono">本日の特典会は終了</span>
          )}
        </div>
      </section>
    </div>
  );
}
