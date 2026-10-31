import { Session } from '../types/schedule';
import { isSessionPast } from '../utils/date';
import { Icon } from './Icon';
import { MapPin } from '../utils/icons';

interface ListViewProps {
  sessions: Session[];
  onSelectSession: (id: string) => void;
}

export function ListView({ sessions, onSelectSession }: ListViewProps) {
  // 特典会終了後数時間の猶予時間を考慮し、今後の公演のみを抽出
  const upcomingSessions = sessions.filter((s) => !isSessionPast(s));

  return (
    <div class="h-full overflow-y-auto no-scrollbar tab-slide space-y-3 pb-6">
      {/* 件数バッジ */}
      <div class="flex items-center justify-between px-1 text-xs text-stone-400">
        <span>今後の公演予定</span>
        <span class="px-2 py-0.5 rounded-full bg-rose-500/10 text-rose-300 border border-rose-500/20 font-bold">
          {upcomingSessions.length}公演
        </span>
      </div>

      {upcomingSessions.length === 0 ? (
        <div class="text-center py-12 px-4 rounded-2xl bg-stone-900/40 border border-white/5 space-y-2">
          <p class="text-sm font-bold text-stone-300">現在予定されている今後の公演はありません</p>
          <p class="text-xs text-stone-500">過去の公演記録は「カレンダー」タブからご覧いただけます</p>
        </div>
      ) : (
        /* 公演カード一覧 */
        <div class="space-y-2.5">
        {upcomingSessions.map((session) => {
          const [, mStr, dStr] = session.date.split('-');
          const monthNum = parseInt(mStr, 10);
          const dayNum = parseInt(dStr, 10);

          let dayColor = 'text-stone-400';
          if (session.day_of_week === '土') dayColor = 'text-blue-400';
          if (session.day_of_week === '日' || (session.day_of_week && session.day_of_week.includes('祝'))) {
            dayColor = 'text-rose-400';
          }

          const sameDayCount = sessions.filter((x) => x.date === session.date).length;
          const sessionTag =
            sameDayCount > 1
              ? session.session_name?.includes('昼')
                ? ' [昼]'
                : ' [夜]'
              : '';

          return (
            <div
              key={session.id}
              onClick={() => onSelectSession(session.id)}
              class="soft-card rounded-2xl p-3 flex items-center gap-3.5 cursor-pointer hover:border-rose-400/40 transition-all shadow-md active:scale-[0.99]"
            >
              {/* カレンダー風・日付アンカーブロック */}
              <div class="w-[56px] h-[60px] rounded-xl bg-stone-950 border border-white/10 flex flex-col items-center justify-center shrink-0 shadow-inner">
                <span class="text-xs font-bold text-stone-400 leading-none">{monthNum}月</span>
                <span class="text-xl font-display font-black text-white leading-tight my-0.5">
                  {dayNum}
                </span>
                <span class={`text-xs font-bold leading-none ${dayColor}`}>
                  {session.day_of_week || ''}
                </span>
              </div>

              {/* イベント情報本体 */}
              <div class="flex-1 min-w-0 space-y-1">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-stone-300 font-semibold truncate flex items-center gap-1">
                    <Icon icon={MapPin} class="w-3 h-3 text-rose-400 shrink-0" />
                    <span class="truncate">{session.venue.name}</span>
                  </span>
                  {sessionTag && (
                    <span class="px-2 py-0.5 rounded bg-rose-500/20 text-rose-300 font-bold text-xs shrink-0">
                      {sessionTag}
                    </span>
                  )}
                </div>

                <div class="text-sm font-extrabold text-white leading-snug line-clamp-2">
                  {session.title}
                </div>

                <div class="text-xs flex items-center justify-between pt-1 border-t border-white/5 text-stone-300">
                  <span class="flex items-center gap-1">
                    <span class="text-rose-400 font-bold">出番</span>
                    <strong class="text-white font-display font-bold">
                      {session.live.start_time}〜
                    </strong>
                  </span>
                  <span class="flex items-center gap-1 text-stone-400">
                    <span>特典会</span>
                    <strong class="text-stone-200 font-display">
                      {session.benefit.start_time}〜
                    </strong>
                  </span>
                </div>
              </div>
            </div>
          );
        })}
      </div>
      )}
    </div>
  );
}
