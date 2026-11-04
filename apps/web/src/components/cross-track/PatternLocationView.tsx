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
    isStage: boolean,
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
          <div class="flex items-center gap-2 text-[10px]">
            <span class="text-stone-300">{subtitle}</span>
            {hasActive && (
              <span class="font-mono text-emerald-400 font-bold">
                {evaluated.active.length}組進行中
              </span>
            )}
          </div>
        </div>

        {/* NOW Block */}
        <div class="flex-1 overflow-y-auto no-scrollbar flex flex-col gap-2 py-0.5 min-h-0">
          {hasActive ? (
            evaluated.active.map((item, idx) => {
              const theme = getGroupTheme(item.session.groupId || item.session.groupName);
              const progressPercent = Math.round(item.progress * 100);

              return (
                <div
                  key={idx}
                  class={`relative rounded-xl border p-3 flex flex-col gap-1.5 overflow-hidden shadow-md ${theme.border} bg-gradient-to-br ${theme.bgGlow}`}
                >
                  {/* Top Line: Session Status, Range & Remaining Minutes */}
                  <div class="flex items-center justify-between">
                    <span class="text-[9px] font-mono font-extrabold uppercase px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/40">
                      NOW ON AIR
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
                  <div class="flex items-baseline justify-between mt-0.5">
                    <div class="text-base font-extrabold text-white tracking-tight">
                      {item.session.groupName}
                    </div>

                    {item.session.location && (
                      <div class="flex items-center gap-1 text-xs text-amber-300 font-medium shrink-0">
                        <Icon icon={MapPin} size={11} />
                        <span>{item.session.location}</span>
                      </div>
                    )}
                  </div>

                  {/* Detail */}
                  {item.session.detail && (
                    <div class="text-[10px] text-stone-300 line-clamp-1 bg-white/5 px-2 py-1 rounded">
                      {item.session.detail}
                    </div>
                  )}

                  {/* Progress Bar */}
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
            <div class="flex-1 flex flex-col items-center justify-center py-4 text-center">
              <span class="text-xs text-stone-300 font-mono">
                {isStage ? '現在ステージの演目はありません' : '現在進行中の特典会はありません'}
              </span>
              {evaluated.next && (
                <span class="text-[11px] text-emerald-400 font-mono mt-1">
                  次の{isStage ? '出番' : '特典会'}まで あと {evaluated.next.remainingMinutes}分
                </span>
              )}
            </div>
          )}
        </div>

        {/* NEXT Footer Block */}
        <div class="pt-1.5 border-t border-white/5 flex items-center justify-between text-xs shrink-0">
          <span class="text-[10px] font-mono font-bold text-stone-300 uppercase tracking-wider">
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
              <span class="text-emerald-400 text-[10px] font-medium shrink-0">
                (あと{evaluated.next.remainingMinutes}分)
              </span>
            </div>
          ) : (
            <span class="text-stone-300 text-[11px] font-mono">予定終了</span>
          )}
        </div>
      </div>
    );
  };

  return (
    <div class="flex-1 flex flex-col gap-2.5 min-h-0 py-2">
      {renderSection('ステージ (STAGE)', 'メインフロア', true, liveEval)}
      {renderSection('特典会 (BENEFIT)', '並行物販エリア', false, benefitEval)}
    </div>
  );
}
