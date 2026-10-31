import { useRef } from 'preact/hooks';
import { Session, Promotion } from '../types/schedule';
import { Icon } from './Icon';
import { PromotionBanner } from './PromotionBanner';
import {
  Calendar,
  CalendarDays,
  Image as ImageIcon,
  MapPin,
  Mic,
  Gift,
  Aperture,
  Ticket,
  Maximize2,
  ChevronLeft,
} from '../utils/icons';

interface CockpitViewProps {
  session: Session;
  allSessions: Session[];
  promotions?: Promotion[];
  isNextLive: boolean;
  selectedImageIndex: number;
  onSelectImageIndex: (idx: number) => void;
  onSelectSession: (id: string) => void;
  onResetToNextLive: () => void;
  onBackToList: () => void;
  onOpenLightbox: () => void;
}

export function CockpitView({
  session,
  allSessions,
  promotions,
  isNextLive,
  selectedImageIndex,
  onSelectImageIndex,
  onSelectSession,
  onResetToNextLive,
  onBackToList,
  onOpenLightbox,
}: CockpitViewProps) {
  // 画像一覧（フライヤー・タイテ表）
  const images =
    session.images && session.images.length > 0
      ? session.images
      : [{ type: 'flyer' as const, label: 'フライヤー', url: session.image_url || '' }];

  const previewTouchStartX = useRef<number | null>(null);
  const previewTouchStartY = useRef<number | null>(null);
  const previewIsSwiping = useRef(false);

  const handlePreviewTouchStart = (e: TouchEvent) => {
    if (e.touches.length !== 1 || images.length <= 1) return;
    previewTouchStartX.current = e.touches[0].clientX;
    previewTouchStartY.current = e.touches[0].clientY;
    previewIsSwiping.current = false;
  };

  const handlePreviewTouchEnd = (e: TouchEvent) => {
    if (
      previewTouchStartX.current === null ||
      previewTouchStartY.current === null ||
      images.length <= 1
    ) {
      return;
    }
    const diffX = e.changedTouches[0].clientX - previewTouchStartX.current;
    const diffY = e.changedTouches[0].clientY - previewTouchStartY.current;
    const absX = Math.abs(diffX);
    const absY = Math.abs(diffY);

    previewTouchStartX.current = null;
    previewTouchStartY.current = null;

    if (absX > 35 && absX > absY) {
      previewIsSwiping.current = true;
      setTimeout(() => {
        previewIsSwiping.current = false;
      }, 300);

      if (diffX < 0 && selectedImageIndex < images.length - 1) {
        onSelectImageIndex(selectedImageIndex + 1);
      } else if (diffX > 0 && selectedImageIndex > 0) {
        onSelectImageIndex(selectedImageIndex - 1);
      }
    }
  };

  const handleImageClick = () => {
    if (previewIsSwiping.current) return;
    onOpenLightbox();
  };

  // 同日開催のセッション一覧（1部/2部などの同日2回し判定）
  const sameDaySessions = allSessions.filter((s) => s.date === session.date);

  // チケット情報
  const ticket = session.ticket;
  const isTicketPending = !ticket?.url || ticket.platform === '公式アナウンス待ち';

  return (
    <div class="h-full overflow-y-auto no-scrollbar tab-slide space-y-3.5 pb-6">
      {/* 別日程閲覧時の復帰バナー */}
      {!isNextLive && (
        <div class="flex items-center justify-between p-2 rounded-xl bg-rose-950/40 border border-rose-500/30 text-xs">
          <button
            onClick={onBackToList}
            class="flex items-center gap-1 text-stone-300 hover:text-white font-medium"
          >
            <Icon icon={ChevronLeft} class="w-3.5 h-3.5" />
            <span>一覧へ戻る</span>
          </button>
          <button
            onClick={onResetToNextLive}
            class="px-2 py-0.5 rounded-lg bg-rose-500 text-stone-950 font-bold hover:bg-rose-400 transition-colors"
          >
            直近ライブへ
          </button>
        </div>
      )}

      {/* フライヤー & タイムテーブル画像プレビュー */}
      <div
        class="relative w-full rounded-2xl overflow-hidden soft-card bg-stone-900 shadow-lg group select-none"
        onTouchStart={handlePreviewTouchStart}
        onTouchEnd={handlePreviewTouchEnd}
        style={{ touchAction: 'pan-y' }}
      >
        <div class="relative w-full h-44 overflow-hidden bg-stone-950 flex items-center justify-center">
          <div
            class="flex w-full h-full transition-transform duration-250 ease-out"
            style={{ transform: `translateX(${-selectedImageIndex * 100}%)` }}
          >
            {images.map((img, idx) => (
              <div
                key={idx}
                class="w-full h-full shrink-0 relative bg-stone-950 flex items-center justify-center"
              >
                {img.url ? (
                  <img
                    src={img.url}
                    referrerPolicy="no-referrer"
                    alt={img.label || session.title}
                    onClick={handleImageClick}
                    class="w-full h-full object-cover object-top cursor-pointer transition-transform duration-300 group-hover:scale-105"
                  />
                ) : (
                  <div class="text-xs text-stone-600">画像なし</div>
                )}
              </div>
            ))}
          </div>

          {/* 拡大タップヒント */}
          <button
            onClick={onOpenLightbox}
            class="absolute top-2.5 right-2.5 p-1.5 rounded-xl bg-stone-950/80 backdrop-blur-md text-stone-200 border border-white/10 hover:text-white transition-colors"
            title="拡大表示"
          >
            <Icon icon={Maximize2} class="w-4 h-4" />
          </button>

          {/* 枚数バッジ */}
          {images.length > 1 && (
            <span class="absolute top-2.5 left-2.5 px-2 py-0.5 rounded-md text-xs font-mono font-bold bg-stone-950/80 text-stone-300 border border-white/10">
              {selectedImageIndex + 1}/{images.length}
            </span>
          )}

          {/* タイテ表 / フライヤー 切替ピル */}
          {images.length > 1 && (
            <div class="absolute bottom-2.5 left-2.5 z-10 flex items-center p-1 rounded-xl bg-stone-950/85 backdrop-blur-md border border-white/10 shadow-lg select-none">
              {/* スライディング・ピルインジケーター */}
              <div
                class="absolute top-1 bottom-1 w-[calc(50%-4px)] rounded-lg bg-rose-500 shadow-sm transition-transform duration-200 ease-out pointer-events-none"
                style={{ transform: `translateX(${selectedImageIndex * 100}%)` }}
              />
              {images.map((img, idx) => {
                const isActive = idx === selectedImageIndex;
                const isTimetable = img.type === 'timetable';
                return (
                  <button
                    key={idx}
                    onClick={(e) => {
                      e.stopPropagation();
                      onSelectImageIndex(idx);
                    }}
                    class={`relative z-10 flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-bold whitespace-nowrap transition-colors duration-150 ${
                      isActive ? 'text-stone-950' : 'text-stone-400 hover:text-stone-200'
                    }`}
                  >
                    <Icon icon={isTimetable ? CalendarDays : ImageIcon} class="w-3.5 h-3.5" />
                    <span>{isTimetable ? 'タイテ表' : 'フライヤー'}</span>
                  </button>
                );
              })}
            </div>
          )}
        </div>
      </div>

      {/* 同日2回し（昼/夜・1部/2部）切り替えピル */}
      {sameDaySessions.length > 1 && (
        <div class="flex items-center gap-1.5 overflow-x-auto no-scrollbar py-0.5">
          {sameDaySessions.map((s) => {
            const isSelected = s.id === session.id;
            const label = s.session_name || s.title.slice(0, 10);
            return (
              <button
                key={s.id}
                onClick={() => onSelectSession(s.id)}
                class={`px-3 py-1 rounded-xl text-xs font-bold transition-all flex items-center gap-1.5 shrink-0 ${
                  isSelected
                    ? 'bg-gradient-to-r from-rose-500 to-rose-600 text-stone-950 shadow-sm'
                    : 'bg-stone-900 text-stone-400 hover:text-stone-200 border border-white/10'
                }`}
              >
                <span>{label}</span>
                <span class="text-xs font-mono opacity-80">{s.live.start_time}~</span>
              </button>
            );
          })}
        </div>
      )}

      {/* 公演ヘッダーカード */}
      <div class="soft-card rounded-2xl p-4 space-y-2 shadow-md">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-1.5 text-xs text-rose-400 font-bold">
            <Icon icon={Calendar} class="w-3.5 h-3.5" />
            <span>
              {session.date} ({session.day_of_week || ''})
            </span>
          </div>
          {isNextLive && (
            <span class="text-xs font-bold text-rose-400 bg-rose-500/15 px-2 py-0.5 rounded border border-rose-500/30">
              ★ 直近ライブ
            </span>
          )}
        </div>

        <h1 class="text-lg font-black text-white leading-snug">{session.title}</h1>

        <div class="flex items-center justify-between gap-2 pt-1 border-t border-white/5 text-xs text-stone-300 min-w-0">
          <a
            href={session.venue.map_url || '#'}
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center gap-1 hover:text-rose-300 transition-colors min-w-0 truncate"
          >
            <Icon icon={MapPin} class="w-3.5 h-3.5 text-rose-400 shrink-0" />
            <span class="font-semibold underline underline-offset-2 truncate">{session.venue.name}</span>
          </a>
          {session.source_tweet_url && (
            <a
              href={session.source_tweet_url}
              target="_blank"
              rel="noreferrer noopener"
              class="flex items-center gap-1 text-stone-400 hover:text-rose-300 transition-colors shrink-0"
            >
              <span>公式告知 ↗</span>
            </a>
          )}
        </div>
      </div>

      {/* 時間・出番情報カード */}
      <div class="grid grid-cols-2 gap-2.5">
        {/* 出番枠 */}
        <div class="soft-card rounded-2xl p-3.5 space-y-1 border-l-4 border-l-rose-500">
          <div class="flex items-center gap-1 text-xs font-bold text-rose-300">
            <Icon icon={Mic} class="w-3.5 h-3.5" />
            <span>出番</span>
          </div>
          <div class="text-xl font-display font-black text-white">
            {session.live.start_time}〜{session.live.end_time || ''}
          </div>
          {session.live.stage && (
            <div class="pt-0.5">
              <span
                class="inline-block px-1.5 py-0.5 rounded bg-rose-500/10 text-rose-300 border border-rose-500/20 text-xs font-semibold truncate max-w-full align-bottom"
                title={session.live.stage}
              >
                {session.live.stage}
              </span>
            </div>
          )}
        </div>

        {/* 特典会枠 */}
        <div class="soft-card rounded-2xl p-3.5 space-y-1 border-l-4 border-l-purple-500">
          <div class="flex items-center gap-1 text-xs font-bold text-purple-300">
            <Icon icon={Gift} class="w-3.5 h-3.5" />
            <span>特典会{session.benefit.type ? ` (${session.benefit.type})` : ''}</span>
          </div>
          <div class="text-xl font-display font-black text-white">
            {session.benefit.start_time}〜{session.benefit.end_time || ''}
          </div>
          {session.benefit.booth && (
            <div class="pt-0.5">
              <span
                class="inline-block px-1.5 py-0.5 rounded bg-purple-500/10 text-purple-300 border border-purple-500/20 text-xs font-semibold truncate max-w-full align-bottom"
                title={session.benefit.booth}
              >
                {session.benefit.booth}
              </span>
            </div>
          )}
        </div>
      </div>

      {/* レギュレーション・特記事項 */}
      <div class="space-y-2">
        {/* 撮影可否ルール */}
        {session.photo?.rule && (
          <div class="p-3 rounded-xl bg-stone-900/80 border border-white/10 text-xs flex items-start gap-2">
            <Icon icon={Aperture} class="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
            <div class="space-y-0.5">
              <span class="font-bold text-emerald-300">撮影可否：</span>
              <span class="text-stone-300">{session.photo.rule}</span>
            </div>
          </div>
        )}

        {/* 入場特典 */}
        {session.admission_bonus && (
          <div class="p-3 rounded-xl bg-stone-900/80 border border-white/10 text-xs flex items-start gap-2">
            <Icon icon={Gift} class="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
            <div class="space-y-0.5">
              <span class="font-bold text-rose-300">入場特典：</span>
              <span class="text-stone-300">
                {session.admission_bonus.replace(/^(?:(?:指名|選べる)?入場特典[：:]\s*)/, '')}
              </span>
            </div>
          </div>
        )}

        {/* チケット価格 */}
        {ticket?.prices && ticket.prices.length > 0 && (
          <div class="p-3 rounded-xl bg-stone-900/80 border border-white/10 text-xs space-y-1.5">
            <div class="flex items-center gap-1 text-stone-400 font-bold">
              <Icon icon={Ticket} class="w-3.5 h-3.5" />
              <span>チケット料金</span>
            </div>
            <div class="flex flex-wrap gap-1.5">
              {ticket.prices.map((p, idx) => (
                <span
                  key={idx}
                  class="px-2 py-0.5 rounded-lg bg-stone-950 border border-white/10 text-xs"
                >
                  <span class="text-stone-400 mr-1">{p.tier}:</span>
                  <strong class="text-rose-300 font-bold">{p.price}</strong>
                </span>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* チケット購入ボタン */}
      <div class="pt-1">
        {isTicketPending ? (
          <div class="flex items-center justify-center gap-1.5 py-3 rounded-xl bg-stone-800 text-stone-400 font-bold text-sm border border-white/10 select-none">
            <Icon icon={Ticket} class="w-4 h-4 shrink-0" />
            <span>チケット告知待ち</span>
          </div>
        ) : (
          <a
            href={ticket.url || '#'}
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center justify-center gap-1.5 py-3 px-4 rounded-xl bg-gradient-to-r from-rose-500 to-rose-600 hover:from-rose-400 hover:to-rose-500 text-stone-950 font-black text-sm shadow-md transition-all"
          >
            <Icon icon={Ticket} class="w-4 h-4 shrink-0" />
            <span class="truncate">
              チケット購入 ({ticket.platform ? ticket.platform.replace(/[（(].*?[）)]/g, '').trim() || '外部サイト' : '外部サイト'})
            </span>
          </a>
        )}
      </div>

      {/* 生誕祭クラファン・プロモーション枠 */}
      <PromotionBanner promotions={promotions} />

      <footer class="pt-1 pb-4 text-center">
        <a
          href="/support.html"
          class="inline-flex min-h-10 items-center rounded-lg px-3 text-xs font-semibold text-stone-400 underline decoration-stone-700 underline-offset-4 transition-colors hover:bg-white/5 hover:text-stone-200 hover:decoration-stone-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-rose-400"
        >
          サイトについて
        </a>
      </footer>
    </div>
  );
}
