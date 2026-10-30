import { useState, useEffect, useRef } from 'preact/hooks';
import { ScheduleData, Session } from './types/schedule';
import { loadScheduleData } from './state';
import { Header } from './components/Header';
import { Navigation, TabName } from './components/Navigation';
import { CockpitView } from './components/CockpitView';
import { ListView } from './components/ListView';
import { CalendarView } from './components/CalendarView';
import { LightboxModal } from './components/LightboxModal';
import { PromotionBanner } from './components/PromotionBanner';

export function App() {
  const [data, setData] = useState<ScheduleData | null>(null);
  const [isFallback, setIsFallback] = useState(false);
  const [activeTab, setActiveTab] = useState<TabName>('next');
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null);
  const [previousTab, setPreviousTab] = useState<TabName>('week');
  const [selectedImageIndex, setSelectedImageIndex] = useState(0);
  const [isLightboxOpen, setIsLightboxOpen] = useState(false);

  const carouselRef = useRef<HTMLDivElement>(null);
  const tabNames: TabName[] = ['next', 'week', 'month'];

  useEffect(() => {
    loadScheduleData()
      .then((loadedData) => {
        setData(loadedData);
        // 初期選択セッションの決定
        if (loadedData.current_next_id) {
          setSelectedSessionId(loadedData.current_next_id);
        } else if (loadedData.sessions?.length > 0) {
          setSelectedSessionId(loadedData.sessions[0].id);
        }
      })
      .catch((err) => {
        console.warn('Initial load fallback', err);
        setIsFallback(true);
      });
  }, []);

  // カルーセルの横スクロール位置を同期
  const scrollToTab = (tab: TabName) => {
    setActiveTab(tab);
    const index = tabNames.indexOf(tab);
    if (carouselRef.current && index !== -1) {
      const width = carouselRef.current.clientWidth;
      carouselRef.current.scrollTo({
        left: index * width,
        behavior: 'smooth',
      });
    }
  };

  // カルーセルのスクロールイベント監視
  const handleCarouselScroll = () => {
    if (!carouselRef.current) return;
    const { scrollLeft, clientWidth } = carouselRef.current;
    if (!clientWidth) return;
    const progress = scrollLeft / clientWidth;
    const nearestIndex = Math.max(0, Math.min(tabNames.length - 1, Math.round(progress)));
    const targetTab = tabNames[nearestIndex];
    if (activeTab !== targetTab) {
      setActiveTab(targetTab);
    }
  };

  // 直近ライブへの復帰
  const handleResetToNextLive = () => {
    if (!data?.sessions?.length) return;
    const nextId = data.current_next_id || data.sessions[0].id;
    setSelectedSessionId(nextId);
    setSelectedImageIndex(0);
    scrollToTab('next');
  };

  // セッション詳細の表示
  const handleSelectSession = (id: string) => {
    setPreviousTab(activeTab);
    setSelectedSessionId(id);
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

  const currentSession =
    data.sessions.find((s: Session) => s.id === selectedSessionId) || data.sessions[0];
  const isNextLive = selectedSessionId === (data.current_next_id || data.sessions[0]?.id);
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
                isNextLive={isNextLive}
                selectedImageIndex={selectedImageIndex}
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
              onSelectSession={handleSelectSession}
            />
          </section>

          {/* スライド 3: 月間カレンダー */}
          <section class="w-full shrink-0 h-full snap-start px-0.5">
            <CalendarView
              sessions={data.sessions}
              onSelectSession={handleSelectSession}
            />
            <PromotionBanner promotions={data.promotions} />
          </section>
        </main>

        {/* 下部ナビゲーションバー */}
        <Navigation
          activeTab={activeTab}
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
