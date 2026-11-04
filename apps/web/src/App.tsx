import { useState, useEffect, useRef } from 'preact/hooks';
import { ScheduleData, Session } from './types/schedule';
import { loadScheduleData } from './state';
import { resolveCurrentGroup, GroupConfig, isPortalMode } from './config/group';
import { updatePageMetadata } from './utils/meta';
import { resolveNextSession } from './utils/date';
import { getAttendedSessionIds, toggleAttendedSessionId } from './utils/attendance';
import { getSessionDelayOffset, setSessionDelayOffset } from './utils/delay';
import { Header } from './components/Header';
import { Navigation, TabName } from './components/Navigation';
import { CockpitView } from './components/CockpitView';
import { ListView } from './components/ListView';
import { CalendarView } from './components/CalendarView';
import { LightboxModal } from './components/LightboxModal';
import { PortalView } from './components/PortalView';
import { LiveCrossTrackView } from './components/LiveCrossTrackView';

function isLiveTrackMode(): { isLive: boolean; eventId?: string } {
  if (typeof window === 'undefined') return { isLive: false };
  const url = new URL(window.location.href);
  const pathname = url.pathname;
  const viewParam = url.searchParams.get('view');

  if (viewParam === 'live') {
    return { isLive: true, eventId: url.searchParams.get('event') || '2026-11-03/mariage' };
  }

  const hostname = window.location.hostname.toLowerCase();
  if (hostname.startsWith('live.') || pathname.startsWith('/live/')) {
    const slug = pathname.replace(/^\/live\/?/, '') || '2026-11-03/mariage';
    return { isLive: true, eventId: slug };
  }

  return { isLive: false };
}

export function App() {
  const liveMode = isLiveTrackMode();
  if (liveMode.isLive) {
    return (
      <LiveCrossTrackView
        eventId={liveMode.eventId}
        onBack={() => {
          const url = new URL(window.location.href);
          url.searchParams.delete('view');
          url.searchParams.delete('event');
          if (url.pathname.startsWith('/live/')) {
            window.location.href = '/';
          } else {
            window.location.href = url.toString();
          }
        }}
      />
    );
  }

  if (isPortalMode()) {
    return <PortalView />;
  }

  return <ScheduleApp />;
}

function ScheduleApp() {
  const [data, setData] = useState<ScheduleData | null>(null);
  const [group, setGroup] = useState<GroupConfig>(resolveCurrentGroup());
  const [isFallback, setIsFallback] = useState(false);
  const [activeTab, setActiveTab] = useState<TabName>('next');
  const [customSelectedSessionId, setCustomSelectedSessionId] = useState<string | null>(null);
  const [previousTab, setPreviousTab] = useState<TabName>('week');
  const [selectedImageIndex, setSelectedImageIndex] = useState(0);
  const [isLightboxOpen, setIsLightboxOpen] = useState(false);
  const [scrollProgress, setScrollProgress] = useState(0);
  const [isProgrammatic, setIsProgrammatic] = useState(false);
  const [attendedSessionIds, setAttendedSessionIds] = useState<string[]>([]);
  const [delayOffsets, setDelayOffsets] = useState<{ [id: string]: number }>({});
  const [now, setNow] = useState(new Date());
  const isProgrammaticRef = useRef(false);
  const programmaticTimerRef = useRef<number | null>(null);

  const carouselRef = useRef<HTMLDivElement>(null);
  const tabNames: TabName[] = ['next', 'week', 'month'];

  // 30秒ごとに時刻を更新し、カウントダウン分数の進行や公演終了後の次回自動遷移を反映
  useEffect(() => {
    const timer = setInterval(() => {
      setNow(new Date());
    }, 30000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    updatePageMetadata(group);
  }, [group]);

  useEffect(() => {
    setAttendedSessionIds(getAttendedSessionIds());
    loadScheduleData()
      .then(({ data: loadedData, isFallback: loadedFallback, group: loadedGroup }) => {
        setData(loadedData);
        setIsFallback(loadedFallback);
        setGroup(loadedGroup);
        updatePageMetadata(loadedGroup);
      })
      .catch((err) => {
        console.warn('Initial load fallback', err);
        setIsFallback(true);
      });
  }, []);

  const handleToggleAttended = (sessionId: string) => {
    const updated = toggleAttendedSessionId(sessionId);
    setAttendedSessionIds(updated);
  };

  const handleChangeDelay = (sessionId: string, minutes: number) => {
    setSessionDelayOffset(sessionId, minutes);
    setDelayOffsets((prev) => ({ ...prev, [sessionId]: minutes }));
  };

  // カルーセルの横スクロール位置を同期
  const scrollToTab = (tab: TabName) => {
    setActiveTab(tab);
    const index = tabNames.indexOf(tab);
    if (index === -1) return;

    // クリックによるプログラム移動中はスクロールイベント競合を遮断
    isProgrammaticRef.current = true;
    setIsProgrammatic(true);
    setScrollProgress(index);

    if (programmaticTimerRef.current) clearTimeout(programmaticTimerRef.current);
    programmaticTimerRef.current = window.setTimeout(() => {
      isProgrammaticRef.current = false;
      setIsProgrammatic(false);
    }, 400);

    if (carouselRef.current) {
      const width = carouselRef.current.clientWidth;
      carouselRef.current.scrollTo({
        left: index * width,
        behavior: 'smooth',
      });
    }
  };

  // カルーセルのスクロールイベント監視
  const handleCarouselScroll = () => {
    if (!carouselRef.current || isProgrammaticRef.current) return;
    const { scrollLeft, clientWidth } = carouselRef.current;
    if (!clientWidth) return;
    const progress = scrollLeft / clientWidth;
    setScrollProgress(progress);
    const nearestIndex = Math.max(0, Math.min(tabNames.length - 1, Math.round(progress)));
    const targetTab = tabNames[nearestIndex];
    if (activeTab !== targetTab) {
      setActiveTab(targetTab);
    }
  };

  // 直近ライブへの復帰（カスタム選択を解除して常に最新の次回直近ライブを表示）
  const handleResetToNextLive = () => {
    setCustomSelectedSessionId(null);
    setSelectedImageIndex(0);
    scrollToTab('next');
  };

  // セッション詳細の表示
  const handleSelectSession = (id: string) => {
    setPreviousTab(activeTab);
    setCustomSelectedSessionId(id);
    setSelectedImageIndex(0);
    scrollToTab('next');
  };

  if (!data) {
    return (
      <div class="h-[100dvh] flex items-center justify-center bg-stone-950 text-stone-400 text-sm">
        <div class="flex items-center gap-2">
          <span class="w-2 h-2 rounded-full bg-rose-500 animate-ping" />
          <span>スケジュール読み込み中...</span>
        </div>
      </div>
    );
  }

  const nextSession = resolveNextSession(data.sessions, data.current_next_id, now, 0, delayOffsets);
  const currentSession =
    (customSelectedSessionId && data.sessions.find((s: Session) => s.id === customSelectedSessionId)) ||
    nextSession ||
    data.sessions[0];
  const isNextLive = currentSession?.id === nextSession?.id;
  const currentImages =
    currentSession?.images && currentSession.images.length > 0
      ? currentSession.images
      : [{ type: 'flyer' as const, label: 'フライヤー', url: currentSession?.image_url || '' }];

  return (
    <div class="h-[100dvh] overflow-hidden antialiased selection:bg-rose-500/20 flex flex-col bg-stone-950 text-stone-100">
      <div class="mx-auto max-w-sm w-full px-3.5 pt-3 flex flex-col h-full min-h-0 relative">
        {/* 上部ヘッダー */}
        <Header
          onLogoClick={handleResetToNextLive}
          isFallback={isFallback}
          generatedAt={data.generated_at}
          title={group.name}
          logoText={group.logoText}
        />

        {/* 3画面スライドカルーセル */}
        <main
          ref={carouselRef}
          onScroll={handleCarouselScroll}
          class="flex-1 min-h-0 flex overflow-x-auto no-scrollbar snap-x snap-mandatory pt-2.5"
          style={{ scrollBehavior: 'smooth' }}
        >
          {/* スライド 1: 直近ライブ・公演詳細 */}
          <section class="w-full shrink-0 h-full snap-start px-0.5">
            {currentSession && (
              <CockpitView
                session={currentSession}
                allSessions={data.sessions}
                promotions={data.promotions}
                isNextLive={isNextLive}
                selectedImageIndex={selectedImageIndex}
                isAttended={attendedSessionIds.includes(currentSession.id)}
                delayMinutes={delayOffsets[currentSession.id] ?? getSessionDelayOffset(currentSession.id)}
                onToggleAttended={handleToggleAttended}
                onChangeDelay={(m) => handleChangeDelay(currentSession.id, m)}
                onSelectImageIndex={setSelectedImageIndex}
                onSelectSession={handleSelectSession}
                onResetToNextLive={handleResetToNextLive}
                onBackToList={() => scrollToTab(previousTab)}
                onOpenLightbox={() => setIsLightboxOpen(true)}
              />
            )}
          </section>

          {/* スライド 2: 全日程一覧 */}
          <section class="w-full shrink-0 h-full snap-start px-0.5">
            <ListView
              sessions={data.sessions}
              attendedSessionIds={attendedSessionIds}
              onSelectSession={handleSelectSession}
            />
          </section>

          {/* スライド 3: 月間カレンダー */}
          <section class="w-full shrink-0 h-full snap-start px-0.5">
            <CalendarView
              sessions={data.sessions}
              attendedSessionIds={attendedSessionIds}
              onSelectSession={handleSelectSession}
            />
          </section>
        </main>

        {/* 下部ナビゲーションバー */}
        <Navigation
          activeTab={activeTab}
          scrollProgress={scrollProgress}
          isProgrammatic={isProgrammatic}
          onSelectTab={scrollToTab}
          isViewingSelectedLive={!isNextLive}
        />

      </div>

      {/* 画像拡大モーダル */}
      <LightboxModal
        isOpen={isLightboxOpen}
        images={currentImages}
        currentIndex={selectedImageIndex}
        onSelectIndex={setSelectedImageIndex}
        onClose={() => setIsLightboxOpen(false)}
      />
    </div>
  );
}
