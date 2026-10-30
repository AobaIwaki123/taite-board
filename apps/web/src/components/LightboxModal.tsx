import { useEffect, useRef, useState } from 'preact/hooks';
import { SessionImage } from '../types/schedule';
import { Icon } from './Icon';
import { CalendarDays, Image as ImageIcon } from '../utils/icons';

interface LightboxModalProps {
  isOpen: boolean;
  images: SessionImage[];
  currentIndex: number;
  onSelectIndex: (idx: number) => void;
  onClose: () => void;
}

export function LightboxModal({
  isOpen,
  images,
  currentIndex,
  onSelectIndex,
  onClose,
}: LightboxModalProps) {
  const touchStartX = useRef<number | null>(null);
  const touchStartY = useRef<number | null>(null);
  const touchStartTime = useRef<number>(0);
  const isSwiping = useRef(false);

  const [dragDeltaX, setDragDeltaX] = useState(0);
  const [isDragging, setIsDragging] = useState(false);

  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
      if (e.key === 'ArrowRight' && currentIndex < images.length - 1) {
        onSelectIndex(currentIndex + 1);
      }
      if (e.key === 'ArrowLeft' && currentIndex > 0) {
        onSelectIndex(currentIndex - 1);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose, currentIndex, images.length, onSelectIndex]);

  if (!isOpen) return null;

  const currentImage = images[currentIndex] || images[0];

  const handleTouchStart = (e: TouchEvent) => {
    if (e.touches.length !== 1) return;
    touchStartX.current = e.touches[0].clientX;
    touchStartY.current = e.touches[0].clientY;
    touchStartTime.current = Date.now();
    isSwiping.current = false;
    setIsDragging(true);
    setDragDeltaX(0);
  };

  const handleTouchMove = (e: TouchEvent) => {
    if (touchStartX.current === null) return;
    const currentX = e.touches[0].clientX;
    const diffX = currentX - touchStartX.current;
    setDragDeltaX(diffX);
  };

  const handleTouchEnd = (e: TouchEvent) => {
    if (touchStartX.current === null || touchStartY.current === null) return;
    const endX = e.changedTouches[0].clientX;
    const endY = e.changedTouches[0].clientY;
    const diffX = endX - touchStartX.current;
    const diffY = endY - touchStartY.current;
    const absX = Math.abs(diffX);
    const absY = Math.abs(diffY);
    const duration = Date.now() - touchStartTime.current;

    touchStartX.current = null;
    touchStartY.current = null;
    setIsDragging(false);
    setDragDeltaX(0);

    // 水平スワイプで画像切替 (40px以上かつ水平方向が垂直方向より大きく600ms以内)
    if (absX > 40 && absX > absY && duration < 600) {
      isSwiping.current = true;
      setTimeout(() => {
        isSwiping.current = false;
      }, 300);

      if (diffX < 0 && currentIndex < images.length - 1) {
        onSelectIndex(currentIndex + 1);
      } else if (diffX > 0 && currentIndex > 0) {
        onSelectIndex(currentIndex - 1);
      }
      return;
    }

    // 下スワイプでモーダルを閉じる
    if (diffY > 80 && absY > absX && duration < 600) {
      isSwiping.current = true;
      setTimeout(() => {
        isSwiping.current = false;
      }, 300);
      onClose();
      return;
    }

    // タップ操作判定 (移動がほぼない場合)
    if (absX < 10 && absY < 10) {
      onClose();
    }
  };

  const handleClick = () => {
    if (isSwiping.current) return;
    onClose();
  };

  // ドラッグ量に応じた進行度 (0.0〜1.0)
  const dragFraction = isDragging ? dragDeltaX / 220 : 0;
  const currentDragProgress = Math.max(
    0,
    Math.min(images.length - 1, currentIndex - dragFraction)
  );

  return (
    <div
      onClick={handleClick}
      onTouchStart={handleTouchStart}
      onTouchMove={handleTouchMove}
      onTouchEnd={handleTouchEnd}
      class="fixed inset-0 z-50 bg-black/95 backdrop-blur-md flex flex-col p-4 animate-fade-in cursor-pointer select-none touch-none"
    >
      {/* 複数画像切替ピル（中央配置・スワイプにリアルタイム追従） */}
      {images.length > 1 && (
        <div
          class="flex items-center justify-center pt-2 pb-1 shrink-0 z-10"
          onClick={(e) => e.stopPropagation()}
        >
          <div class="relative flex items-center p-1 rounded-full bg-stone-900/90 border border-white/10 backdrop-blur shadow-lg">
            {/* スライディング・ピルインジケーター */}
            <div
              class="absolute top-1 bottom-1 w-[calc(50%-4px)] rounded-full bg-rose-500 shadow pointer-events-none"
              style={{
                transform: `translateX(${currentDragProgress * 100}%)`,
                transition: isDragging ? 'none' : 'transform 200ms ease-out',
              }}
            />
            {images.map((img, idx) => {
              const isActive = Math.abs(currentDragProgress - idx) < 0.5;
              const isTimetable = img.type === 'timetable';
              return (
                <button
                  key={idx}
                  onClick={() => onSelectIndex(idx)}
                  class={`relative z-10 flex items-center gap-1.5 px-3.5 py-1.5 rounded-full text-xs font-bold transition-colors duration-150 cursor-pointer ${
                    isActive ? 'text-stone-950' : 'text-stone-300 hover:text-white'
                  }`}
                >
                  <Icon icon={isTimetable ? CalendarDays : ImageIcon} class="w-3.5 h-3.5" />
                  <span>{isTimetable ? 'タイテ表' : 'フライヤー'}</span>
                </button>
              );
            })}
          </div>
        </div>
      )}

      {/* 画像プレビュー本体 */}
      <div class="flex-1 flex items-center justify-center w-full min-h-0 p-2">
        {currentImage?.url && (
          <img
            src={currentImage.url}
            referrerPolicy="no-referrer"
            alt={currentImage.label || '拡大画像'}
            class="max-w-[94vw] max-h-[85vh] object-contain rounded-2xl shadow-2xl ring-1 ring-white/10"
            style={{
              transform: `translateX(${isDragging ? dragDeltaX * 0.7 : 0}px)`,
              transition: isDragging ? 'none' : 'transform 200ms ease-out',
            }}
          />
        )}
      </div>
    </div>
  );
}
