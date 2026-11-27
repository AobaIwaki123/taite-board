import { type SessionExpense, exportAllExpenses, replaceAllExpenses } from './expense.ts';
import { getAttendedSessionIds, setAttendedSessionIds } from './attendance.ts';

const SYNC_SESSION_KEY = 'taite_sync_session';

export type SyncRole = 'user' | 'admin';

export interface SyncSession {
  userId: string;
  role?: SyncRole;
  idToken: string;
  refreshToken: string;
  expiresAt: number; // unix ms
  lastSyncedAt?: string;
}

export interface SyncPayload {
  schema_version: number;
  updated_at?: string;
  expenses: Record<string, SessionExpense>;
  attended: string[];
}

export interface MergeResult {
  merged: SyncPayload;
  localChanged: boolean;
  cloudNeedsPush: boolean;
}

export function getSyncSession(): SyncSession | null {
  if (typeof window === 'undefined' || !window.localStorage) return null;
  try {
    const raw = localStorage.getItem(SYNC_SESSION_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as SyncSession;
    if (!parsed.userId || !parsed.idToken || !parsed.refreshToken) return null;
    return parsed;
  } catch {
    return null;
  }
}

export function saveSyncSession(session: SyncSession): void {
  if (typeof window === 'undefined' || !window.localStorage) return;
  try {
    localStorage.setItem(SYNC_SESSION_KEY, JSON.stringify(session));
  } catch {
    // ignore
  }
}

export function clearSyncSession(): void {
  if (typeof window === 'undefined' || !window.localStorage) return;
  try {
    localStorage.removeItem(SYNC_SESSION_KEY);
  } catch {
    // ignore
  }
}

function parseTimeMs(iso?: string): number {
  if (!iso) return 0;
  const t = Date.parse(iso);
  return Number.isFinite(t) ? t : 0;
}

/**
 * Merges local and cloud expense & attendance payloads using per-session Last-Write-Wins.
 */
export function mergeSyncData(
  localExpenses: Record<string, SessionExpense>,
  cloudExpenses: Record<string, SessionExpense>,
  localAttended: string[],
  cloudAttended: string[],
): MergeResult {
  const mergedExpenses: Record<string, SessionExpense> = {};
  const allKeys = new Set([...Object.keys(localExpenses || {}), ...Object.keys(cloudExpenses || {})]);

  let localChanged = false;
  let cloudNeedsPush = false;

  for (const key of allKeys) {
    const localItem = localExpenses?.[key];
    const cloudItem = cloudExpenses?.[key];

    if (localItem && !cloudItem) {
      mergedExpenses[key] = localItem;
      cloudNeedsPush = true;
    } else if (!localItem && cloudItem) {
      mergedExpenses[key] = cloudItem;
      localChanged = true;
    } else if (localItem && cloudItem) {
      const localTs = parseTimeMs(localItem.updatedAt);
      const cloudTs = parseTimeMs(cloudItem.updatedAt);
      if (cloudTs > localTs) {
        mergedExpenses[key] = cloudItem;
        if (JSON.stringify(localItem) !== JSON.stringify(cloudItem)) {
          localChanged = true;
        }
      } else {
        mergedExpenses[key] = localItem;
        if (localTs > cloudTs || JSON.stringify(localItem) !== JSON.stringify(cloudItem)) {
          cloudNeedsPush = true;
        }
      }
    }
  }

  const mergedAttended = Array.from(new Set([...(cloudAttended || []), ...(localAttended || [])]));
  if (mergedAttended.length !== (localAttended || []).length) {
    localChanged = true;
  }
  if (mergedAttended.length !== (cloudAttended || []).length) {
    cloudNeedsPush = true;
  }

  return {
    merged: {
      schema_version: 1,
      expenses: mergedExpenses,
      attended: mergedAttended,
    },
    localChanged,
    cloudNeedsPush,
  };
}

async function ensureValidSession(session: SyncSession, forceRefresh = false): Promise<SyncSession> {
  if (!forceRefresh && Date.now() + 60_000 < session.expiresAt) {
    return session;
  }
  const res = await fetch('/api/sync/refresh', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: session.refreshToken }),
  });
  const data = await res.json();
  if (!res.ok) {
    if (res.status === 400 || res.status === 401) {
      clearSyncSession();
    }
    throw new Error(data.error || 'セッションの有効期限が切れました');
  }
  const expiresSec = parseInt(String(data.expires_in || '3600'), 10) || 3600;
  const refreshed: SyncSession = {
    ...session,
    idToken: data.id_token,
    refreshToken: data.refresh_token || session.refreshToken,
    expiresAt: Date.now() + expiresSec * 1000,
  };
  saveSyncSession(refreshed);
  return refreshed;
}

export async function logoutSyncSession(): Promise<void> {
  try {
    await fetch('/api/sync/logout', {
      method: 'POST',
      credentials: 'same-origin',
    });
  } catch {
    // ignore network errors on logout
  } finally {
    clearSyncSession();
  }
}

export async function authenticateSyncAccount(
  mode: 'login' | 'register',
  userId: string,
  password: string,
): Promise<SyncSession> {
  const endpoint = mode === 'register' ? '/api/sync/register' : '/api/sync/login';
  const res = await fetch(endpoint, {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ user_id: userId, password }),
  });
  const data = await res.json();
  if (!res.ok) {
    throw new Error(data.error || 'ログインに失敗しました');
  }
  const expiresSec = parseInt(String(data.expires_in || '3600'), 10) || 3600;
  const session: SyncSession = {
    userId: data.user_id,
    role: data.role === 'admin' ? 'admin' : 'user',
    idToken: data.id_token,
    refreshToken: data.refresh_token,
    expiresAt: Date.now() + expiresSec * 1000,
  };
  saveSyncSession(session);
  return session;
}

async function fetchCurrentRole(idToken: string): Promise<SyncRole | null> {
  try {
    const res = await fetch('/api/sync/me', {
      method: 'GET',
      credentials: 'same-origin',
      headers: { Authorization: `Bearer ${idToken}` },
    });
    if (!res.ok) return null;
    const data = await res.json();
    return data.role === 'admin' ? 'admin' : 'user';
  } catch {
    return null;
  }
}

export async function syncWithCloud(): Promise<{
  session: SyncSession;
  attended: string[];
  expenseCount: number;
}> {
  const current = getSyncSession();
  if (!current) {
    throw new Error('ログインしていません');
  }
  let validSession = await ensureValidSession(current);

  let getRes = await fetch('/api/sync/data', {
    method: 'GET',
    credentials: 'same-origin',
    headers: { Authorization: `Bearer ${validSession.idToken}` },
  });
  if (getRes.status === 401) {
    validSession = await ensureValidSession(validSession, true);
    getRes = await fetch('/api/sync/data', {
      method: 'GET',
      credentials: 'same-origin',
      headers: { Authorization: `Bearer ${validSession.idToken}` },
    });
  }
  const cloudRaw = await getRes.json();
  if (!getRes.ok) {
    if (getRes.status === 401) clearSyncSession();
    throw new Error(cloudRaw.error || 'クラウドデータの取得に失敗しました');
  }

  const localExpenses = exportAllExpenses();
  const localAttended = getAttendedSessionIds();

  const { merged, localChanged, cloudNeedsPush } = mergeSyncData(
    localExpenses,
    cloudRaw.expenses || {},
    localAttended,
    cloudRaw.attended || [],
  );

  if (localChanged) {
    replaceAllExpenses(merged.expenses);
    setAttendedSessionIds(merged.attended);
  }

  let syncedAt = cloudRaw.updated_at || new Date().toISOString();
  if (cloudNeedsPush) {
    const putRes = await fetch('/api/sync/data', {
      method: 'PUT',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${validSession.idToken}`,
      },
      body: JSON.stringify(merged),
    });
    const putData = await putRes.json();
    if (!putRes.ok) {
      throw new Error(putData.error || 'クラウドへの保存に失敗しました');
    }
    syncedAt = putData.updated_at || new Date().toISOString();
  }

  const latestRole = await fetchCurrentRole(validSession.idToken);

  const updatedSession: SyncSession = {
    ...validSession,
    role: latestRole ?? validSession.role ?? 'user',
    lastSyncedAt: syncedAt,
  };
  saveSyncSession(updatedSession);

  return {
    session: updatedSession,
    attended: merged.attended,
    expenseCount: Object.keys(merged.expenses).length,
  };
}

export async function pushLocalToCloud(): Promise<SyncSession | null> {
  const current = getSyncSession();
  if (!current) return null;

  let validSession = await ensureValidSession(current);
  const payload: SyncPayload = {
    schema_version: 1,
    expenses: exportAllExpenses(),
    attended: getAttendedSessionIds(),
  };

  let putRes = await fetch('/api/sync/data', {
    method: 'PUT',
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${validSession.idToken}`,
    },
    body: JSON.stringify(payload),
  });
  if (putRes.status === 401) {
    validSession = await ensureValidSession(validSession, true);
    putRes = await fetch('/api/sync/data', {
      method: 'PUT',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${validSession.idToken}`,
      },
      body: JSON.stringify(payload),
    });
  }
  const putData = await putRes.json();
  if (!putRes.ok) {
    if (putRes.status === 401) clearSyncSession();
    throw new Error(putData.error || 'クラウドへの同期に失敗しました');
  }

  const updatedSession: SyncSession = {
    ...validSession,
    lastSyncedAt: putData.updated_at || new Date().toISOString(),
  };
  saveSyncSession(updatedSession);
  return updatedSession;
}

/**
 * Fetches an admin API path (/api/admin/...) attaching Bearer token if available
 * and retrying once after refreshing an expired session.
 */
export async function authorizedAdminFetch(
  subpath: string,
  options: RequestInit = {},
): Promise<Response> {
  const url = subpath.startsWith('/api/admin') ? subpath : `/api/admin${subpath}`;
  let current = getSyncSession();
  if (current) {
    try {
      current = await ensureValidSession(current, false);
    } catch {
      current = getSyncSession();
    }
  }

  const buildHeaders = (token?: string): Headers => {
    const h = new Headers(options.headers);
    if (token) h.set('Authorization', `Bearer ${token}`);
    return h;
  };

  let res = await fetch(url, {
    credentials: 'same-origin',
    ...options,
    headers: buildHeaders(current?.idToken),
  });

  if (res.status === 401 && current) {
    try {
      const refreshed = await ensureValidSession(current, true);
      res = await fetch(url, {
        credentials: 'same-origin',
        ...options,
        headers: buildHeaders(refreshed.idToken),
      });
    } catch {
      // keep initial 401 response for caller handling
    }
  }

  return res;
}

export interface AuthUserInfo {
  user_id: string;
  uid: string;
  role: SyncRole;
}

/**
 * Checks the caller's authentication and RBAC role via /api/sync/me.
 * Returns null if not authenticated (401).
 */
export async function fetchCurrentAuthUser(): Promise<AuthUserInfo | null> {
  let current = getSyncSession();
  if (current) {
    try {
      current = await ensureValidSession(current, false);
    } catch {
      current = getSyncSession();
    }
  }

  const headers: Record<string, string> = {};
  if (current?.idToken) {
    headers.Authorization = `Bearer ${current.idToken}`;
  }

  let res = await fetch('/api/sync/me', {
    credentials: 'same-origin',
    cache: 'no-store',
    headers,
  });

  if (res.status === 401 && current) {
    try {
      const refreshed = await ensureValidSession(current, true);
      res = await fetch('/api/sync/me', {
        credentials: 'same-origin',
        cache: 'no-store',
        headers: { Authorization: `Bearer ${refreshed.idToken}` },
      });
    } catch {
      return null;
    }
  }

  if (!res.ok) return null;
  const info = (await res.json()) as AuthUserInfo;
  if (current && current.role !== info.role) {
    saveSyncSession({ ...current, role: info.role });
  }
  return info;
}
