import { useEffect, useState } from 'preact/hooks';
import { REGISTERED_GROUPS, GroupConfig, resolveCurrentGroup } from '../config/group';
import {
  SyncSession,
  authorizedAdminFetch,
  getSyncSession,
} from '../utils/sync';
import { setAttendedSessionIds as persistAttendedIds } from '../utils/attendance';
import { Icon } from './Icon';
import { SyncAccountModal } from './SyncAccountModal';
import {
  Calendar,
  Camera,
  Check,
  ChevronDown,
  ChevronUp,
  Clock,
  ExternalLink,
  Gift,
  List,
  MapPin,
  Plus,
  RefreshCw,
  Sparkles,
  User,
  X,
} from '../utils/icons';

type AdminTab = 'intake' | 'sessions';

interface IntakeCandidate {
  title?: string;
  date?: string;
  venue_raw?: string;
  live_start_raw?: string;
  live_end_raw?: string;
  benefit_start_raw?: string;
  benefit_end_raw?: string;
  benefit_booth_raw?: string;
  admission_bonus?: string;
  ticket_url?: string;
  photo_song?: string;
  photo_song_number?: number;
  photo_device?: string;
}

interface IntakeItem {
  source?: string;
  source_id?: string;
  url?: string;
  author?: string;
  text?: string;
  images?: string[];
  needs_ai?: boolean;
  candidate?: IntakeCandidate;
}

interface QueueForm {
  title: string;
  date: string;
  venue: string;
  liveStart: string;
  liveEnd: string;
  benefitStart: string;
  benefitEnd: string;
  booth: string;
  bonus: string;
  ticketUrl: string;
  photoSong: string;
  photoSongNumber: string;
  photoDevice: string;
  notes: string;
  targetSessionId: string;
}

interface QueueEntry {
  qid: number;
  url: string;
  status: 'fetching' | 'ready' | 'committed' | 'error';
  aiRunning: boolean;
  error: string;
  commitError: string;
  committedSessionId?: string;
  committedAction?: 'created' | 'merged';
  detailsOpen: boolean;
  item: IntakeItem | null;
  form: QueueForm;
}

interface AdminSessionSummary {
  id: string;
  date: string;
  title: string;
  venue: string;
  live_start?: string;
  live_end?: string;
  benefit_start?: string;
  benefit_end?: string;
  admission_bonus?: string;
  photo_rule?: string;
}

const ROW_INPUT_CLASS =
  'w-full bg-transparent text-body text-white placeholder-stone-600 focus:outline-none';

const FIELD_GROUP_CLASS =
  'rounded-2xl bg-stone-950/60 border border-white/10 divide-y divide-white/5 overflow-hidden';

const MANAGED_GROUPS: GroupConfig[] = Object.values(REGISTERED_GROUPS).filter(
  (g) => g.id !== 'sample',
);

const TICKET_URL_PATTERN =
  /https?:\/\/(?:[\w-]+\.)?(?:livepocket\.jp|tiget\.net|t-dv\.com|ticketdive\.com|eplus\.jp|ticketvillage\.jp|pia\.jp|l-tike\.com)\/[^\s)\]>」』"']+/i;

function cleanTitleText(raw: string): string {
  return raw
    .replace(/^[🎀✨🎂🤍🕰️\s]+|[🎀✨🎂🤍🕰️\s]+$/g, '')
    .replace(/[『』｢｣]/g, '')
    .trim();
}

function formatTimeRange(start?: string, end?: string): string {
  const s = (start || '').trim();
  const e = (end || '').trim();
  if (!s || s === '--:--') return '--:--';
  if (!e || e === '--:--') return s;
  return `${s}-${e}`;
}

async function readJsonSafe<T = Record<string, any>>(res: Response): Promise<T | null> {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text) as T;
  } catch {
    return null;
  }
}

function deriveFormFromItem(item: IntakeItem): QueueForm {
  const c = item.candidate || {};
  const text = item.text || '';
  const lines = text
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);

  const rawTitle =
    c.title ||
    lines.find(
      (l) =>
        (l.includes('『') || l.includes('｢') || l.includes('Vol') || l.includes('SP')) &&
        !l.includes('撮影') &&
        !l.includes('撮可'),
    ) ||
    lines.find((l) => !l.startsWith('/') && !l.includes('明日のライブ')) ||
    '';

  const fallbackBonus =
    lines.find(
      (l) =>
        !l.includes('特典会') &&
        !l.includes('撮影') &&
        (l.includes('入場特典') || l.includes('お目当て特典') || l.includes('新規特典')),
    ) || '';

  const ticketMatch = text.match(TICKET_URL_PATTERN);

  return {
    title: cleanTitleText(rawTitle),
    date: c.date || '',
    venue: c.venue_raw || '',
    liveStart: c.live_start_raw || '',
    liveEnd: c.live_end_raw || '',
    benefitStart: c.benefit_start_raw || '',
    benefitEnd: c.benefit_end_raw || '',
    booth: c.benefit_booth_raw || '',
    bonus: c.admission_bonus || fallbackBonus,
    ticketUrl: c.ticket_url || (ticketMatch ? ticketMatch[0] : ''),
    photoSong: c.photo_song || '',
    photoSongNumber: c.photo_song_number ? String(c.photo_song_number) : '',
    photoDevice: c.photo_device || '',
    notes: '',
    targetSessionId: '',
  };
}

function emptyForm(): QueueForm {
  return {
    title: '',
    date: '',
    venue: '',
    liveStart: '',
    liveEnd: '',
    benefitStart: '',
    benefitEnd: '',
    booth: '',
    bonus: '',
    ticketUrl: '',
    photoSong: '',
    photoSongNumber: '',
    photoDevice: '',
    notes: '',
    targetSessionId: '',
  };
}

function hasOptionalDetails(f: QueueForm): boolean {
  return Boolean(f.benefitStart || f.benefitEnd || f.booth || f.ticketUrl || f.notes);
}

function GroupedField({
  icon,
  label,
  children,
}: {
  icon?: any;
  label: string;
  children: preact.ComponentChildren;
}) {
  return (
    <div class="px-3.5 py-2.5 focus-within:bg-white/[0.03] transition-colors">
      <label class="flex items-center gap-1.5 text-xs font-bold text-stone-400 mb-1">
        {icon && <Icon icon={icon} class="w-3.5 h-3.5 text-rose-400/80 shrink-0" />}
        <span>{label}</span>
      </label>
      {children}
    </div>
  );
}

/**
 * Mobile-first Admin view built on the main Preact + Tailwind design system.
 */
export function AdminView() {
  const [groupId, setGroupId] = useState<string>(() => {
    const cur = resolveCurrentGroup().id;
    return cur !== 'sample' ? cur : MANAGED_GROUPS[0]?.id || 'ribonclock';
  });
  const [groupMenuOpen, setGroupMenuOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<AdminTab>('intake');
  const [urlsInput, setUrlsInput] = useState('');
  const [queue, setQueue] = useState<QueueEntry[]>([]);
  const [seq, setSeq] = useState(1);
  const [batchError, setBatchError] = useState<string | null>(null);
  const [isFetchingBatch, setIsFetchingBatch] = useState(false);

  const [sessions, setSessions] = useState<AdminSessionSummary[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(false);
  const [reconcileMsg, setReconcileMsg] = useState<string | null>(null);
  const [reconciling, setReconciling] = useState(false);
  const [authMessage, setAuthMessage] = useState<string | null>(null);
  const [syncSession, setSyncSession] = useState<SyncSession | null>(() => getSyncSession());
  const [syncModalOpen, setSyncModalOpen] = useState(false);

  const currentGroup = REGISTERED_GROUPS[groupId] || MANAGED_GROUPS[0];

  const loadSessions = async (targetGroup = groupId) => {
    setSessionsLoading(true);
    try {
      const res = await authorizedAdminFetch(`/sessions?group=${encodeURIComponent(targetGroup)}`);
      if (res.status === 401 || res.status === 403) {
        setAuthMessage(
          res.status === 403
            ? 'ログイン中のアカウントに admin ロール権限がありません。'
            : '管理画面の利用には admin アカウントでのログインが必要です。',
        );
      }
      if (res.ok) {
        const data = await readJsonSafe<any>(res);
        if (data && Array.isArray(data.sessions)) {
          setSessions(data.sessions);
          setAuthMessage(null);
          return;
        }
      }
    } catch {
      // handled below in dev fallback
    } finally {
      setSessionsLoading(false);
    }

    if (import.meta.env.DEV) {
      const grp = REGISTERED_GROUPS[targetGroup] || MANAGED_GROUPS[0];
      try {
        const pubRes = await fetch(grp?.dataUrl || 'schedule.json');
        if (pubRes.ok) {
          const pubData = await pubRes.json();
          const list = Array.isArray(pubData.sessions) ? pubData.sessions : [];
          setSessions(
            list.map((s: any) => ({
              id: s.id,
              date: s.date,
              title: s.title,
              venue: s.venue?.name || '',
              live_start: s.live?.start_time,
              live_end: s.live?.end_time,
              benefit_start: s.benefit?.start_time,
              benefit_end: s.benefit?.end_time,
              admission_bonus: s.admission_bonus,
              photo_rule: s.photo?.rule,
            })),
          );
          setAuthMessage(null);
        }
      } catch {
        // ignore dev fallback error
      }
    }
  };

  useEffect(() => {
    void loadSessions(groupId);
  }, [groupId]);

  const updateEntry = (qid: number, updater: (prev: QueueEntry) => QueueEntry) => {
    setQueue((prev) => prev.map((entry) => (entry.qid === qid ? updater(entry) : entry)));
  };

  const updateField = (qid: number, field: keyof QueueForm, value: string) => {
    updateEntry(qid, (entry) => ({
      ...entry,
      form: { ...entry.form, [field]: value },
    }));
  };

  const runAiParse = async (qid: number, item: IntakeItem) => {
    updateEntry(qid, (e) => ({ ...e, aiRunning: true, commitError: '' }));
    try {
      const res = await authorizedAdminFetch('/intake/ai-parse', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          source_id: item.source_id || '',
          images: item.images || [],
          text: item.text || '',
        }),
      });
      const data = await readJsonSafe<any>(res);
      if (!res.ok) throw new Error(data?.error || `AI解析に失敗しました (HTTP ${res.status})`);

      const c: IntakeCandidate | undefined = data?.candidate;
      if (c) {
        updateEntry(qid, (entry) => {
          const f = { ...entry.form };
          if (c.title && !f.title) f.title = cleanTitleText(c.title);
          if (c.date) f.date = c.date;
          if (c.venue_raw) f.venue = c.venue_raw;
          if (c.live_start_raw) f.liveStart = c.live_start_raw;
          if (c.live_end_raw) f.liveEnd = c.live_end_raw;
          if (c.benefit_start_raw) f.benefitStart = c.benefit_start_raw;
          if (c.benefit_end_raw) f.benefitEnd = c.benefit_end_raw;
          if (c.benefit_booth_raw) f.booth = c.benefit_booth_raw;
          if (c.admission_bonus && !f.bonus) f.bonus = c.admission_bonus;
          if (c.ticket_url && !f.ticketUrl) f.ticketUrl = c.ticket_url;
          if (c.photo_song && !f.photoSong) f.photoSong = c.photo_song;
          if (c.photo_song_number && !f.photoSongNumber) {
            f.photoSongNumber = String(c.photo_song_number);
          }
          if (c.photo_device && !f.photoDevice) f.photoDevice = c.photo_device;
          return {
            ...entry,
            form: f,
            detailsOpen: entry.detailsOpen || hasOptionalDetails(f),
          };
        });
      }
    } catch (err) {
      updateEntry(qid, (e) => ({
        ...e,
        commitError: err instanceof Error ? err.message : 'AI解析エラー',
      }));
    } finally {
      updateEntry(qid, (e) => ({ ...e, aiRunning: false }));
    }
  };

  const handleFetchUrls = async (e: Event) => {
    e.preventDefault();
    setBatchError(null);
    const urls = Array.from(
      new Set(
        urlsInput
          .split(/\r?\n/)
          .map((s) => s.trim())
          .filter(Boolean),
      ),
    );
    if (urls.length === 0) {
      setBatchError('X ポストまたは TimeTree の URL を入力してください。');
      return;
    }

    setIsFetchingBatch(true);
    let nextId = seq;
    const added: QueueEntry[] = urls.map((url) => ({
      qid: nextId++,
      url,
      status: 'fetching',
      aiRunning: false,
      error: '',
      commitError: '',
      detailsOpen: false,
      item: null,
      form: emptyForm(),
    }));
    setSeq(nextId);
    setUrlsInput('');
    setQueue((prev) => [...prev, ...added]);

    for (const entry of added) {
      try {
        const res = await authorizedAdminFetch('/intake', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ url: entry.url }),
        });
        const data = await readJsonSafe<IntakeItem & { error?: string }>(res);
        if (!res.ok || !data) {
          throw new Error(data?.error || `取得に失敗しました (HTTP ${res.status})`);
        }

        const derived = deriveFormFromItem(data);
        updateEntry(entry.qid, (prev) => ({
          ...prev,
          status: 'ready',
          item: data,
          form: derived,
          detailsOpen: hasOptionalDetails(derived),
        }));

        if (data.needs_ai && Array.isArray(data.images) && data.images.length > 0) {
          await runAiParse(entry.qid, data);
        }
      } catch (err) {
        updateEntry(entry.qid, (prev) => ({
          ...prev,
          status: 'error',
          error: err instanceof Error ? err.message : '取得エラー',
        }));
      }
    }

    setIsFetchingBatch(false);
  };

  const handleAddManual = () => {
    const qid = seq;
    setSeq((n) => n + 1);
    setQueue((prev) => [
      ...prev,
      {
        qid,
        url: '',
        status: 'ready',
        aiRunning: false,
        error: '',
        commitError: '',
        detailsOpen: false,
        item: { source: 'manual', source_id: `manual-${Date.now()}`, url: '', text: '', images: [] },
        form: emptyForm(),
      },
    ]);
  };

  const handleLoadDevSample = () => {
    const qid = seq;
    setSeq((n) => n + 1);
    const sampleItem: IntakeItem = {
      source: 'x',
      source_id: '2125167544894198124',
      url: 'https://x.com/Ribbon_Clock/status/2125167544894198124',
      text: '11/28(土) 渋谷RING\n『アイドルライブ SP』\n出演 18:30-18:55 / 特典会 19:00-20:00 (並行A)\n入場特典: 2S写メ券\n1曲目「えすえぬ・えーす」静止画・動画撮影OK！',
      images: ['https://pbs.twimg.com/media/HVi3dC9bsAAinY6?format=jpg&name=large'],
      candidate: {
        title: 'アイドルライブ SP',
        date: '2026-11-28',
        venue_raw: '渋谷RING',
        live_start_raw: '18:30',
        live_end_raw: '18:55',
        benefit_start_raw: '19:00',
        benefit_end_raw: '20:00',
        benefit_booth_raw: '並行A',
        admission_bonus: '2S写メ券',
        photo_song: 'えすえぬ・えーす',
        photo_song_number: 1,
        photo_device: 'video_ok',
      },
    };
    const derived = deriveFormFromItem(sampleItem);
    setQueue((prev) => [
      ...prev,
      {
        qid,
        url: sampleItem.url || '',
        status: 'ready',
        aiRunning: false,
        error: '',
        commitError: '',
        detailsOpen: false,
        item: sampleItem,
        form: derived,
      },
    ]);
  };

  const commitEntry = async (entry: QueueEntry) => {
    const f = entry.form;
    updateEntry(entry.qid, (prev) => ({ ...prev, commitError: '' }));

    try {
      const songNum = parseInt(f.photoSongNumber.trim(), 10) || 0;
      const res = await authorizedAdminFetch('/intake/promote', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          group: groupId,
          target_session_id: f.targetSessionId,
          source_id: entry.item?.source_id || '',
          source_url: entry.item?.url || '',
          author: entry.item?.author || '',
          title: f.title.trim(),
          date: f.date.trim(),
          venue: f.venue.trim(),
          live_start: f.liveStart.trim(),
          live_end: f.liveEnd.trim(),
          benefit_start: f.benefitStart.trim(),
          benefit_end: f.benefitEnd.trim(),
          booth: f.booth.trim(),
          ticket_url: f.ticketUrl.trim(),
          image_url: entry.item?.images?.[0] || '',
          admission_bonus: f.bonus.trim(),
          photo_song: f.photoSong.trim(),
          photo_song_number: songNum,
          photo_device: f.photoDevice,
          special_notes: f.notes.trim(),
        }),
      });
      const data = await readJsonSafe<any>(res);
      if (res.status === 409) {
        updateEntry(entry.qid, (prev) => ({
          ...prev,
          detailsOpen: true,
          commitError: '同日に複数公演があります。詳細項目内の「統合先の明示指定」から対象を選んでください。',
        }));
        return;
      }
      if (!res.ok) throw new Error(data?.error || `反映に失敗しました (HTTP ${res.status})`);

      updateEntry(entry.qid, (prev) => ({
        ...prev,
        status: 'committed',
        committedSessionId: data?.session_id || '',
        committedAction: data?.action === 'merged' ? 'merged' : 'created',
      }));
      void loadSessions(groupId);
    } catch (err) {
      updateEntry(entry.qid, (prev) => ({
        ...prev,
        commitError: err instanceof Error ? err.message : '登録エラー',
      }));
    }
  };

  const handleReconcile = async () => {
    setReconciling(true);
    setReconcileMsg(null);
    try {
      const res = await authorizedAdminFetch('/reconcile', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ group: groupId, dry_run: false }),
      });
      const data = await readJsonSafe<any>(res);
      if (!res.ok) throw new Error(data?.error || `名寄せ実行に失敗しました (HTTP ${res.status})`);
      setReconcileMsg(data?.message || '名寄せを実行しました');
      void loadSessions(groupId);
    } catch (err) {
      setReconcileMsg(err instanceof Error ? err.message : '名寄せに失敗しました');
    } finally {
      setReconciling(false);
    }
  };

  const readyEntries = queue.filter((q) => q.status === 'ready' && !q.aiRunning);

  return (
    <div class="h-[100dvh] overflow-hidden antialiased selection:bg-rose-500/20 flex flex-col bg-stone-950 text-stone-100">
      <div class="mx-auto max-w-sm w-full px-3.5 pt-3 flex flex-col h-full min-h-0 relative">
        {/* ヘッダー: トグルグループ選択 ＋ 公開へ */}
        <header class="flex-shrink-0 flex items-center justify-between gap-2 pb-2.5 border-b border-white/5 relative z-30">
          <div class="relative min-w-0">
            <button
              type="button"
              onClick={() => setGroupMenuOpen((o) => !o)}
              aria-expanded={groupMenuOpen}
              class="flex items-center gap-1.5 px-2.5 h-8 rounded-xl bg-stone-900 hover:bg-stone-800 border border-white/10 text-sm font-bold text-white transition-colors max-w-full"
            >
              <span class="text-base shrink-0">{currentGroup?.logoText}</span>
              <span class="truncate">{currentGroup?.name}</span>
              <Icon
                icon={groupMenuOpen ? ChevronUp : ChevronDown}
                class="w-3.5 h-3.5 text-stone-400 shrink-0"
              />
            </button>

            {groupMenuOpen && (
              <>
                <div class="fixed inset-0 z-20" onClick={() => setGroupMenuOpen(false)} />
                <div class="absolute left-0 top-10 z-30 w-56 bg-stone-900 rounded-2xl p-1.5 shadow-2xl border border-white/15 space-y-1">
                  {MANAGED_GROUPS.map((g) => {
                    const active = g.id === groupId;
                    return (
                      <button
                        key={g.id}
                        type="button"
                        onClick={() => {
                          setGroupId(g.id);
                          setGroupMenuOpen(false);
                        }}
                        class={`w-full flex items-center justify-between gap-2 px-3 py-2 rounded-xl text-xs font-bold transition-colors ${
                          active
                            ? 'bg-rose-500/20 text-rose-200 border border-rose-500/35'
                            : 'text-stone-300 hover:bg-white/5 border border-transparent'
                        }`}
                      >
                        <span class="flex items-center gap-2 truncate">
                          <span>{g.logoText}</span>
                          <span class="truncate">{g.name}</span>
                        </span>
                        {active && <Icon icon={Check} class="w-3.5 h-3.5 text-rose-300 shrink-0" />}
                      </button>
                    );
                  })}
                </div>
              </>
            )}
          </div>

          <div class="flex items-center gap-1.5 shrink-0">
            <button
              type="button"
              onClick={() => setSyncModalOpen(true)}
              class="flex items-center justify-center w-8 h-8 rounded-xl bg-white/5 hover:bg-white/10 border border-white/5 text-stone-400 hover:text-white transition-colors"
              title="アカウント"
              aria-label="アカウント"
            >
              <Icon icon={User} class="w-3.5 h-3.5" />
            </button>
            <a
              href="/"
              class="flex items-center gap-1 px-2.5 h-8 rounded-xl bg-white/5 hover:bg-white/10 border border-white/5 text-xs font-bold text-stone-300 hover:text-white whitespace-nowrap transition-colors"
            >
              <span>公開へ</span>
            </a>
          </div>
        </header>

        {/* メインコンテンツ */}
        <main class="flex-1 min-h-0 overflow-y-auto no-scrollbar tab-slide pt-3 pb-6 space-y-3">
          {authMessage && !import.meta.env.DEV && (
            <div class="soft-card rounded-2xl p-3.5 border border-rose-500/35 flex items-center justify-between gap-3">
              <p class="text-xs text-rose-200 leading-relaxed">{authMessage}</p>
              <button
                type="button"
                onClick={() => setSyncModalOpen(true)}
                class="px-3 py-1.5 rounded-xl bg-rose-500 text-stone-950 text-xs font-bold shrink-0"
              >
                ログイン
              </button>
            </div>
          )}

          {activeTab === 'intake' ? (
            <>
              {/* URL 取込 */}
              <form
                onSubmit={handleFetchUrls}
                class="soft-card rounded-2xl p-3.5 space-y-3 shadow-lg shadow-black/30"
              >
                <div class="flex items-center justify-between">
                  <span class="flex items-center gap-1.5 text-xs font-bold text-rose-400">
                    <Icon icon={Sparkles} class="w-3.5 h-3.5" />
                    <span>告知リンクから取り込む</span>
                  </span>
                </div>

                <textarea
                  value={urlsInput}
                  onInput={(e) => setUrlsInput((e.target as HTMLTextAreaElement).value)}
                  rows={2}
                  placeholder="https://x.com/... または TimeTree URL"
                  class="w-full px-3.5 py-2.5 rounded-xl bg-stone-950/75 border border-white/10 text-body text-stone-100 placeholder-stone-500 focus:outline-none focus:border-rose-400/60 transition-colors resize-y min-h-[76px]"
                />

                {batchError && <p class="text-xs text-rose-300">{batchError}</p>}

                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={handleAddManual}
                    class="flex items-center justify-center gap-1 px-3 py-2.5 rounded-xl bg-white/5 hover:bg-white/10 border border-white/10 text-xs font-bold text-stone-300 whitespace-nowrap transition-colors"
                  >
                    <Icon icon={Plus} class="w-3.5 h-3.5" />
                    <span>手入力</span>
                  </button>
                  <button
                    type="submit"
                    disabled={isFetchingBatch}
                    class="flex-1 py-2.5 px-4 rounded-xl bg-gradient-to-r from-rose-500 to-rose-400 text-stone-950 font-bold text-sm shadow-md shadow-rose-500/20 disabled:opacity-50 transition-opacity"
                  >
                    {isFetchingBatch ? '取得中...' : '取得して解析'}
                  </button>
                </div>
              </form>

              {queue.length === 0 && (
                <div class="soft-card rounded-2xl p-5 text-center space-y-3 bg-gradient-to-b from-stone-900/70 to-stone-950/80">
                  <p class="text-xs text-stone-400 leading-relaxed">
                    X ポストまたは TimeTree の URL を貼り付けると、日時・会場・特典・撮可情報を抽出して編集カードを開きます。
                  </p>
                  {import.meta.env.DEV && (
                    <button
                      type="button"
                      onClick={handleLoadDevSample}
                      class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-white/5 hover:bg-white/10 border border-white/10 text-xs font-bold text-stone-300"
                    >
                      <Icon icon={Plus} class="w-3.5 h-3.5 text-rose-400" />
                      <span>サンプル候補で表示確認 (dev)</span>
                    </button>
                  )}
                </div>
              )}

              {/* 複数件時のみ表示 */}
              {queue.length >= 2 && (
                <div class="flex items-center justify-between gap-2 px-1">
                  <span class="text-xs text-stone-400">
                    全{queue.length}件（反映待ち {readyEntries.length}）
                  </span>
                  <div class="flex items-center gap-2">
                    {readyEntries.length > 0 && (
                      <button
                        type="button"
                        onClick={() => readyEntries.forEach((item) => void commitEntry(item))}
                        class="px-3 py-1.5 rounded-xl bg-emerald-500/20 border border-emerald-500/40 text-emerald-300 text-xs font-bold"
                      >
                        まとめて反映 ({readyEntries.length})
                      </button>
                    )}
                    <button
                      type="button"
                      onClick={() => setQueue([])}
                      class="px-2.5 py-1.5 rounded-xl bg-white/5 text-xs text-stone-400 hover:text-white"
                    >
                      クリア
                    </button>
                  </div>
                </div>
              )}

              {/* キューカード */}
              {queue.map((entry) => {
                if (entry.status === 'fetching') {
                  return (
                    <div
                      key={entry.qid}
                      class="soft-card rounded-2xl p-3.5 flex items-center gap-2 text-xs text-sky-300"
                    >
                      <Icon icon={RefreshCw} class="w-3.5 h-3.5 animate-spin shrink-0" />
                      <span class="truncate">{entry.url}</span>
                    </div>
                  );
                }

                if (entry.status === 'error') {
                  return (
                    <div
                      key={entry.qid}
                      class="soft-card rounded-2xl p-3.5 border border-rose-500/40 space-y-2"
                    >
                      <div class="flex items-center justify-between gap-2">
                        <span class="text-xs font-bold text-rose-300">取得失敗</span>
                        <button
                          type="button"
                          onClick={() => setQueue((q) => q.filter((x) => x.qid !== entry.qid))}
                          class="p-1 text-stone-400 hover:text-white"
                          aria-label="閉じる"
                        >
                          <Icon icon={X} class="w-4 h-4" />
                        </button>
                      </div>
                      <p class="text-xs text-rose-200 break-words">{entry.error}</p>
                    </div>
                  );
                }

                if (entry.status === 'committed') {
                  return (
                    <div
                      key={entry.qid}
                      class="soft-card rounded-2xl p-3.5 border border-emerald-500/40 flex items-center justify-between gap-2"
                    >
                      <div class="min-w-0">
                        <div class="flex items-center gap-1.5 text-xs font-bold text-emerald-300">
                          <Icon icon={Check} class="w-3.5 h-3.5 shrink-0" />
                          <span>{entry.committedAction === 'merged' ? '統合完了' : '新規反映完了'}</span>
                          {entry.committedSessionId && (
                            <span class="text-stone-400">{entry.committedSessionId}</span>
                          )}
                        </div>
                        <p class="text-sm font-bold text-white truncate mt-0.5">
                          {entry.form.title || entry.form.date}
                        </p>
                      </div>
                      <button
                        type="button"
                        onClick={() => setQueue((q) => q.filter((x) => x.qid !== entry.qid))}
                        class="p-1.5 rounded-lg bg-white/5 text-stone-300 hover:text-white"
                        aria-label="閉じる"
                      >
                        <Icon icon={X} class="w-4 h-4" />
                      </button>
                    </div>
                  );
                }

                const f = entry.form;
                const images = entry.item?.images || [];

                return (
                  <div
                    key={entry.qid}
                    class="soft-card rounded-2xl p-3.5 space-y-3 shadow-lg shadow-black/30"
                  >
                    {/* カード上端: 最小限の操作 */}
                    <div class="flex items-center justify-between gap-2 pb-2 border-b border-white/5">
                      <div class="flex items-center gap-2 min-w-0">
                        {entry.aiRunning && (
                          <span class="flex items-center gap-1 text-xs font-bold text-amber-300">
                            <Icon icon={RefreshCw} class="w-3.5 h-3.5 animate-spin" />
                            <span>AI解析中</span>
                          </span>
                        )}
                        {entry.url && (
                          <a
                            href={entry.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            class="inline-flex items-center gap-1 text-xs text-sky-400 hover:underline truncate"
                          >
                            <span>元投稿を開く</span>
                            <Icon icon={ExternalLink} class="w-3 h-3 shrink-0" />
                          </a>
                        )}
                      </div>
                      <div class="flex items-center gap-1.5 shrink-0">
                        {images.length > 0 && entry.item && (
                          <button
                            type="button"
                            disabled={entry.aiRunning}
                            onClick={() => void runAiParse(entry.qid, entry.item!)}
                            class="px-2 py-1 rounded-lg bg-amber-500/10 border border-amber-500/25 text-xs font-bold text-amber-300 disabled:opacity-50"
                          >
                            AI再解析
                          </button>
                        )}
                        <button
                          type="button"
                          onClick={() => setQueue((q) => q.filter((x) => x.qid !== entry.qid))}
                          class="p-1 rounded-lg text-stone-400 hover:text-white hover:bg-white/5"
                          aria-label="除外"
                        >
                          <Icon icon={X} class="w-4 h-4" />
                        </button>
                      </div>
                    </div>

                    {/* 添付画像 & 原文 */}
                    {(images.length > 0 || entry.item?.text) && (
                      <div class="space-y-2">
                        {images.length > 0 && (
                          <div class="flex gap-2 overflow-x-auto no-scrollbar">
                            {images.map((img) => (
                              <a
                                key={img}
                                href={img}
                                target="_blank"
                                rel="noopener noreferrer"
                                class="shrink-0"
                              >
                                <img
                                  src={img}
                                  alt="添付画像"
                                  class="w-20 h-20 object-cover rounded-xl border border-white/10"
                                />
                              </a>
                            ))}
                          </div>
                        )}
                        {entry.item?.text && (
                          <details class="rounded-xl bg-stone-950/50 border border-white/5 px-3 py-2">
                            <summary class="text-xs font-bold text-stone-400 cursor-pointer select-none">
                              投稿原文を見る
                            </summary>
                            <p class="mt-2 text-xs text-stone-300 whitespace-pre-wrap leading-relaxed">
                              {entry.item.text}
                            </p>
                          </details>
                        )}
                      </div>
                    )}

                    {/* グループ化インセットフォーム（原則縦 1 列） */}
                    <div class="space-y-3">
                      {/* グループ 1: 基本 */}
                      <div class={FIELD_GROUP_CLASS}>
                        <GroupedField icon={Calendar} label="日付">
                          <input
                            type="date"
                            value={f.date}
                            onInput={(e) =>
                              updateField(entry.qid, 'date', (e.target as HTMLInputElement).value)
                            }
                            class={ROW_INPUT_CLASS}
                          />
                        </GroupedField>
                        <GroupedField icon={Sparkles} label="イベントタイトル">
                          <input
                            type="text"
                            value={f.title}
                            onInput={(e) =>
                              updateField(entry.qid, 'title', (e.target as HTMLInputElement).value)
                            }
                            placeholder="補遺のみの場合は空欄可"
                            class={ROW_INPUT_CLASS}
                          />
                        </GroupedField>
                        <GroupedField icon={MapPin} label="会場">
                          <input
                            type="text"
                            value={f.venue}
                            onInput={(e) =>
                              updateField(entry.qid, 'venue', (e.target as HTMLInputElement).value)
                            }
                            placeholder="例: 渋谷RING"
                            class={ROW_INPUT_CLASS}
                          />
                        </GroupedField>
                      </div>

                      {/* グループ 2: 時間・特典 */}
                      <div class={FIELD_GROUP_CLASS}>
                        <GroupedField icon={Clock} label="出番 開始">
                          <input
                            type="text"
                            inputMode="numeric"
                            value={f.liveStart}
                            onInput={(e) =>
                              updateField(entry.qid, 'liveStart', (e.target as HTMLInputElement).value)
                            }
                            placeholder="18:30"
                            class={ROW_INPUT_CLASS}
                          />
                        </GroupedField>
                        <GroupedField icon={Clock} label="出番 終了">
                          <input
                            type="text"
                            inputMode="numeric"
                            value={f.liveEnd}
                            onInput={(e) =>
                              updateField(entry.qid, 'liveEnd', (e.target as HTMLInputElement).value)
                            }
                            placeholder="18:55"
                            class={ROW_INPUT_CLASS}
                          />
                        </GroupedField>
                        <GroupedField icon={Gift} label="入場特典">
                          <input
                            type="text"
                            value={f.bonus}
                            onInput={(e) =>
                              updateField(entry.qid, 'bonus', (e.target as HTMLInputElement).value)
                            }
                            placeholder="例: 2S写メ券"
                            class={ROW_INPUT_CLASS}
                          />
                        </GroupedField>
                      </div>

                      {/* グループ 3: 撮可 */}
                      <div class={FIELD_GROUP_CLASS}>
                        <GroupedField icon={Camera} label="撮影可能曲">
                          <input
                            type="text"
                            value={f.photoSong}
                            onInput={(e) =>
                              updateField(entry.qid, 'photoSong', (e.target as HTMLInputElement).value)
                            }
                            placeholder="曲名"
                            class={ROW_INPUT_CLASS}
                          />
                        </GroupedField>

                        {/* 曲番号のように確実に短い項目のみ横並びを許容 */}
                        <div class="grid grid-cols-[96px_minmax(0,1fr)] divide-x divide-white/5">
                          <GroupedField label="曲番号">
                            <input
                              type="number"
                              min="1"
                              step="1"
                              inputMode="numeric"
                              value={f.photoSongNumber}
                              onInput={(e) =>
                                updateField(
                                  entry.qid,
                                  'photoSongNumber',
                                  (e.target as HTMLInputElement).value,
                                )
                              }
                              placeholder="1"
                              class={ROW_INPUT_CLASS}
                            />
                          </GroupedField>
                          <GroupedField label="撮影種別">
                            <select
                              value={f.photoDevice}
                              onChange={(e) =>
                                updateField(
                                  entry.qid,
                                  'photoDevice',
                                  (e.target as HTMLSelectElement).value,
                                )
                              }
                              class={ROW_INPUT_CLASS}
                            >
                              <option value="" class="bg-stone-900">指定なし</option>
                              <option value="photo_ok" class="bg-stone-900">撮影OK</option>
                              <option value="still_only" class="bg-stone-900">静止画のみ</option>
                              <option value="video_ok" class="bg-stone-900">動画・静止画OK</option>
                            </select>
                          </GroupedField>
                        </div>
                      </div>

                      {/* 低頻度・任意項目の折りたたみ */}
                      <div>
                        <button
                          type="button"
                          onClick={() =>
                            updateEntry(entry.qid, (prev) => ({
                              ...prev,
                              detailsOpen: !prev.detailsOpen,
                            }))
                          }
                          class="flex items-center justify-between w-full px-1 py-1.5 text-xs font-bold text-stone-400 hover:text-stone-200"
                        >
                          <span>詳細項目（特典会・チケット・メモ）</span>
                          <Icon
                            icon={entry.detailsOpen ? ChevronUp : ChevronDown}
                            class="w-4 h-4"
                          />
                        </button>

                        {entry.detailsOpen && (
                          <div class={`${FIELD_GROUP_CLASS} mt-1.5`}>
                            <GroupedField label="特典会 開始">
                              <input
                                type="text"
                                inputMode="numeric"
                                value={f.benefitStart}
                                onInput={(e) =>
                                  updateField(
                                    entry.qid,
                                    'benefitStart',
                                    (e.target as HTMLInputElement).value,
                                  )
                                }
                                placeholder="19:00"
                                class={ROW_INPUT_CLASS}
                              />
                            </GroupedField>
                            <GroupedField label="特典会 終了">
                              <input
                                type="text"
                                inputMode="numeric"
                                value={f.benefitEnd}
                                onInput={(e) =>
                                  updateField(
                                    entry.qid,
                                    'benefitEnd',
                                    (e.target as HTMLInputElement).value,
                                  )
                                }
                                placeholder="20:00"
                                class={ROW_INPUT_CLASS}
                              />
                            </GroupedField>
                            <GroupedField label="特典会ブース">
                              <input
                                type="text"
                                value={f.booth}
                                onInput={(e) =>
                                  updateField(entry.qid, 'booth', (e.target as HTMLInputElement).value)
                                }
                                placeholder="例: 並行A"
                                class={ROW_INPUT_CLASS}
                              />
                            </GroupedField>
                            <GroupedField label="チケット URL">
                              <input
                                type="url"
                                value={f.ticketUrl}
                                onInput={(e) =>
                                  updateField(
                                    entry.qid,
                                    'ticketUrl',
                                    (e.target as HTMLInputElement).value,
                                  )
                                }
                                placeholder="https://..."
                                class={ROW_INPUT_CLASS}
                              />
                            </GroupedField>
                            <GroupedField label="特記事項">
                              <input
                                type="text"
                                value={f.notes}
                                onInput={(e) =>
                                  updateField(entry.qid, 'notes', (e.target as HTMLInputElement).value)
                                }
                                placeholder="任意メモ"
                                class={ROW_INPUT_CLASS}
                              />
                            </GroupedField>
                            <GroupedField label="統合先の明示指定（通常は自動判定）">
                              <select
                                value={f.targetSessionId}
                                onChange={(e) =>
                                  updateField(
                                    entry.qid,
                                    'targetSessionId',
                                    (e.target as HTMLSelectElement).value,
                                  )
                                }
                                class={ROW_INPUT_CLASS}
                              >
                                <option value="" class="bg-stone-900">自動判定</option>
                                {sessions
                                  .slice()
                                  .reverse()
                                  .map((s) => (
                                    <option key={s.id} value={s.id} class="bg-stone-900">
                                      {s.date} {s.live_start || '--:--'} {s.title}
                                    </option>
                                  ))}
                              </select>
                            </GroupedField>
                          </div>
                        )}
                      </div>
                    </div>

                    {entry.commitError && (
                      <p class="text-xs text-rose-300 bg-rose-500/10 border border-rose-500/30 rounded-xl p-2.5">
                        {entry.commitError}
                      </p>
                    )}

                    <button
                      type="button"
                      disabled={entry.aiRunning}
                      onClick={() => void commitEntry(entry)}
                      class="w-full py-2.5 px-4 rounded-xl bg-emerald-500/20 hover:bg-emerald-500/30 border border-emerald-500/45 text-emerald-200 font-bold text-sm disabled:opacity-50 transition-colors"
                    >
                      本番反映する
                    </button>
                  </div>
                );
              })}
            </>
          ) : (
            /* 公開中予定タブ */
            <div class="space-y-2.5">
              <div class="flex items-center justify-between gap-2">
                <span class="text-xs font-bold text-stone-400">
                  公開中 {sessions.length}件
                </span>
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    disabled={reconciling}
                    onClick={() => void handleReconcile()}
                    class="px-3 py-1.5 rounded-xl bg-amber-500/15 border border-amber-500/30 text-xs font-bold text-amber-300 disabled:opacity-50"
                  >
                    {reconciling ? '実行中...' : '名寄せ実行'}
                  </button>
                  <button
                    type="button"
                    onClick={() => void loadSessions(groupId)}
                    class="p-1.5 rounded-xl bg-white/5 border border-white/10 text-stone-300"
                    aria-label="再読み込み"
                  >
                    <Icon
                      icon={RefreshCw}
                      class={`w-3.5 h-3.5 ${sessionsLoading ? 'animate-spin' : ''}`}
                    />
                  </button>
                </div>
              </div>

              {reconcileMsg && (
                <p class="text-xs text-stone-200 bg-stone-900 border border-white/10 rounded-xl p-2.5">
                  {reconcileMsg}
                </p>
              )}

              {sessions
                .slice()
                .reverse()
                .map((s) => (
                  <div key={s.id} class="soft-card rounded-2xl p-3 space-y-1">
                    <div class="flex items-center justify-between gap-2 text-xs text-sky-400 font-bold">
                      <span>{s.date}</span>
                      <span>{formatTimeRange(s.live_start, s.live_end)}</span>
                    </div>
                    <p class="text-sm font-bold text-white leading-snug">{s.title}</p>
                    <p class="text-xs text-stone-400">{s.venue}</p>
                    {(s.admission_bonus || s.photo_rule) && (
                      <p class="text-xs text-stone-300 pt-0.5">
                        {[s.admission_bonus, s.photo_rule].filter(Boolean).join(' / ')}
                      </p>
                    )}
                  </div>
                ))}
            </div>
          )}
        </main>

        {/* 下部 2 タブナビ */}
        <nav class="flex-shrink-0 pt-2 pb-2.5 border-t border-white/5 bg-stone-950/80 backdrop-blur-md">
          <div class="grid grid-cols-2 p-1 rounded-2xl bg-stone-900 border border-white/10">
            <button
              type="button"
              onClick={() => setActiveTab('intake')}
              class={`flex items-center justify-center gap-1.5 py-2.5 rounded-xl text-xs font-bold transition-colors ${
                activeTab === 'intake'
                  ? 'bg-gradient-to-r from-rose-500 to-rose-400 text-stone-950 shadow-md'
                  : 'text-stone-400 hover:text-stone-200'
              }`}
            >
              <Icon icon={Plus} class="w-3.5 h-3.5" />
              <span>登録</span>
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('sessions')}
              class={`flex items-center justify-center gap-1.5 py-2.5 rounded-xl text-xs font-bold transition-colors ${
                activeTab === 'sessions'
                  ? 'bg-gradient-to-r from-rose-500 to-rose-400 text-stone-950 shadow-md'
                  : 'text-stone-400 hover:text-stone-200'
              }`}
            >
              <Icon icon={List} class="w-3.5 h-3.5" />
              <span>公開中予定 ({sessions.length})</span>
            </button>
          </div>
        </nav>
      </div>

      <SyncAccountModal
        isOpen={syncModalOpen}
        session={syncSession}
        syncError={null}
        isSyncing={false}
        onClose={() => setSyncModalOpen(false)}
        onAuthSuccess={(session, attended) => {
          setSyncSession(session);
          persistAttendedIds(attended);
          setSyncModalOpen(false);
          void loadSessions(groupId);
        }}
        onSyncCompleted={(session, attended) => {
          setSyncSession(session);
          persistAttendedIds(attended);
          void loadSessions(groupId);
        }}
        onLoggedOut={() => {
          setSyncSession(null);
          setSyncModalOpen(false);
        }}
      />
    </div>
  );
}
