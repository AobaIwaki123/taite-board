export interface CrossTrackSession {
  id?: string;
  groupId?: string;
  groupName: string;
  groupShortName?: string;
  type: 'live' | 'benefit';
  label: string;
  startTime: string; // "11:55"
  endTime: string;   // "12:15"
  location?: string; // "メインステージ", "並行C" など
  detail?: string;
}

export interface CrossTrackEvent {
  id: string;
  date: string;       // "2026-11-03"
  dayOfWeek: string;  // "火祝"
  title: string;      // "アイドルマリアージュ Vol.79"
  venueName: string;  // "ベルサール虎ノ門"
  venueUrl?: string;
  ticketUrl?: string;
  sessions: CrossTrackSession[];
}

export function parseMinutes(timeStr: string): number {
  const [h, m] = timeStr.split(':').map(Number);
  return h * 60 + m;
}

export function formatMinutes(totalMinutes: number): string {
  const h = Math.floor(totalMinutes / 60);
  const m = totalMinutes % 60;
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`;
}

export type SessionTimeStatus = 'active' | 'upcoming' | 'passed';

export interface EvaluatedSession {
  session: CrossTrackSession;
  status: SessionTimeStatus;
  startMinutes: number;
  endMinutes: number;
  durationMinutes: number;
  remainingMinutes: number;
  progress: number;
}

export function evaluateSessions(
  sessions: CrossTrackSession[],
  currentMinutes: number
): {
  active: EvaluatedSession[];
  upcoming: EvaluatedSession[];
  passed: EvaluatedSession[];
  next: EvaluatedSession | null;
} {
  const evaluated: EvaluatedSession[] = sessions.map((session) => {
    const start = parseMinutes(session.startTime);
    const end = parseMinutes(session.endTime);
    const duration = end - start;

    let status: SessionTimeStatus = 'upcoming';
    let remaining = start - currentMinutes;
    let progress = 0;

    if (currentMinutes >= end) {
      status = 'passed';
      remaining = 0;
      progress = 1;
    } else if (currentMinutes >= start) {
      status = 'active';
      remaining = end - currentMinutes;
      progress = duration > 0 ? (currentMinutes - start) / duration : 0;
    }

    return {
      session,
      status,
      startMinutes: start,
      endMinutes: end,
      durationMinutes: duration,
      remainingMinutes: remaining,
      progress: Math.min(1, Math.max(0, progress)),
    };
  });

  const active = evaluated.filter((s) => s.status === 'active');
  const upcoming = evaluated.filter((s) => s.status === 'upcoming');
  const passed = evaluated.filter((s) => s.status === 'passed');
  const next = upcoming.length > 0 ? upcoming[0] : null;

  return { active, upcoming, passed, next };
}

// 汎用グループカラーパレット
export interface GroupTheme {
  border: string;
  bgGlow: string;
  badge: string;
  bar: string;
  accentText: string;
}

const PALETTE: GroupTheme[] = [
  {
    border: 'border-rose-500/50',
    bgGlow: 'from-rose-950/40 via-stone-900/90 to-stone-950',
    badge: 'bg-rose-500/20 text-rose-300 border-rose-500/40',
    bar: 'bg-rose-500',
    accentText: 'text-rose-400',
  },
  {
    border: 'border-pink-500/50',
    bgGlow: 'from-pink-950/40 via-stone-900/90 to-stone-950',
    badge: 'bg-pink-500/20 text-pink-300 border-pink-500/40',
    bar: 'bg-pink-500',
    accentText: 'text-pink-400',
  },
  {
    border: 'border-cyan-500/50',
    bgGlow: 'from-cyan-950/40 via-stone-900/90 to-stone-950',
    badge: 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40',
    bar: 'bg-cyan-500',
    accentText: 'text-cyan-400',
  },
  {
    border: 'border-amber-500/50',
    bgGlow: 'from-amber-950/40 via-stone-900/90 to-stone-950',
    badge: 'bg-amber-500/20 text-amber-300 border-amber-500/40',
    bar: 'bg-amber-500',
    accentText: 'text-amber-400',
  },
  {
    border: 'border-purple-500/50',
    bgGlow: 'from-purple-950/40 via-stone-900/90 to-stone-950',
    badge: 'bg-purple-500/20 text-purple-300 border-purple-500/40',
    bar: 'bg-purple-500',
    accentText: 'text-purple-400',
  },
  {
    border: 'border-emerald-500/50',
    bgGlow: 'from-emerald-950/40 via-stone-900/90 to-stone-950',
    badge: 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40',
    bar: 'bg-emerald-500',
    accentText: 'text-emerald-400',
  },
  {
    border: 'border-blue-500/50',
    bgGlow: 'from-blue-950/40 via-stone-900/90 to-stone-950',
    badge: 'bg-blue-500/20 text-blue-300 border-blue-500/40',
    bar: 'bg-blue-500',
    accentText: 'text-blue-400',
  },
  {
    border: 'border-indigo-500/50',
    bgGlow: 'from-indigo-950/40 via-stone-900/90 to-stone-950',
    badge: 'bg-indigo-500/20 text-indigo-300 border-indigo-500/40',
    bar: 'bg-indigo-500',
    accentText: 'text-indigo-400',
  },
];

export function getGroupTheme(groupIdOrName: string): GroupTheme {
  let hash = 0;
  for (let i = 0; i < groupIdOrName.length; i++) {
    hash = (hash << 5) - hash + groupIdOrName.charCodeAt(i);
    hash |= 0;
  }
  const index = Math.abs(hash) % PALETTE.length;
  return PALETTE[index];
}

// タイムラインの主要な変化点（Step）を自動抽出（正式名称 groupName を使用）
export interface TimelineStep {
  minutes: number;
  timeStr: string;
  description: string;
}

export function extractTimelineSteps(sessions: CrossTrackSession[]): TimelineStep[] {
  const pointsMap = new Map<number, string[]>();

  sessions.forEach((s) => {
    const startM = parseMinutes(s.startTime);
    const endM = parseMinutes(s.endTime);

    const startDesc = `${s.groupName} ${s.label}開始`;
    const endDesc = `${s.groupName} ${s.label}終了`;

    if (!pointsMap.has(startM)) pointsMap.set(startM, []);
    pointsMap.get(startM)!.push(startDesc);

    if (!pointsMap.has(endM)) pointsMap.set(endM, []);
    pointsMap.get(endM)!.push(endDesc);
  });

  const sortedMinutes = Array.from(pointsMap.keys()).sort((a, b) => a - b);

  return sortedMinutes.map((minutes) => ({
    minutes,
    timeStr: formatMinutes(minutes),
    description: pointsMap.get(minutes)!.slice(0, 2).join(' / '),
  }));
}

// 2026-11-03 アイドルマリアージュ Vol.79 全体タイムテーブル（すべて正式名称を使用）
export const SAMPLE_CROSS_TRACK_EVENTS: Record<string, CrossTrackEvent> = {
  '2026-11-03/mariage': {
    id: "2026-11-03/mariage",
    date: "2026-11-03",
    dayOfWeek: "火祝",
    title: "アイドルマリアージュ Vol.79",
    venueName: "ベルサール虎ノ門",
    venueUrl: "https://maps.google.com/?q=ベルサール虎ノ門",
    ticketUrl: "https://ticketdive.com/event/idolmariage_79",
    sessions: [
      {
        groupName: "RUN WITH YOU",
        type: "live",
        label: "ライブ出番",
        startTime: "10:10",
        endTime: "10:30",
      },
      {
        groupName: "RUN WITH YOU",
        type: "benefit",
        label: "特典会",
        startTime: "10:35",
        endTime: "11:35",
        location: "並行A",
      },
      {
        groupName: "YumeLia",
        type: "live",
        label: "ライブ出番",
        startTime: "10:30",
        endTime: "10:50",
      },
      {
        groupName: "YumeLia",
        type: "benefit",
        label: "特典会",
        startTime: "10:55",
        endTime: "11:55",
        location: "並行B",
      },
      {
        groupName: "Bright Charm",
        type: "live",
        label: "ライブ出番",
        startTime: "10:50",
        endTime: "11:10",
      },
      {
        groupName: "Bright Charm",
        type: "benefit",
        label: "特典会",
        startTime: "11:15",
        endTime: "12:15",
        location: "並行C",
      },
      {
        groupName: "BANZAI JAPAN",
        type: "live",
        label: "ライブ出番",
        startTime: "11:10",
        endTime: "11:35",
      },
      {
        groupName: "BANZAI JAPAN",
        type: "benefit",
        label: "特典会",
        startTime: "11:40",
        endTime: "12:40",
        location: "並行A",
      },
      {
        groupName: "MELYUME",
        type: "live",
        label: "ライブ出番",
        startTime: "11:35",
        endTime: "11:55",
      },
      {
        groupName: "MELYUME",
        type: "benefit",
        label: "特典会",
        startTime: "12:00",
        endTime: "13:00",
        location: "並行B",
      },
      {
        groupId: "ribonclock",
        groupName: "リボンクロック",
        type: "live",
        label: "ライブ出番",
        startTime: "11:55",
        endTime: "12:15",
        detail: "指名入場特典：お目当て3択（10秒動画券 / 写真+30秒トーク 等）",
      },
      {
        groupId: "ribonclock",
        groupName: "リボンクロック",
        type: "benefit",
        label: "特典会",
        startTime: "12:20",
        endTime: "13:20",
        location: "並行C",
      },
      {
        groupName: "ゆるっと革命団",
        type: "live",
        label: "ライブ出番",
        startTime: "12:20",
        endTime: "12:40",
      },
      {
        groupName: "ゆるっと革命団",
        type: "benefit",
        label: "特典会",
        startTime: "12:45",
        endTime: "13:45",
        location: "並行A",
      },
      {
        groupName: "きゅん死に一生を!?",
        type: "live",
        label: "ライブ出番",
        startTime: "12:40",
        endTime: "13:00",
      },
      {
        groupName: "きゅん死に一生を!?",
        type: "benefit",
        label: "特典会",
        startTime: "13:05",
        endTime: "14:05",
        location: "並行B",
      },
      {
        groupName: "わくめる！",
        type: "live",
        label: "ライブ出番",
        startTime: "13:00",
        endTime: "13:20",
      },
      {
        groupName: "わくめる！",
        type: "benefit",
        label: "特典会",
        startTime: "13:25",
        endTime: "14:25",
        location: "並行C",
      },
      {
        groupName: "ラプラス",
        type: "live",
        label: "ライブ出番",
        startTime: "13:20",
        endTime: "13:45",
      },
      {
        groupName: "ラプラス",
        type: "benefit",
        label: "特典会",
        startTime: "13:50",
        endTime: "14:50",
        location: "並行A",
      },
      {
        groupName: "マイノリティアラート",
        type: "live",
        label: "ライブ出番",
        startTime: "13:45",
        endTime: "14:05",
      },
      {
        groupName: "マイノリティアラート",
        type: "benefit",
        label: "特典会",
        startTime: "14:10",
        endTime: "15:10",
        location: "並行B",
      },
      {
        groupName: "LEVEL7",
        type: "live",
        label: "ライブ出番",
        startTime: "14:05",
        endTime: "14:30",
      },
      {
        groupName: "LEVEL7",
        type: "benefit",
        label: "特典会",
        startTime: "14:35",
        endTime: "15:35",
        location: "並行C",
      },
      {
        groupName: "カリスマめんざいふっ！",
        type: "live",
        label: "ライブ出番",
        startTime: "14:35",
        endTime: "14:55",
      },
      {
        groupName: "カリスマめんざいふっ！",
        type: "benefit",
        label: "特典会",
        startTime: "15:00",
        endTime: "16:00",
        location: "並行A",
      },
      {
        groupId: "dollykiss",
        groupName: "Dolly Kiss",
        type: "live",
        label: "ライブ出番",
        startTime: "14:55",
        endTime: "15:15",
        detail: "指名入場特典：10秒動画券（2本回しで写メ撮り放題券）",
      },
      {
        groupId: "dollykiss",
        groupName: "Dolly Kiss",
        type: "benefit",
        label: "特典会",
        startTime: "15:20",
        endTime: "16:20",
        location: "並行B",
      },
      {
        groupName: "Loulouchouchou",
        type: "live",
        label: "ライブ出番",
        startTime: "15:15",
        endTime: "15:40",
      },
      {
        groupName: "Loulouchouchou",
        type: "benefit",
        label: "特典会",
        startTime: "15:45",
        endTime: "16:45",
        location: "並行C",
      },
      {
        groupName: "虹色幻想曲〜プリズム・ファンタジア〜",
        type: "live",
        label: "ライブ出番",
        startTime: "15:40",
        endTime: "16:00",
      },
      {
        groupName: "虹色幻想曲〜プリズム・ファンタジア〜",
        type: "benefit",
        label: "特典会",
        startTime: "16:05",
        endTime: "17:05",
        location: "並行A",
      },
      {
        groupName: "Lovlis",
        type: "live",
        label: "ライブ出番",
        startTime: "16:00",
        endTime: "16:25",
      },
      {
        groupName: "Lovlis",
        type: "benefit",
        label: "特典会",
        startTime: "16:30",
        endTime: "17:30",
        location: "並行B",
      },
      {
        groupName: "New Crown",
        type: "live",
        label: "ライブ出番",
        startTime: "16:25",
        endTime: "16:45",
      },
      {
        groupName: "New Crown",
        type: "benefit",
        label: "特典会",
        startTime: "16:50",
        endTime: "17:50",
        location: "並行C",
      },
      {
        groupName: "黎明リフレイン",
        type: "live",
        label: "ライブ出番",
        startTime: "16:50",
        endTime: "17:10",
      },
      {
        groupName: "黎明リフレイン",
        type: "benefit",
        label: "特典会",
        startTime: "17:15",
        endTime: "18:15",
        location: "並行A",
      },
      {
        groupName: "Chime de Happy",
        type: "live",
        label: "ライブ出番",
        startTime: "17:10",
        endTime: "17:30",
      },
      {
        groupName: "Chime de Happy",
        type: "benefit",
        label: "特典会",
        startTime: "17:35",
        endTime: "18:35",
        location: "並行B",
      },
      {
        groupName: "てんしめし！",
        type: "live",
        label: "ライブ出番",
        startTime: "17:30",
        endTime: "17:50",
      },
      {
        groupName: "てんしめし！",
        type: "benefit",
        label: "特典会",
        startTime: "17:55",
        endTime: "18:55",
        location: "並行C",
      },
      {
        groupName: "ブルーなままで",
        type: "live",
        label: "ライブ出番",
        startTime: "17:50",
        endTime: "18:15",
      },
      {
        groupName: "ブルーなままで",
        type: "benefit",
        label: "特典会",
        startTime: "18:20",
        endTime: "19:20",
        location: "並行A",
      },
      {
        groupName: "PHiZZ",
        type: "live",
        label: "ライブ出番",
        startTime: "18:15",
        endTime: "18:35",
      },
      {
        groupName: "PHiZZ",
        type: "benefit",
        label: "特典会",
        startTime: "18:40",
        endTime: "19:40",
        location: "並行B",
      },
      {
        groupName: "折れないしっぽ",
        type: "live",
        label: "ライブ出番",
        startTime: "18:35",
        endTime: "18:55",
      },
      {
        groupName: "折れないしっぽ",
        type: "benefit",
        label: "特典会",
        startTime: "19:00",
        endTime: "20:00",
        location: "並行C",
      },
      {
        groupName: "君のメインヒロイン",
        type: "live",
        label: "ライブ出番",
        startTime: "19:00",
        endTime: "19:25",
      },
      {
        groupName: "君のメインヒロイン",
        type: "benefit",
        label: "特典会",
        startTime: "19:30",
        endTime: "20:30",
        location: "並行A",
      },
      {
        groupName: "IWI!",
        type: "live",
        label: "ライブ出番",
        startTime: "19:25",
        endTime: "19:50",
      },
      {
        groupName: "IWI!",
        type: "benefit",
        label: "特典会",
        startTime: "20:45",
        endTime: "21:45",
        location: "終演後",
      },
      {
        groupName: "Magi",
        type: "live",
        label: "ライブ出番",
        startTime: "19:50",
        endTime: "20:15",
      },
      {
        groupName: "Magi",
        type: "benefit",
        label: "特典会",
        startTime: "20:45",
        endTime: "21:45",
        location: "終演後",
      },
      {
        groupName: "Aerolipop",
        type: "live",
        label: "ライブ出番",
        startTime: "20:15",
        endTime: "20:40",
      },
      {
        groupName: "Aerolipop",
        type: "benefit",
        label: "特典会",
        startTime: "20:45",
        endTime: "21:45",
        location: "終演後",
      },
    ],
  },
};
