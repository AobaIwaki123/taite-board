import {
  CrossTrackEvent,
  evaluateSessions,
  getGroupTheme,
} from '../../utils/crossTrack';

interface Props {
  event: CrossTrackEvent;
  currentMinutes: number;
  highlightGroupId?: string;
}

/**
 * 案B: 進行スリップ・タイムシート (All-Access Pass / Schedule Slip)
 * スタッフパスや関係者のタイムシートを思わせる、印刷物のようなタイポグラフィと無駄のない機能美。
 */
export function PatternScheduleSlip({ event, currentMinutes, highlightGroupId }: Props) {
  const liveSessions = event.sessions.filter((s) => s.type === 'live');
  const benefitSessions = event.sessions.filter((s) => s.type === 'benefit');

  const liveEval = evaluateSessions(liveSessions, currentMinutes);
  const benefitEval = evaluateSessions(benefitSessions, currentMinutes);

  return (
    <div class="flex-1 flex flex-col gap-4 py-3 w-full overflow-y-auto no-scrollbar min-h-0 font-sans">
      {/* タイムシート全体を1枚の精密なスリップとして表現 */}
      <div class="rounded-xl border border-stone-800 bg-stone-900/95 overflow-hidden shadow-sm flex flex-col divide-y divide-stone-800">
        {/* 1. STAGE / ライブ枠 */}
        <div class="p-3.5 flex flex-col gap-3">
          {/* Slip Sub-header */}
          <div class="flex items-center justify-between text-floor font-mono text-stone-400 tracking-wider uppercase pb-1 border-b border-dashed border-stone-800/80">
            <span>[ STAGE / ライブ ]</span>
            <span>{liveEval.active.length > 0 ? `${liveEval.active.length} ACT ACTIVE` : 'IDLE'}</span>
          </div>

          {liveEval.active.length > 0 ? (
            <div class="flex flex-col gap-3 pt-1">
              {liveEval.active.map((item, idx) => {
                const theme = getGroupTheme(item.session.groupId || item.session.groupName);
                const progressPercent = Math.round(item.progress * 100);
                const isHighlighted = highlightGroupId && item.session.groupId === highlightGroupId;

                return (
                  <div key={idx} class="flex flex-col gap-1.5 relative">
                    {/* Time & Group Header */}
                    <div class="flex items-baseline justify-between gap-2">
                      <div class="flex items-center gap-2 min-w-0">
                        {/* グループのキーカラーを静かに灯すドット */}
                        <span
                          class={`w-2 h-2 rounded-full shrink-0 ${
                            isHighlighted ? 'bg-amber-400' : theme.bar
                          }`}
                        />
                        <span class="text-subheading font-bold text-stone-100 tracking-tight leading-snug truncate">
                          {item.session.groupName}
                        </span>
                      </div>
                      <span class="text-meta font-mono font-medium text-stone-400 shrink-0">
                        {item.session.startTime} - {item.session.endTime}
                      </span>
                    </div>

                    {/* Progress Rule (細い目盛り罫線) */}
                    <div class="w-full bg-stone-800/50 rounded-full h-[2px] overflow-hidden">
                      <div
                        class={`h-full transition-all duration-300 ${
                          isHighlighted ? 'bg-amber-400' : theme.bar
                        }`}
                        style={{ width: `${progressPercent}%` }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <div class="py-4 text-center text-meta text-stone-500 font-mono">
              STAGE CURRENTLY IDLE
            </div>
          )}
        </div>

        {/* 2. MEET & GREET / 特典会枠 */}
        <div class="p-3.5 flex flex-col gap-3">
          {/* Slip Sub-header */}
          <div class="flex items-center justify-between text-floor font-mono text-stone-400 tracking-wider uppercase pb-1 border-b border-dashed border-stone-800/80">
            <span>[ MEET & GREET / 特典会 ]</span>
            <span>{benefitEval.active.length > 0 ? `${benefitEval.active.length} SESSION ACTIVE` : 'IDLE'}</span>
          </div>

          {benefitEval.active.length > 0 ? (
            <div class="flex flex-col gap-3 pt-1">
              {benefitEval.active.map((item, idx) => {
                const theme = getGroupTheme(item.session.groupId || item.session.groupName);
                const progressPercent = Math.round(item.progress * 100);
                const isHighlighted = highlightGroupId && item.session.groupId === highlightGroupId;

                return (
                  <div key={idx} class="flex flex-col gap-1.5 relative">
                    {/* Time & Group Header */}
                    <div class="flex items-baseline justify-between gap-2">
                      <div class="flex items-center gap-2 min-w-0">
                        <span
                          class={`w-2 h-2 rounded-full shrink-0 ${
                            isHighlighted ? 'bg-amber-400' : theme.bar
                          }`}
                        />
                        <span class="text-subheading font-bold text-stone-100 tracking-tight leading-snug truncate">
                          {item.session.groupName}
                        </span>
                      </div>

                      {/* ブース名と時刻の整然とした並び */}
                      <div class="flex items-center gap-2 shrink-0 font-mono">
                        {item.session.location && (
                          <span class="text-floor font-bold px-1.5 py-0.5 rounded bg-stone-800 text-stone-300 border border-stone-700">
                            {item.session.location}
                          </span>
                        )}
                        <span class="text-meta font-medium text-stone-400">
                          {item.session.startTime} - {item.session.endTime}
                        </span>
                      </div>
                    </div>

                    {/* Progress Rule */}
                    <div class="w-full bg-stone-800/50 rounded-full h-[2px] overflow-hidden">
                      <div
                        class={`h-full transition-all duration-300 ${
                          isHighlighted ? 'bg-amber-400' : theme.bar
                        }`}
                        style={{ width: `${progressPercent}%` }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <div class="py-4 text-center text-meta text-stone-500 font-mono">
              NO ACTIVE SESSIONS
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
