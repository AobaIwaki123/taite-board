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
    <div class="flex-1 flex flex-col gap-3 py-2 w-full overflow-y-auto no-scrollbar min-h-0">
      {/* 1. ライブ中 Section */}
      <section class="rounded-2xl border border-white/10 bg-stone-900/80 p-4 backdrop-blur-sm shrink-0 flex flex-col gap-3 shadow-md">
        {/* Header */}
        <div class="flex items-center justify-between border-b border-white/10 pb-2 shrink-0">
          <div class="flex items-center gap-2.5">
            <span
              class={`w-2.5 h-2.5 rounded-full ${
                liveEval.active.length > 0 ? 'bg-emerald-400 animate-pulse' : 'bg-stone-600'
              }`}
            />
            <h2 class="text-body font-black tracking-wide text-stone-100">
              ライブ中
            </h2>
          </div>
          {liveEval.active.length > 0 && (
            <span class="text-meta font-mono text-emerald-400 font-bold">
              {liveEval.active.length}組出演中
            </span>
          )}
        </div>

        {/* Live Active Cards */}
        {liveEval.active.length > 0 ? (
          liveEval.active.map((item, idx) => {
            const theme = getGroupTheme(item.session.groupId || item.session.groupName);
            const progressPercent = Math.round(item.progress * 100);

            return (
              <div
                key={idx}
                class={`relative rounded-xl border p-3.5 flex flex-col gap-2 shadow-lg shrink-0 ${theme.border} bg-gradient-to-br ${theme.bgGlow}`}
              >
                {/* Time Range */}
                <div class="flex items-center justify-between">
                  <span class="text-meta font-mono font-bold text-stone-200 tracking-wider">
                    {item.session.startTime} - {item.session.endTime}
                  </span>
                </div>

                {/* Group Name (Formal, Large) */}
                <div class="text-subheading font-black text-white tracking-tight leading-tight break-words">
                  {item.session.groupName}
                </div>

                {/* Progress Bar (あと何分で終わるかを示すバー) */}
                <div class="w-full bg-white/15 rounded-full h-1.5 overflow-hidden mt-1">
                  <div
                    class={`h-full transition-all duration-300 ${theme.bar}`}
                    style={{ width: `${progressPercent}%` }}
                  />
                </div>
              </div>
            );
          })
        ) : (
          <div class="py-6 text-center text-meta text-stone-400 font-mono">
            現在ライブ中のグループはありません
          </div>
        )}
      </section>

      {/* 2. 特典会中 Section */}
      <section class="rounded-2xl border border-white/10 bg-stone-900/80 p-4 backdrop-blur-sm shrink-0 flex flex-col gap-3 shadow-md">
        {/* Header */}
        <div class="flex items-center justify-between border-b border-white/10 pb-2 shrink-0">
          <div class="flex items-center gap-2.5">
            <span
              class={`w-2.5 h-2.5 rounded-full ${
                benefitEval.active.length > 0 ? 'bg-emerald-400 animate-pulse' : 'bg-stone-600'
              }`}
            />
            <h2 class="text-body font-black tracking-wide text-stone-100">
              特典会中
            </h2>
          </div>
          {benefitEval.active.length > 0 && (
            <span class="text-meta font-mono text-emerald-400 font-bold">
              {benefitEval.active.length}組開催中
            </span>
          )}
        </div>

        {/* Benefit Active Cards */}
        {benefitEval.active.length > 0 ? (
          <div class="flex flex-col gap-2.5">
            {benefitEval.active.map((item, idx) => {
              const theme = getGroupTheme(item.session.groupId || item.session.groupName);
              const progressPercent = Math.round(item.progress * 100);

              return (
                <div
                  key={idx}
                  class={`relative rounded-xl border p-3 flex flex-col gap-2 shadow-lg shrink-0 ${theme.border} bg-gradient-to-br ${theme.bgGlow}`}
                >
                  {/* Top Line: Time & Location Badge */}
                  <div class="flex items-center justify-between">
                    <span class="text-meta font-mono font-bold text-stone-200 tracking-wider">
                      {item.session.startTime} - {item.session.endTime}
                    </span>

                    {item.session.location && (
                      <div class="flex items-center gap-1 text-meta px-2.5 py-0.5 rounded-full bg-amber-400/20 text-amber-300 font-bold border border-amber-400/40 shrink-0">
                        <Icon icon={MapPin} size={13} />
                        <span>{item.session.location}</span>
                      </div>
                    )}
                  </div>

                  {/* Group Name (Formal, Large) */}
                  <div class="text-subheading font-black text-white tracking-tight leading-tight break-words">
                    {item.session.groupName}
                  </div>

                  {/* Progress Bar (あと何分で終わるかを示すバー) */}
                  <div class="w-full bg-white/15 rounded-full h-1.5 overflow-hidden mt-1">
                    <div
                      class={`h-full transition-all duration-300 ${theme.bar}`}
                      style={{ width: `${progressPercent}%` }}
                    />
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
