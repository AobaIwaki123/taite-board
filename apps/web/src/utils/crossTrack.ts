export interface CrossTrackSession {
  groupId: string;
  groupName: string;
  groupShortName: string;
  themeColor: 'rose' | 'pink';
  type: 'live' | 'benefit';
  label: string;
  startTime: string; // "11:55"
  endTime: string;   // "12:15"
  location?: string; // "並行C" など
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
  remainingMinutes: number; // if active: minutes until end. if upcoming: minutes until start.
  progress: number; // 0 to 1 if active
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

// 2026-11-03 アイドルマリアージュの合同タイテデータ
export const SAMPLE_CROSS_TRACK_EVENTS: Record<string, CrossTrackEvent> = {
  '2026-11-03/mariage': {
    id: '2026-11-03/mariage',
    date: '2026-11-03',
    dayOfWeek: '火祝',
    title: 'アイドルマリアージュ Vol.79',
    venueName: 'ベルサール虎ノ門',
    venueUrl: 'https://maps.google.com/?q=ベルサール虎ノ門',
    ticketUrl: 'https://ticketdive.com/event/idolmariage_79',
    sessions: [
      {
        groupId: 'ribonclock',
        groupName: 'リボンクロック',
        groupShortName: 'リボクロ',
        themeColor: 'rose',
        type: 'live',
        label: 'ライブ出番',
        startTime: '11:55',
        endTime: '12:15',
        detail: '指名入場特典：お目当て3択（10秒動画券 / 写真+30秒トーク 等）',
      },
      {
        groupId: 'ribonclock',
        groupName: 'リボンクロック',
        groupShortName: 'リボクロ',
        themeColor: 'rose',
        type: 'benefit',
        label: '特典会',
        startTime: '12:20',
        endTime: '13:20',
        location: '並行C',
      },
      {
        groupId: 'dollykiss',
        groupName: 'Dolly Kiss',
        groupShortName: 'ドリキス',
        themeColor: 'pink',
        type: 'live',
        label: 'ライブ出番',
        startTime: '14:55',
        endTime: '15:15',
        detail: '指名入場特典：10秒動画券（2本回しで写メ撮り放題券）',
      },
      {
        groupId: 'dollykiss',
        groupName: 'Dolly Kiss',
        groupShortName: 'ドリキス',
        themeColor: 'pink',
        type: 'benefit',
        label: '特典会',
        startTime: '15:20',
        endTime: '16:20',
        location: '並行B',
      },
    ],
  },
};
