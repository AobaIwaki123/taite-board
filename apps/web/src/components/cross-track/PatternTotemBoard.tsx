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
 * 案A: インフォメーション・サインボード (Festival Totem / Signboard)
 * 会場ロビーの大型案内板やフライトボードのような、重厚で整然としたスロット構造と機能美。
 */
export function PatternTotemBoard({ event, currentMinutes, highlightGroupId }: Props) {
  const liveSessions = event.sessions.filter((s) => s.type === 'live');
  const benefitSessions = event.sessions.filter((s) => s.type === 'benefit');

  const liveEval = evaluateSessions(liveSessions, currentMinutes);
  const benefitEval = evaluateSessions(benefitSessions, currentMinutes);

  return (
    <div class="flex-1 flex flex-col gap-4 py-3 w-full overflow-y-auto no-scrollbar min-h-0">
      {/* 1. ライブステージ Section */}
      <section class="rounded-xl border border-stone-800 bg-stone-900/95 p-3.5 shrink-0 flex flex-col gap-3 shadow-sm">
        {/* Header: ソリッドな案内板の表題 */}
        <div class="flex items-center justify-between border-b border-stone-800 pb-2 shrink-0">
          <div class="flex items-center gap-2">
            <span
              class={`w-2 h-2 rounded-full ${
                liveEval.active.length > 0 ? 'bg-emerald-400' : 'bg-stone-600'
              }`}
            />
            <h2 class="text-body font-bold tracking-wider text-stone-100 uppercase">
              ライブ中
            </h2>
          </div>
          <span class="text-floor font-mono text-stone-400">
            {liveEval.active.length > 0 ? `${liveEval.active.length}組` : '待機中'}
          </span>
        </div>

        {/* Live Slot Cards */}
        {liveEval.active.length > 0 ? (
          <div class="flex flex-col gap-2">
            {liveEval.active.map((item, idx) => {
              const theme = getGroupTheme(item.session.groupId || item.session.groupName);
              const progressPercent = Math.round(item.progress * 100);
              const isHighlighted = highlightGroupId && item.session.groupId === highlightGroupId;

              return (
                <div
                  key={idx}
                  class={`relative rounded-lg border bg-stone-950/80 px-3.5 pt-3 pb-2.5 flex flex-col gap-2 shrink-0 overflow-hidden transition-colors ${
                    isHighlighted
                      ? 'border-amber-500/60 ring-1 ring-amber-500/30'
                      : 'border-stone-800/80 hover:border-stone-700'
                  }`}
                >
                  {/* Slot Top: Time */}
                  <div class="flex items-center justify-between">
                    <span class="text-meta font-mono font-medium text-stone-400 tracking-wider">
                      {item.session.startTime} — {item.session.endTime}
                    </span>
                    {isHighlighted && (
                      <span class="text-floor font-mono font-bold text-amber-400">
                        MY IDOL
                      </span>
                    )}
                  </div>

                  {/* Group Name (Formal, High legibility) */}
                  <div class="text-subheading font-bold text-stone-100 tracking-tight leading-snug break-words">
                    {item.session.groupName}
                  </div>

                  {/* Bottom Slit Gauge (埋め込み型進行スリット) */}
                  <div class="w-full bg-stone-800/60 rounded-full h-[2px] overflow-hidden mt-1">
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
          <div class="py-5 text-center text-meta text-stone-500 font-mono">
            現在進行中のライブはありません
          </div>
        )}
      </section>

      {/* 2. 特典会・物販 Section */}
      <section class="rounded-xl border border-stone-800 bg-stone-900/95 p-3.5 shrink-0 flex flex-col gap-3 shadow-sm">
        {/* Header */}
        <div class="flex items-center justify-between border-b border-stone-800 pb-2 shrink-0">
          <div class="flex items-center gap-2">
            <span
              class={`w-2 h-2 rounded-full ${
                benefitEval.active.length > 0 ? 'bg-amber-400' : 'bg-stone-600'
              }`}
            />
            <h2 class="text-body font-bold tracking-wider text-stone-100 uppercase">
              特典会中
            </h2>
          </div>
          <span class="text-floor font-mono text-stone-400">
            {benefitEval.active.length > 0 ? `${benefitEval.active.length}組` : '待機中'}
          </span>
        </div>

        {/* Benefit Slot Cards */}
        {benefitEval.active.length > 0 ? (
          <div class="flex flex-col gap-2">
            {benefitEval.active.map((item, idx) => {
              const theme = getGroupTheme(item.session.groupId || item.session.groupName);
              const progressPercent = Math.round(item.progress * 100);
              const isHighlighted = highlightGroupId && item.session.groupId === highlightGroupId;

              return (
                <div
                  key={idx}
                  class={`relative rounded-lg border bg-stone-950/80 px-3.5 pt-3 pb-2.5 flex flex-col gap-2 shrink-0 overflow-hidden transition-colors ${
                    isHighlighted
                      ? 'border-amber-500/60 ring-1 ring-amber-500/30'
                      : 'border-stone-800/80 hover:border-stone-700'
                  }`}
                >
                  {/* Slot Top: Time & Solid Booth Plate */}
                  <div class="flex items-center justify-between">
                    <span class="text-meta font-mono font-medium text-stone-400 tracking-wider">
                      {item.session.startTime} — {item.session.endTime}
                    </span>

                    {/* アクリルプレートを想起させるマットなブース札 */}
                    {item.session.location && (
                      <span class="text-meta font-mono font-bold px-2 py-0.5 rounded bg-stone-800 text-amber-300 border border-stone-700 shrink-0">
                        {item.session.location}
                      </span>
                    )}
                  </div>

                  {/* Group Name */}
                  <div class="text-subheading font-bold text-stone-100 tracking-tight leading-snug break-words">
                    {item.session.groupName}
                  </div>

                  {/* Bottom Slit Gauge */}
                  <div class="w-full bg-stone-800/60 rounded-full h-[2px] overflow-hidden mt-1">
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
          <div class="py-5 text-center text-meta text-stone-500 font-mono">
            現在開催中の特典会はありません
          </div>
        )}
      </section>
    </div>
  );
}
