import {
  CrossTrackEvent,
  evaluateSessions,
  getGroupTheme,
} from '../../utils/crossTrack';
import { Icon } from '../Icon';
import { MapPin, Sparkles, Mic, Gift } from '../../utils/icons';

interface Props {
  event: CrossTrackEvent;
  currentMinutes: number;
  highlightGroupId?: string;
}

export function PatternLocationView({ event, currentMinutes, highlightGroupId }: Props) {
  const liveSessions = event.sessions.filter((s) => s.type === 'live');
  const benefitSessions = event.sessions.filter((s) => s.type === 'benefit');

  const liveEval = evaluateSessions(liveSessions, currentMinutes);
  const benefitEval = evaluateSessions(benefitSessions, currentMinutes);

  return (
    <div class="flex-1 flex flex-col gap-3.5 py-2.5 w-full overflow-y-auto no-scrollbar min-h-0">
      {/* 1. ライブ中 Section */}
      <section class="rounded-2xl border border-white/10 bg-stone-900/90 p-3.5 backdrop-blur-md shrink-0 flex flex-col gap-3 shadow-lg">
        {/* Section Header */}
        <div class="flex items-center justify-between border-b border-white/10 pb-2 shrink-0">
          <div class="flex items-center gap-2">
            <span class="p-1 rounded-lg bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">
              <Icon icon={Mic} size={14} />
            </span>
            <div class="flex items-baseline gap-1.5">
              <h2 class="text-body font-black tracking-wide text-white">
                ライブ中
              </h2>
              <span class="text-floor font-mono text-stone-400 font-bold uppercase tracking-wider">
                STAGE
              </span>
            </div>
          </div>
          {liveEval.active.length > 0 ? (
            <span class="text-meta font-mono font-bold px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/40 animate-pulse">
              {liveEval.active.length}組出演中
            </span>
          ) : (
            <span class="text-floor font-mono text-stone-400">
              出演待機中
            </span>
          )}
        </div>

        {/* Live Active Cards */}
        {liveEval.active.length > 0 ? (
          <div class="flex flex-col gap-2.5">
            {liveEval.active.map((item, idx) => {
              const theme = getGroupTheme(item.session.groupId || item.session.groupName);
              const progressPercent = Math.round(item.progress * 100);
              const isHighlighted = highlightGroupId && item.session.groupId === highlightGroupId;
              const remaining = item.remainingMinutes;

              return (
                <div
                  key={idx}
                  class={`relative rounded-xl border p-3.5 flex flex-col gap-2.5 shadow-md shrink-0 overflow-hidden transition-all ${
                    isHighlighted
                      ? 'border-amber-400/80 bg-gradient-to-br from-amber-950/40 via-stone-900 to-stone-950 ring-1 ring-amber-400/40 shadow-[0_0_15px_rgba(251,191,36,0.2)]'
                      : `${theme.border} bg-gradient-to-br ${theme.bgGlow}`
                  }`}
                >
                  {/* Cyalume Left Light Strip (サイリウム発光バー) */}
                  <div
                    class={`absolute left-0 top-0 bottom-0 w-1 ${
                      isHighlighted ? 'bg-amber-400' : theme.bar
                    } shadow-[0_0_8px_currentColor]`}
                  />

                  {/* Header Line: Time Range & Remaining Countdown */}
                  <div class="flex items-center justify-between pl-1">
                    <span class="text-meta font-mono font-bold text-stone-200 tracking-wider">
                      {item.session.startTime} - {item.session.endTime}
                    </span>

                    <div class="flex items-center gap-1.5">
                      {isHighlighted && (
                        <span class="flex items-center gap-1 text-floor font-bold px-2 py-0.5 rounded-full bg-amber-400/20 text-amber-300 border border-amber-400/50">
                          <Icon icon={Sparkles} size={11} />
                          <span>推し</span>
                        </span>
                      )}
                      <span
                        class={`text-floor font-mono font-bold px-2 py-0.5 rounded-full border ${
                          remaining <= 5
                            ? 'bg-rose-500/20 text-rose-300 border-rose-500/40 animate-pulse'
                            : 'bg-white/10 text-stone-300 border-white/10'
                        }`}
                      >
                        {remaining <= 1 ? 'まもなく終了' : `残り ${remaining}分`}
                      </span>
                    </div>
                  </div>

                  {/* Group Name (Formal, High-contrast, Leading) */}
                  <div class="pl-1 text-subheading font-black text-white tracking-tight leading-snug break-words">
                    {item.session.groupName}
                  </div>

                  {/* Progress Bar (サイリウムカラー発光ゲージ) */}
                  <div class="pl-1 w-full flex items-center gap-2">
                    <div class="flex-1 bg-white/10 rounded-full h-1.5 overflow-hidden">
                      <div
                        class={`h-full transition-all duration-300 ${
                          isHighlighted ? 'bg-amber-400' : theme.bar
                        } shadow-[0_0_6px_currentColor]`}
                        style={{ width: `${progressPercent}%` }}
                      />
                    </div>
                    <span class="text-floor font-mono text-stone-400 shrink-0">
                      {progressPercent}%
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          <div class="py-6 text-center text-meta text-stone-400 font-mono">
            現在ライブ中のグループはありません
          </div>
        )}
      </section>

      {/* 2. 特典会中 Section */}
      <section class="rounded-2xl border border-white/10 bg-stone-900/90 p-3.5 backdrop-blur-md shrink-0 flex flex-col gap-3 shadow-lg">
        {/* Section Header */}
        <div class="flex items-center justify-between border-b border-white/10 pb-2 shrink-0">
          <div class="flex items-center gap-2">
            <span class="p-1 rounded-lg bg-pink-500/20 text-pink-300 border border-pink-500/30">
              <Icon icon={Gift} size={14} />
            </span>
            <div class="flex items-baseline gap-1.5">
              <h2 class="text-body font-black tracking-wide text-white">
                特典会・物販中
              </h2>
              <span class="text-floor font-mono text-stone-400 font-bold uppercase tracking-wider">
                MEET & GREET
              </span>
            </div>
          </div>
          {benefitEval.active.length > 0 ? (
            <span class="text-meta font-mono font-bold px-2 py-0.5 rounded-full bg-pink-500/20 text-pink-300 border border-pink-500/40 animate-pulse">
              {benefitEval.active.length}組開催中
            </span>
          ) : (
            <span class="text-floor font-mono text-stone-400">
              開催待機中
            </span>
          )}
        </div>

        {/* Benefit Active Cards */}
        {benefitEval.active.length > 0 ? (
          <div class="flex flex-col gap-2.5">
            {benefitEval.active.map((item, idx) => {
              const theme = getGroupTheme(item.session.groupId || item.session.groupName);
              const progressPercent = Math.round(item.progress * 100);
              const isHighlighted = highlightGroupId && item.session.groupId === highlightGroupId;
              const remaining = item.remainingMinutes;

              return (
                <div
                  key={idx}
                  class={`relative rounded-xl border p-3.5 flex flex-col gap-2.5 shadow-md shrink-0 overflow-hidden transition-all ${
                    isHighlighted
                      ? 'border-amber-400/80 bg-gradient-to-br from-amber-950/40 via-stone-900 to-stone-950 ring-1 ring-amber-400/40 shadow-[0_0_15px_rgba(251,191,36,0.2)]'
                      : `${theme.border} bg-gradient-to-br ${theme.bgGlow}`
                  }`}
                >
                  {/* Cyalume Left Light Strip (サイリウム発光バー) */}
                  <div
                    class={`absolute left-0 top-0 bottom-0 w-1 ${
                      isHighlighted ? 'bg-amber-400' : theme.bar
                    } shadow-[0_0_8px_currentColor]`}
                  />

                  {/* Header Line: Time Range & Booth Badge */}
                  <div class="flex items-center justify-between pl-1">
                    <span class="text-meta font-mono font-bold text-stone-200 tracking-wider">
                      {item.session.startTime} - {item.session.endTime}
                    </span>

                    <div class="flex items-center gap-1.5">
                      {isHighlighted && (
                        <span class="flex items-center gap-1 text-floor font-bold px-2 py-0.5 rounded-full bg-amber-400/20 text-amber-300 border border-amber-400/50">
                          <Icon icon={Sparkles} size={11} />
                          <span>推し</span>
                        </span>
                      )}

                      {/* 会場ブース・エリアバッジ（現場オタクの最重要情報） */}
                      {item.session.location && (
                        <div class="flex items-center gap-1 text-meta px-2.5 py-0.5 rounded-full bg-amber-400/20 text-amber-300 font-bold border border-amber-400/40 shrink-0 shadow-sm">
                          <Icon icon={MapPin} size={12} class="text-amber-400" />
                          <span>{item.session.location}</span>
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Group Name & Remaining Counter */}
                  <div class="pl-1 flex items-baseline justify-between gap-2">
                    <div class="text-subheading font-black text-white tracking-tight leading-snug break-words">
                      {item.session.groupName}
                    </div>
                    <span
                      class={`text-floor font-mono font-bold px-2 py-0.5 rounded-full border shrink-0 ${
                        remaining <= 10
                          ? 'bg-rose-500/20 text-rose-300 border-rose-500/40 animate-pulse'
                          : 'bg-white/10 text-stone-300 border-white/10'
                      }`}
                    >
                      {remaining <= 1 ? 'まもなく終了' : `残り ${remaining}分`}
                    </span>
                  </div>

                  {/* Progress Bar */}
                  <div class="pl-1 w-full flex items-center gap-2">
                    <div class="flex-1 bg-white/10 rounded-full h-1.5 overflow-hidden">
                      <div
                        class={`h-full transition-all duration-300 ${
                          isHighlighted ? 'bg-amber-400' : theme.bar
                        } shadow-[0_0_6px_currentColor]`}
                        style={{ width: `${progressPercent}%` }}
                      />
                    </div>
                    <span class="text-floor font-mono text-stone-400 shrink-0">
                      {progressPercent}%
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          <div class="py-6 text-center text-meta text-stone-400 font-mono">
            現在特典会中のグループはありません
          </div>
        )}
      </section>
    </div>
  );
}
