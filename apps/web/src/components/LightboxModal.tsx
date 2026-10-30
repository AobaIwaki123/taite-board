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

  // ズーム・パン状態
  const [scale, setScale] = useState(1);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [isPanning, setIsPanning] = useState(false);
  const lastTapRef = useRef<{ time: number; x: number; y: number }>({ time: 0, x: 0, y: 0 });
  const panStartRef = useRef<{ x: number; y: number }>({ x: 0, y: 0 });
  const singleTapTimer = useRef<number | null>(null);

  // 画像切り替え時にズームをリセット
  useEffect(() => {
    setScale(1);
    setPan({ x: 0, y: 0 });
  }, [currentIndex]);

  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (scale > 1) {
          setScale(1);
          setPan({ x: 0, y: 0 });
        } else {
          onClose();
        }
      }
      if (scale === 1) {
        if (e.key === 'ArrowRight' && currentIndex < images.length - 1) {
          onSelectIndex(currentIndex + 1);
        }
        if (e.key === 'ArrowLeft' && currentIndex > 0) {
          onSelectIndex(currentIndex - 1);
        }
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose, currentIndex, images.length, onSelectIndex, scale]);

  if (!isOpen) return null;

  const currentImage = images[currentIndex] || images[0];

  const handleTouchStart = (e: TouchEvent) => {
    if (e.touches.length !== 1) return;
    const touch = e.touches[0];
    touchStartX.current = touch.clientX;
    touchStartY.current = touch.clientY;
    touchStartTime.current = Date.now();
    isSwiping.current = false;

    if (scale > 1) {
      // 拡大中のパン移動開始
      setIsPanning(true);
      panStartRef.current = {
        x: touch.clientX - pan.x,
        y: touch.clientY - pan.y,
      };
    } else {
      setIsDragging(true);
      setDragDeltaX(0);
    }
  };

  const handleTouchMove = (e: TouchEvent) => {
    if (touchStartX.current === null || e.touches.length !== 1) return;
    const touch = e.touches[0];
    const diffX = touch.clientX - touchStartX.current;

    if (scale > 1) {
      // 拡大中のパン移動
      const newX = touch.clientX - panStartRef.current.x;
      const newY = touch.clientY - panStartRef.current.y;
      const maxPanX = (window.innerWidth * (scale - 1)) / 1.6;
      const maxPanY = (window.innerHeight * (scale - 1)) / 1.6;
      setPan({
        x: Math.max(-maxPanX, Math.min(maxPanX, newX)),
        y: Math.max(-maxPanY, Math.min(maxPanY, newY)),
      });
    } else {
      setDragDeltaX(diffX);
    }
  };

  const handleTouchEnd = (e: TouchEvent) => {
    if (touchStartX.current === null || touchStartY.current === null) return;
    const touch = e.changedTouches[0];
    const diffX = touch.clientX - touchStartX.current;
    const diffY = touch.clientY - touchStartY.current;
    const absX = Math.abs(diffX);
    const absY = Math.abs(diffY);
    const duration = Date.now() - touchStartTime.current;

    touchStartX.current = null;
    touchStartY.current = null;
    setIsDragging(false);
    setIsPanning(false);
    setDragDeltaX(0);

    // ダブルタップ判定 (前回のタップから320ms以内かつ移動が小さい)
    const now = Date.now();
    const timeSinceLastTap = now - lastTapRef.current.time;
    const distFromLastTap = Math.hypot(
      touch.clientX - lastTapRef.current.x,
      touch.clientY - lastTapRef.current.y
    );

    if (timeSinceLastTap < 320 && distFromLastTap < 30) {
      // ダブルタップ成立
      if (singleTapTimer.current) {
        clearTimeout(singleTapTimer.current);
        singleTapTimer.current = null;
      }
      lastTapRef.current = { time: 0, x: 0, y: 0 };
      isSwiping.current = true;
      setTimeout(() => {
        isSwiping.current = false;
      }, 300);

      if (scale > 1) {
        setScale(1);
        setPan({ x: 0, y: 0 });
      } else {
        setScale(2.5);
        const tapOffsetX = (window.innerWidth / 2 - touch.clientX) * 0.7;
        const tapOffsetY = (window.innerHeight / 2 - touch.clientY) * 0.7;
        setPan({ x: tapOffsetX, y: tapOffsetY });
      }
      return;
    }

    lastTapRef.current = {
      time: now,
      x: touch.clientX,
      y: touch.clientY,
    };

    // 拡大中なら画像切り替えやクローズは行わない
    if (scale > 1) {
      return;
    }

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

    // タップ操作判定 (ダブルタップ猶予を待ってクローズ)
    if (absX < 10 && absY < 10) {
      if (singleTapTimer.current) clearTimeout(singleTapTimer.current);
      singleTapTimer.current = window.setTimeout(() => {
        onClose();
      }, 280);
    }
  };

  const handleBackdropClick = () => {
    if (isSwiping.current) return;
    if (singleTapTimer.current) {
      clearTimeout(singleTapTimer.current);
      singleTapTimer.current = null;
    }
    if (scale > 1) {
      setScale(1);
      setPan({ x: 0, y: 0 });
    } else {
      onClose();
    }
  };

  const handleDoubleClick = (e: MouseEvent) => {
    e.stopPropagation();
    if (scale > 1) {
      setScale(1);
      setPan({ x: 0, y: 0 });
    } else {
      setScale(2.5);
      setPan({ x: 0, y: 0 });
    }
  };

  // ドラッグ量に応じた進行度 (0.0〜1.0)
  const dragFraction = isDragging ? dragDeltaX / 220 : 0;
  const currentDragProgress = Math.max(
    0,
    Math.min(images.length - 1, currentIndex - dragFraction)
  );

  return (
    <div
      onClick={handleBackdropClick}
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
      <div class="flex-1 flex items-center justify-center w-full min-h-0 p-2 overflow-hidden">
        {currentImage?.url && (
          <img
            src={currentImage.url}
            referrerPolicy="no-referrer"
            alt={currentImage.label || '拡大画像'}
            onDblClick={handleDoubleClick}
            class="max-w-[94vw] max-h-[85vh] object-contain rounded-2xl shadow-2xl ring-1 ring-white/10 select-none"
            style={{
              transform: `translate(${pan.x + (isDragging ? dragDeltaX * 0.7 : 0)}px, ${pan.y}px) scale(${scale})`,
              transition:
                isDragging || isPanning
                  ? 'none'
                  : 'transform 240ms cubic-bezier(0.2, 0.8, 0.2, 1)',
              cursor: scale > 1 ? (isPanning ? 'grabbing' : 'grab') : 'pointer',
              touchAction: 'none',
            }}
          />
        )}
      </div>

      {/* 拡大中インジケーター */}
      {scale > 1 && (
        <div class="absolute bottom-4 left-1/2 -translate-x-1/2 px-3 py-1 rounded-full bg-stone-900/90 border border-white/20 text-xs font-mono text-rose-300 backdrop-blur shadow-lg pointer-events-none select-none animate-fade-in">
          拡大中 ({scale.toFixed(1)}x) ・ ダブルタップで戻る
        </div>
      )}
    </div>
  );
}
