import {
  CrossTrackEvent,
  EvaluatedSession,
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

  const renderSection = (
    title: string,
    subtitle: string,
    evaluated: { active: EvaluatedSession[]; next: EvaluatedSession | null }
  ) => {
    const hasActive = evaluated.active.length > 0;

    return (
      <div class="flex-1 flex flex-col gap-2 rounded-2xl border border-white/10 bg-stone-900/60 p-3 backdrop-blur-sm min-h-0">
        {/* Section Header */}
        <div class="flex items-center justify-between border-b border-white/5 pb-1.5 shrink-0">
          <div class="flex items-center gap-2">
            <span
              class={`w-2 h-2 rounded-full ${
                hasActive ? 'bg-emerald-400 animate-pulse' : 'bg-stone-600'
              }`}
            />
            <span class="text-xs font-mono font-bold tracking-wider text-stone-200 uppercase">
              {title}
            </span>
          </div>
          <span class="text-[10px] text-stone-400">{subtitle}</span>
        </div>

        {/* NOW Block */}
        <div class="flex-1 overflow-y-auto no-scrollbar flex flex-col gap-2 py-0.5 min-h-0">
          {hasActive ? (
            evaluated.active.map((activeSession, idx) => {
              const theme = getGroupTheme(activeSession.session.groupId || activeSession.session.groupName);

              return (
                <div
                  key={idx}
                  class={`relative rounded-xl border p-2.5 flex flex-col gap-1 overflow-hidden ${theme.border} bg-gradient-to-br ${theme.bgGlow}`}
                >
                  <div class="flex items-center justify-between">
                    <span class="text-[9px] font-mono font-extrabold uppercase px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/40">
                      NOW ON AIR
                    </span>
                    <span class="text-xs font-mono font-bold text-stone-200">
                      {activeSession.session.startTime} - {activeSession.session.endTime}
                    </span>
                  </div>

                  <div class="text-base font-extrabold text-white tracking-tight mt-0.5">
                    {activeSession.session.groupName}
                  </div>

                  {activeSession.session.location && (
                    <div class="flex items-center gap-1 text-[11px] text-amber-300">
                      <Icon icon={MapPin} size={11} />
                      <span>ブース: {activeSession.session.location}</span>
                    </div>
                  )}

                  {activeSession.session.detail && (
                    <div class="text-[10px] text-stone-300 line-clamp-1">
                      {activeSession.session.detail}
                    </div>
                  )}
                </div>
              );
            })
          ) : (
            <div class="py-4 text-center text-xs text-stone-400 font-mono">
              進行中の演目なし
            </div>
          )}
        </div>

        {/* NEXT Footer Block */}
        <div class="pt-1.5 border-t border-white/5 flex items-center justify-between text-xs shrink-0">
          <span class="text-[10px] font-mono font-bold text-stone-400 uppercase tracking-wider">
            NEXT
          </span>
          {evaluated.next ? (
            <div class="flex items-center gap-2 text-stone-300 font-mono text-[11px] truncate">
              <span class="text-stone-200 font-bold shrink-0">{evaluated.next.session.startTime}〜</span>
              <span class="text-white truncate">
                {evaluated.next.session.groupShortName}
              </span>
              {evaluated.next.session.location && (
                <span class="text-amber-300 text-[10px] shrink-0">({evaluated.next.session.location})</span>
              )}
            </div>
          ) : (
            <span class="text-stone-400 text-[11px] font-mono">予定終了</span>
          )}
        </div>
      </div>
    );
  };

  return (
    <div class="flex-1 flex flex-col gap-2.5 min-h-0 py-2">
      {renderSection('ステージ (STAGE)', 'ライブ本編', liveEval)}
      {renderSection('特典会 (BENEFIT)', '並行物販エリア', benefitEval)}
    </div>
  );
}
