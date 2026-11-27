import type { ComponentChildren } from 'preact';
import { useCallback, useEffect, useState } from 'preact/hooks';
import {
  fetchCurrentAuthUser,
  getSyncSession,
  type AuthUserInfo,
  type SyncSession,
} from '../utils/sync';
import { SyncAccountModal } from './SyncAccountModal';

interface AdminAuthGateProps {
  title: string;
  children: ComponentChildren;
}

type GateState =
  | { kind: 'checking' }
  | { kind: 'authorized'; user: AuthUserInfo | null }
  | { kind: 'unauthenticated' }
  | { kind: 'forbidden'; user: AuthUserInfo };

export function AdminAuthGate({ title, children }: AdminAuthGateProps) {
  const [state, setState] = useState<GateState>(() =>
    import.meta.env.DEV ? { kind: 'authorized', user: null } : { kind: 'checking' }
  );
  const [session, setSession] = useState<SyncSession | null>(() => getSyncSession());
  const [modalOpen, setModalOpen] = useState(false);

  const checkAccess = useCallback(async () => {
    if (import.meta.env.DEV) {
      setState({ kind: 'authorized', user: null });
      return;
    }
    setState({ kind: 'checking' });
    try {
      const user = await fetchCurrentAuthUser();
      if (!user) {
        setState({ kind: 'unauthenticated' });
        return;
      }
      if (user.role !== 'admin') {
        setState({ kind: 'forbidden', user });
        return;
      }
      setState({ kind: 'authorized', user });
    } catch {
      setState({ kind: 'unauthenticated' });
    }
  }, []);

  useEffect(() => {
    void checkAccess();
  }, [checkAccess]);

  if (state.kind === 'authorized') {
    return <>{children}</>;
  }

  if (state.kind === 'checking') {
    return (
      <div class="h-[100dvh] flex items-center justify-center bg-slate-950 text-slate-300 text-sm">
        管理者権限を確認中...
      </div>
    );
  }

  return (
    <div class="h-[100dvh] flex items-center justify-center bg-slate-950 text-slate-100 p-4">
      <div class="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900 p-6 space-y-4 shadow-2xl">
        <div class="space-y-1">
          <div class="text-xs font-mono text-rose-300">RBAC PROTECTED</div>
          <h1 class="text-lg font-bold text-slate-100">{title}</h1>
          <p class="text-xs text-slate-300 leading-relaxed">
            {state.kind === 'forbidden'
              ? `現在ログイン中のアカウント（${state.user.user_id}）には admin ロール権限がありません。管理者アカウントでログインし直してください。`
              : 'このダッシュボードの閲覧には admin ロール権限を持つアカウントでのログインが必要です。'}
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-2 pt-1">
          <button
            type="button"
            onClick={() => setModalOpen(true)}
            class="px-4 py-2 rounded-xl bg-rose-500 hover:bg-rose-400 text-slate-950 text-xs font-bold cursor-pointer transition-colors"
          >
            {state.kind === 'forbidden' ? 'アカウント切替' : 'ログイン'}
          </button>
          <a
            href="/admin"
            class="px-3 py-2 rounded-xl border border-slate-700 bg-slate-950 text-xs text-slate-200 hover:border-slate-500"
          >
            Admin へ
          </a>
          <a
            href="/"
            class="px-3 py-2 rounded-xl border border-slate-700 bg-slate-950 text-xs text-slate-300 hover:border-slate-500"
          >
            公開スケジュールへ
          </a>
        </div>
      </div>

      <SyncAccountModal
        isOpen={modalOpen}
        session={session}
        syncError={null}
        isSyncing={false}
        onClose={() => setModalOpen(false)}
        onAuthSuccess={(nextSession) => {
          setSession(nextSession);
          setModalOpen(false);
          void checkAccess();
        }}
        onSyncCompleted={(nextSession) => {
          setSession(nextSession);
          void checkAccess();
        }}
        onLoggedOut={() => {
          setSession(null);
          setModalOpen(false);
          void checkAccess();
        }}
      />
    </div>
  );
}
