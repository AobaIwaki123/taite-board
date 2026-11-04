import {
  CrossTrackEvent,
  evaluateSessions,
} from '../../utils/crossTrack';

interface Props {
  event: CrossTrackEvent;
  currentMinutes: number;
}

/**
 * 案A1: プロフェッショナル案 (Signboard Clean)
 * 端正で無駄のない左柱（LIVE 01 / 特典会 並行A）＋右スロットのソリッドなミニマル構成。
 */
export function PatternSignboardClean({ event, currentMinutes }: Props) {
  const liveSessions = event.sessions.filter((s) => s.type === 'live');
  const benefitSessions = event.sessions.filter((s) => s.type === 'benefit');

  const liveEval = evaluateSessions(liveSessions, currentMinutes);
  const benefitEval = evaluateSessions(benefitSessions, currentMinutes);

  return (
    <div class="flex-1 flex flex-col gap-3 py-2.5 w-full overflow-y-auto no-scrollbar min-h-0">
      {/* 1. ライブ枠 Section */}
      <section class="flex flex-col gap-1.5 shrink-0">
        <div class="flex items-center justify-between px-1 pb-1 border-b border-stone-800 text-floor font-mono text-stone-400">
          <span class="font-bold tracking-wider uppercase text-stone-300">STAGE / ライブ中</span>
          <span>{liveEval.active.length > 0 ? `${liveEval.active.length} ACT` : '待機中'}</span>
        </div>

        {liveEval.active.length > 0 ? (
          <div class="flex flex-col gap-2 pt-0.5">
            {liveEval.active.map((item, idx) => {
              const progressPercent = Math.round(item.progress * 100);

              return (
                <div
                  key={idx}
                  class="rounded-lg border border-stone-800 bg-stone-900/95 flex items-stretch overflow-hidden shadow-sm shrink-0"
                >
                  {/* 左柱: LIVE 01 */}
                  <div class="w-[52px] shrink-0 bg-stone-950 border-r border-stone-800/80 flex flex-col items-center justify-center py-2.5 px-1 font-mono select-none">
                    <span class="text-[10px] text-stone-400 font-bold tracking-wider leading-none">
                      LIVE
                    </span>
                    <span class="text-meta text-stone-200 font-black tracking-tight leading-none mt-1">
                      {String(idx + 1).padStart(2, '0')}
                    </span>
                  </div>

                  {/* 右スロット情報 */}
                  <div class="flex-1 min-w-0 p-3 flex flex-col justify-between gap-2">
                    {/* Time */}
                    <div class="flex items-center justify-between">
                      <span class="text-meta font-mono font-medium text-stone-400 tracking-wider">
                        {item.session.startTime} — {item.session.endTime}
                      </span>
                    </div>

                    {/* Group Name */}
                    <div class="text-subheading font-bold text-white tracking-tight leading-snug break-words">
                      {item.session.groupName}
                    </div>

                    {/* Clean Progress Slit */}
                    <div class="w-full bg-stone-800/60 rounded-full h-[2px] overflow-hidden mt-0.5">
                      <div
                        class="h-full bg-stone-300 transition-all duration-300"
                        style={{ width: `${progressPercent}%` }}
                      />
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          <div class="py-4 text-center text-meta text-stone-500 font-mono">
            現在進行中のライブはありません
          </div>
        )}
      </section>

      {/* 2. 特典会枠 Section */}
      <section class="flex flex-col gap-1.5 shrink-0 pt-1">
        <div class="flex items-center justify-between px-1 pb-1 border-b border-stone-800 text-floor font-mono text-stone-400">
          <span class="font-bold tracking-wider uppercase text-stone-300">MEET & GREET / 特典会中</span>
          <span>{benefitEval.active.length > 0 ? `${benefitEval.active.length} SESSION` : '待機中'}</span>
        </div>

        {benefitEval.active.length > 0 ? (
          <div class="flex flex-col gap-2 pt-0.5">
            {benefitEval.active.map((item, idx) => {
              const progressPercent = Math.round(item.progress * 100);
              const location = item.session.location || '物販卓';

              return (
                <div
                  key={idx}
                  class="rounded-lg border border-stone-800 bg-stone-900/95 flex items-stretch overflow-hidden shadow-sm shrink-0"
                >
                  {/* 左柱: 特典会 / 並行A */}
                  <div class="w-[52px] shrink-0 bg-stone-950 border-r border-stone-800/80 flex flex-col items-center justify-center py-2 px-1 text-center select-none">
                    <span class="text-[9px] text-stone-400 font-bold tracking-tight leading-none">
                      特典会
                    </span>
                    <span class="text-floor font-mono font-black text-amber-300 tracking-tight leading-tight mt-1 break-words max-w-full">
                      {location}
                    </span>
                  </div>

                  {/* 右スロット情報 */}
                  <div class="flex-1 min-w-0 p-3 flex flex-col justify-between gap-2">
                    {/* Time */}
                    <div class="flex items-center justify-between">
                      <span class="text-meta font-mono font-medium text-stone-400 tracking-wider">
                        {item.session.startTime} — {item.session.endTime}
                      </span>
                    </div>

                    {/* Group Name */}
                    <div class="text-subheading font-bold text-white tracking-tight leading-snug break-words">
                      {item.session.groupName}
                    </div>

                    {/* Clean Progress Slit */}
                    <div class="w-full bg-stone-800/60 rounded-full h-[2px] overflow-hidden mt-0.5">
                      <div
                        class="h-full bg-amber-400/90 transition-all duration-300"
                        style={{ width: `${progressPercent}%` }}
                      />
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          <div class="py-4 text-center text-meta text-stone-500 font-mono">
            現在開催中の特典会はありません
          </div>
        )}
      </section>
    </div>
  );
}
