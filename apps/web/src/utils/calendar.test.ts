import test from 'node:test';
import assert from 'node:assert/strict';
import type { Session } from '../types/schedule.ts';
import { buildGoogleCalendarUrl } from './calendar.ts';

function baseSession(overrides: Partial<Session> = {}): Session {
  return {
    id: '20261128-1',
    title: 'テスト対バンライブ',
    date: '2026-11-28',
    venue: { name: 'Spotify O-WEST', map_query: 'Spotify O-WEST 渋谷' },
    live: { start_time: '18:30', end_time: '18:55' },
    benefit: { start_time: '19:10', end_time: '20:10', booth: 'ロビーA' },
    ticket: { platform: 'TicketDive', url: 'https://ticketdive.com/event/example' },
    special_notes: '声出しOK',
    ...overrides,
  };
}

test('buildGoogleCalendarUrl formats JST timed event through benefit end', () => {
  const url = new URL(buildGoogleCalendarUrl(baseSession()));
  assert.equal(url.origin + url.pathname, 'https://calendar.google.com/calendar/render');
  assert.equal(url.searchParams.get('action'), 'TEMPLATE');
  assert.equal(url.searchParams.get('text'), 'テスト対バンライブ');
  assert.equal(url.searchParams.get('dates'), '20261128T183000/20261128T201000');
  assert.equal(url.searchParams.get('ctz'), 'Asia/Tokyo');
  assert.equal(url.searchParams.get('location'), 'Spotify O-WEST 渋谷');
  const details = url.searchParams.get('details') || '';
  assert.match(details, /出演: 18:30 - 18:55/);
  assert.match(details, /特典会: 19:10 - 20:10 \(ロビーA\)/);
  assert.match(details, /特記事項: 声出しOK/);
  assert.match(details, /チケット: https:\/\/ticketdive\.com\/event\/example/);
});

test('buildGoogleCalendarUrl handles late-night rollover past 24:00', () => {
  const url = new URL(
    buildGoogleCalendarUrl(
      baseSession({
        date: '2026-11-30',
        live: { start_time: '23:30', end_time: '24:30' },
        benefit: { start_time: '--:--', end_time: '--:--' },
      })
    )
  );
  assert.equal(url.searchParams.get('dates'), '20261130T233000/20261201T003000');
});

test('buildGoogleCalendarUrl falls back to all-day when times are TBD and omits empty details', () => {
  const url = new URL(
    buildGoogleCalendarUrl(
      baseSession({
        live: { start_time: '--:--', end_time: '--:--' },
        benefit: { start_time: '--:--', end_time: '--:--' },
        special_notes: undefined,
        ticket: undefined,
      })
    )
  );
  assert.equal(url.searchParams.get('dates'), '20261128/20261129');
  assert.equal(url.searchParams.get('details'), null);
});
