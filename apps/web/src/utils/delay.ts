/**
 * Live Schedule Delay Offset Management (Local-First / Zero Backend)
 * Allows users at the venue to shift timetable timings by +5, +10, +15, +20 mins.
 */

const STORAGE_PREFIX = 'ribonclock_delay_';

export function getSessionDelayOffset(sessionId: string): number {
  try {
    const val = localStorage.getItem(STORAGE_PREFIX + sessionId);
    if (!val) return 0;
    const num = parseInt(val, 10);
    return isNaN(num) ? 0 : num;
  } catch {
    return 0;
  }
}

export function setSessionDelayOffset(sessionId: string, minutes: number): void {
  try {
    if (minutes <= 0) {
      localStorage.removeItem(STORAGE_PREFIX + sessionId);
    } else {
      localStorage.setItem(STORAGE_PREFIX + sessionId, String(minutes));
    }
  } catch {
    // ignore
  }
}
