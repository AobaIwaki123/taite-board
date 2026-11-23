import { useState } from 'preact/hooks';
import { Icon } from './Icon';
import { Cloud, LogIn, LogOut, RefreshCw, SlidersHorizontal, User, X } from '../utils/icons';
import {
  SyncSession,
  authenticateSyncAccount,
  clearSyncSession,
  syncWithCloud,
} from '../utils/sync';

interface SyncAccountModalProps {
  isOpen: boolean;
  session: SyncSession | null;
  syncError: string | null;
  isSyncing: boolean;
  onClose: () => void;
  onAuthSuccess: (session: SyncSession, attended: string[]) => void;
  onSyncCompleted: (session: SyncSession, attended: string[]) => void;
  onLoggedOut: () => void;
}

export function SyncAccountModal({
  isOpen,
  session,
  syncError,
  isSyncing,
  onClose,
  onAuthSuccess,
  onSyncCompleted,
  onLoggedOut,
}: SyncAccountModalProps) {
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [userId, setUserId] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [localError, setLocalError] = useState<string | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setLocalError(null);
    setStatusMessage(null);
    setSubmitting(true);

    try {
      await authenticateSyncAccount(mode, userId, password);
      const synced = await syncWithCloud();
      setPassword('');
      setStatusMessage(`同期完了 (${synced.expenseCount}件の記録を共有)`);
      onAuthSuccess(synced.session, synced.attended);
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : 'ログインに失敗しました');
    } finally {
      setSubmitting(false);
    }
  };

  const handleManualSync = async () => {
    setLocalError(null);
    setStatusMessage(null);
    setSubmitting(true);
    try {
      const synced = await syncWithCloud();
      setStatusMessage(`最新データを同期しました (${synced.expenseCount}件)`);
      onSyncCompleted(synced.session, synced.attended);
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : '同期に失敗しました');
    } finally {
      setSubmitting(false);
    }
  };

  const handleLogout = () => {
    clearSyncSession();
    setStatusMessage(null);
    setLocalError(null);
    onLoggedOut();
  };

  const displayError = localError || syncError;

  return (
    <div
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div class="w-full max-w-sm rounded-2xl bg-stone-900 border border-white/10 shadow-2xl p-4 space-y-4">
        <div class="flex items-center justify-between border-b border-white/10 pb-2.5">
          <div class="flex items-center gap-2 text-sm font-bold text-white">
            <Icon icon={Cloud} class="w-4 h-4 text-sky-400" />
            <span>家計簿・参戦チェック同期</span>
          </div>
          <button
            type="button"
            onClick={onClose}
            class="w-7 h-7 flex items-center justify-center rounded-lg bg-white/5 hover:bg-white/10 text-stone-400 hover:text-white"
            aria-label="閉じる"
          >
            <Icon icon={X} class="w-4 h-4" />
          </button>
        </div>

        {session ? (
          <div class="space-y-3">
            <div class="p-3 rounded-xl bg-stone-950/80 border border-emerald-500/30 space-y-1.5">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2 text-sm font-bold text-white">
                  <Icon icon={User} class="w-4 h-4 text-emerald-400" />
                  <span>{session.userId}</span>
                </div>
                <div class="flex items-center gap-1.5">
                  {session.role === 'admin' && (
                    <span class="px-2 py-0.5 rounded-full bg-rose-500/20 text-rose-300 text-xs font-bold border border-rose-500/30">
                      admin
                    </span>
                  )}
                  <span class="px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 text-xs font-bold">
                    同期有効
                  </span>
                </div>
              </div>
              <div class="text-xs text-stone-400">
                最終同期: {session.lastSyncedAt ? new Date(session.lastSyncedAt).toLocaleString('ja-JP') : '未同期'}
              </div>
            </div>

            {displayError && (
              <div class="p-2.5 rounded-xl bg-rose-500/15 border border-rose-500/30 text-rose-200 text-xs">
                {displayError}
              </div>
            )}

            {statusMessage && (
              <div class="p-2.5 rounded-xl bg-emerald-500/15 border border-emerald-500/30 text-emerald-200 text-xs">
                {statusMessage}
              </div>
            )}

            <div class="flex gap-2">
              <button
                type="button"
                disabled={submitting || isSyncing}
                onClick={handleManualSync}
                class="flex-1 flex items-center justify-center gap-1.5 py-2 px-3 rounded-xl bg-sky-500 hover:bg-sky-400 text-stone-950 font-bold text-xs disabled:opacity-50"
              >
                <Icon icon={RefreshCw} class={`w-3.5 h-3.5 ${submitting || isSyncing ? 'animate-spin' : ''}`} />
                <span>今すぐ同期</span>
              </button>
              <button
                type="button"
                onClick={handleLogout}
                class="flex items-center justify-center gap-1.5 py-2 px-3 rounded-xl bg-stone-800 hover:bg-stone-700 border border-white/10 text-stone-200 font-bold text-xs"
              >
                <Icon icon={LogOut} class="w-3.5 h-3.5" />
                <span>ログアウト</span>
              </button>
            </div>

            {session.role === 'admin' && (
              <a
                href="/admin"
                class="w-full flex items-center justify-center gap-2 py-2.5 px-4 rounded-xl bg-rose-500/15 hover:bg-rose-500/25 border border-rose-500/35 text-rose-200 font-bold text-xs"
              >
                <Icon icon={SlidersHorizontal} class="w-4 h-4" />
                <span>管理ダッシュボードを開く (/admin)</span>
              </a>
            )}

            <p class="text-xs text-stone-400 leading-relaxed">
              別のスマホ・PC で同じユーザーIDとパスワードでログインすると、家計簿と参戦チェックが自動的にマージされます。ログアウトしても端末内の記録は消えません（
              <a
                href="/privacy-policy.html"
                target="_blank"
                rel="noopener noreferrer"
                class="text-sky-400 hover:text-sky-300 underline underline-offset-2"
              >
                プライバシーポリシー
              </a>
              ）。
            </p>
          </div>
        ) : (
          <form onSubmit={handleSubmit} class="space-y-3">
            <div class="grid grid-cols-2 gap-1.5 p-1 rounded-xl bg-stone-950 border border-white/10">
              <button
                type="button"
                onClick={() => {
                  setMode('login');
                  setLocalError(null);
                }}
                class={`py-1.5 rounded-lg text-xs font-bold transition-colors ${
                  mode === 'login' ? 'bg-sky-500 text-stone-950' : 'text-stone-400 hover:text-white'
                }`}
              >
                ログイン
              </button>
              <button
                type="button"
                onClick={() => {
                  setMode('register');
                  setLocalError(null);
                }}
                class={`py-1.5 rounded-lg text-xs font-bold transition-colors ${
                  mode === 'register' ? 'bg-sky-500 text-stone-950' : 'text-stone-400 hover:text-white'
                }`}
              >
                新規作成
              </button>
            </div>

            <div class="space-y-1">
              <label for="sync-user-id" class="block text-xs font-bold text-stone-300">
                ユーザーID
              </label>
              <input
                id="sync-user-id"
                type="text"
                required
                minLength={3}
                maxLength={32}
                pattern="[a-zA-Z0-9._-]{3,32}"
                autoComplete="username"
                placeholder="例: ribbon_01 (半角英数3〜32文字)"
                value={userId}
                onInput={(e) => setUserId((e.target as HTMLInputElement).value)}
                class="w-full px-3 py-2 rounded-xl bg-stone-950 border border-white/15 text-stone-100 text-sm focus:outline-none focus:border-sky-400"
              />
            </div>

            <div class="space-y-1">
              <label for="sync-password" class="block text-xs font-bold text-stone-300">
                パスワード
              </label>
              <input
                id="sync-password"
                type="password"
                required
                minLength={6}
                maxLength={128}
                autoComplete={mode === 'register' ? 'new-password' : 'current-password'}
                placeholder="6文字以上"
                value={password}
                onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
                class="w-full px-3 py-2 rounded-xl bg-stone-950 border border-white/15 text-stone-100 text-sm focus:outline-none focus:border-sky-400"
              />
            </div>

            {displayError && (
              <div class="p-2.5 rounded-xl bg-rose-500/15 border border-rose-500/30 text-rose-200 text-xs">
                {displayError}
              </div>
            )}

            <div class="p-2.5 rounded-xl bg-amber-500/10 border border-amber-400/25 space-y-1.5 text-xs text-stone-300 leading-relaxed">
              <p>
                同期データは運営側で利用・公開しませんが、システム仕様上、管理者からはクラウド保存内容（ユーザーID・家計簿・メモ・参戦チェック）がすべて閲覧可能な状態です。本名や個人情報などは入力しないでください。
              </p>
              <p>
                未ログイン時は端末内のみに保存されます。詳細は{' '}
                <a
                  href="/privacy-policy.html"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-sky-400 hover:text-sky-300 underline underline-offset-2 font-bold"
                >
                  プライバシーポリシー
                </a>{' '}
                をご確認ください。
              </p>
            </div>

            <button
              type="submit"
              disabled={submitting}
              class="w-full flex items-center justify-center gap-2 py-2.5 px-4 rounded-xl bg-sky-500 hover:bg-sky-400 text-stone-950 font-extrabold text-sm disabled:opacity-50"
            >
              <Icon icon={LogIn} class="w-4 h-4" />
              <span>
                {submitting
                  ? '処理中...'
                  : mode === 'register'
                  ? '上記に同意してアカウント作成'
                  : 'ログインして同期する'}
              </span>
            </button>
          </form>
        )}
      </div>
    </div>
  );
}
