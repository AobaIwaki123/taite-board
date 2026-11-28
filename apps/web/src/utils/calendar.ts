import type { Session } from '../types/schedule';

function parseTimeMinutes(raw?: string | null): number | null {
  if (!raw) return null;
  const m = raw.trim().match(/^(\d{1,2}):(\d{2})$/);
  if (!m) return null;
  const hh = parseInt(m[1], 10);
  const mm = parseInt(m[2], 10);
  if (!Number.isFinite(hh) || !Number.isFinite(mm) || hh < 0 || hh > 29 || mm < 0 || mm > 59) {
    return null;
  }
  return hh * 60 + mm;
}

function addDaysToYmd(ymd: string, deltaDays: number): string {
  const [y, m, d] = ymd.split('-').map((v) => parseInt(v, 10));
  const dt = new Date(Date.UTC(y, m - 1, d + deltaDays));
  const yy = String(dt.getUTCFullYear()).padStart(4, '0');
  const mm = String(dt.getUTCMonth() + 1).padStart(2, '0');
  const dd = String(dt.getUTCDate()).padStart(2, '0');
  return `${yy}${mm}${dd}`;
}

function formatDateTimeToken(ymd: string, totalMinutes: number): string {
  const dayOffset = Math.floor(totalMinutes / 1440);
  const minsInDay = totalMinutes % 1440;
  const hh = String(Math.floor(minsInDay / 60)).padStart(2, '0');
  const mm = String(minsInDay % 60).padStart(2, '0');
  return `${addDaysToYmd(ymd, dayOffset)}T${hh}${mm}00`;
}

function formatDatesParam(session: Session): string {
  const ymd = session.date;
  const liveStart = parseTimeMinutes(session.live?.start_time);
  if (liveStart === null) {
    return `${addDaysToYmd(ymd, 0)}/${addDaysToYmd(ymd, 1)}`;
  }

  const liveEnd = parseTimeMinutes(session.live?.end_time);
  const benefitEnd = parseTimeMinutes(session.benefit?.end_time);

  let end = liveStart + 30;
  if (benefitEnd !== null && benefitEnd > liveStart) {
    end = benefitEnd;
  } else if (liveEnd !== null && liveEnd > liveStart) {
    end = liveEnd;
  }

  return `${formatDateTimeToken(ymd, liveStart)}/${formatDateTimeToken(ymd, end)}`;
}

/**
 * Builds a Google Calendar event creation template URL in Asia/Tokyo timezone.
 * Falls back to an all-day event when performance times are TBD.
 */
export function buildGoogleCalendarUrl(session: Session): string {
  const params = new URLSearchParams();
  params.set('action', 'TEMPLATE');
  params.set('text', session.title);
  params.set('dates', formatDatesParam(session));
  params.set('ctz', 'Asia/Tokyo');

  const location = (session.venue?.map_query || session.venue?.name || '').trim();
  if (location) {
    params.set('location', location);
  }

  const detailLines: string[] = [];
  if (parseTimeMinutes(session.live?.start_time) !== null) {
    const end = parseTimeMinutes(session.live?.end_time) !== null ? session.live.end_time : '';
    detailLines.push(`出演: ${session.live.start_time}${end ? ` - ${end}` : ''}`);
  }
  if (parseTimeMinutes(session.benefit?.start_time) !== null) {
    const end = parseTimeMinutes(session.benefit?.end_time) !== null ? session.benefit.end_time : '';
    const booth = session.benefit?.booth ? ` (${session.benefit.booth})` : '';
    detailLines.push(`特典会: ${session.benefit.start_time}${end ? ` - ${end}` : ''}${booth}`);
  }
  if (session.admission_bonus?.trim()) {
    detailLines.push(`入場特典: ${session.admission_bonus.trim()}`);
  }
  if (session.special_notes?.trim()) {
    detailLines.push(`特記事項: ${session.special_notes.trim()}`);
  }
  if (session.ticket?.url?.trim()) {
    detailLines.push(`チケット: ${session.ticket.url.trim()}`);
  }
  const sourceUrl = session.source_tweet_url?.trim() || session.source_refs?.find((r) => r.url)?.url?.trim();
  if (sourceUrl) {
    detailLines.push(`告知: ${sourceUrl}`);
  }

  if (detailLines.length > 0) {
    let details = detailLines.join('\n');
    if (details.length > 1000) {
      details = `${details.slice(0, 997)}...`;
    }
    params.set('details', details);
  }

  return `https://calendar.google.com/calendar/render?${params.toString()}`;
}
