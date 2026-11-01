/**
 * User Attendance Tracking (Local-First / Zero Backend)
 * Stores attended session IDs in browser localStorage.
 */

const STORAGE_KEY = 'ribonclock_attended_sessions';

export function getAttendedSessionIds(): string[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export function toggleAttendedSessionId(sessionId: string): string[] {
  try {
    const current = getAttendedSessionIds();
    const exists = current.includes(sessionId);
    const updated = exists ? current.filter((id) => id !== sessionId) : [...current, sessionId];
    localStorage.setItem(STORAGE_KEY, JSON.stringify(updated));
    return updated;
  } catch {
    return getAttendedSessionIds();
  }
}

export function isSessionAttended(sessionId: string, attendedIds?: string[]): boolean {
  const ids = attendedIds || getAttendedSessionIds();
  return ids.includes(sessionId);
}
