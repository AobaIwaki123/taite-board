import {
  CrossTrackEvent,
  evaluateSessions,
} from '../../utils/crossTrack';

interface Props {
  event: CrossTrackEvent;
  currentMinutes: number;
}

/**
 * 案A2: ディテール案 (Signboard Detailed)
 * 反転フラップの水平分割ヘアライン、金属ボルトピン、5分刻みタイム目盛り（Ticks）を彫り込んだ偏執的クラフトマンシップ版。
 */
export function PatternSignboardDetailed({ event, currentMinutes }: Props) {
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
                  class="relative rounded-lg border border-stone-800 bg-stone-900/95 flex items-stretch overflow-hidden shadow-sm shrink-0"
                >
                  {/* 反転フラップ（スプリットフラップ）の中央水平分割ヘアライン */}
                  <div class="absolute inset-x-0 top-1/2 -translate-y-1/2 h-[1px] bg-black/90 shadow-[0_1px_0_rgba(255,255,255,0.05)] pointer-events-none z-10" />

                  {/* 左柱: LIVE 01 （ボルトピン留め） */}
                  <div class="w-[52px] shrink-0 bg-stone-950 border-r border-stone-800/80 flex flex-col items-center justify-between py-2 px-1 font-mono select-none relative z-20">
                    {/* 上部ボルトピン */}
                    <div class="w-1.5 h-1.5 rounded-full bg-stone-800 border border-stone-700/80 shadow-inner" />

                    <div class="flex flex-col items-center my-auto">
                      <span class="text-[10px] text-stone-400 font-bold tracking-wider leading-none">
                        LIVE
                      </span>
                      <span class="text-meta text-stone-100 font-black tracking-tight leading-none mt-1">
                        {String(idx + 1).padStart(2, '0')}
                      </span>
                    </div>

                    {/* 下部ボルトピン */}
                    <div class="w-1.5 h-1.5 rounded-full bg-stone-800 border border-stone-700/80 shadow-inner" />
                  </div>

                  {/* 右スロット情報 */}
                  <div class="flex-1 min-w-0 p-3 flex flex-col justify-between gap-2 relative z-20">
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

                    {/* 5分刻みタイム目盛り（Ticks）付きプログレスインジケーター */}
                    <div class="w-full flex flex-col gap-0.5 mt-0.5">
                      {/* 目盛りライン */}
                      <div class="w-full flex items-center justify-between px-0.5 text-stone-700 select-none font-mono text-[7px] leading-none">
                        <span>|</span>
                        <span>·</span>
                        <span>|</span>
                        <span>·</span>
                        <span>|</span>
                        <span>·</span>
                        <span>|</span>
                        <span>·</span>
                        <span>|</span>
                      </div>
                      {/* 進行スリット */}
                      <div class="w-full bg-stone-800/80 rounded-full h-[2px] overflow-hidden relative">
                        <div
                          class="h-full bg-stone-200 transition-all duration-300"
                          style={{ width: `${progressPercent}%` }}
                        />
                      </div>
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
                  class="relative rounded-lg border border-stone-800 bg-stone-900/95 flex items-stretch overflow-hidden shadow-sm shrink-0"
                >
                  {/* 反転フラップの中央水平分割ヘアライン */}
                  <div class="absolute inset-x-0 top-1/2 -translate-y-1/2 h-[1px] bg-black/90 shadow-[0_1px_0_rgba(255,255,255,0.05)] pointer-events-none z-10" />

                  {/* 左柱: 特典会 / 並行A （ボルトピン留め） */}
                  <div class="w-[52px] shrink-0 bg-stone-950 border-r border-stone-800/80 flex flex-col items-center justify-between py-2 px-1 text-center select-none relative z-20">
                    <div class="w-1.5 h-1.5 rounded-full bg-stone-800 border border-stone-700/80 shadow-inner" />

                    <div class="flex flex-col items-center my-auto">
                      <span class="text-[9px] text-stone-400 font-bold tracking-tight leading-none">
                        特典会
                      </span>
                      <span class="text-floor font-mono font-black text-amber-300 tracking-tight leading-tight mt-1 break-words max-w-full">
                        {location}
                      </span>
                    </div>

                    <div class="w-1.5 h-1.5 rounded-full bg-stone-800 border border-stone-700/80 shadow-inner" />
                  </div>

                  {/* 右スロット情報 */}
                  <div class="flex-1 min-w-0 p-3 flex flex-col justify-between gap-2 relative z-20">
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

                    {/* 5分刻みタイム目盛り（Ticks）付きプログレスインジケーター */}
                    <div class="w-full flex flex-col gap-0.5 mt-0.5">
                      <div class="w-full flex items-center justify-between px-0.5 text-stone-700 select-none font-mono text-[7px] leading-none">
                        <span>|</span>
                        <span>·</span>
                        <span>|</span>
                        <span>·</span>
                        <span>|</span>
                        <span>·</span>
                        <span>|</span>
                        <span>·</span>
                        <span>|</span>
                      </div>
                      <div class="w-full bg-stone-800/80 rounded-full h-[2px] overflow-hidden relative">
                        <div
                          class="h-full bg-amber-400/90 transition-all duration-300"
                          style={{ width: `${progressPercent}%` }}
                        />
                      </div>
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
