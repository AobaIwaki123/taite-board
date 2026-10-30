import { useEffect } from 'preact/hooks';
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
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const currentImage = images[currentIndex] || images[0];

  return (
    <div
      onClick={onClose}
      class="fixed inset-0 z-50 bg-black/95 backdrop-blur-md flex flex-col p-4 animate-fade-in"
    >
      {/* 上部コントロールバー */}
      <div class="flex items-center justify-between py-2 shrink-0">
        {/* 画像切替ピル */}
        {images.length > 1 ? (
          <div class="flex items-center gap-1.5 p-1 rounded-full bg-stone-900 border border-white/10">
            {images.map((img, idx) => {
              const isActive = idx === currentIndex;
              const isTimetable = img.type === 'timetable';
              return (
                <button
                  key={idx}
                  onClick={(e) => {
                    e.stopPropagation();
                    onSelectIndex(idx);
                  }}
                  class={`flex items-center gap-1 px-3 py-1 rounded-full text-xs font-bold transition-all ${
                    isActive
                      ? 'bg-rose-500 text-stone-950 shadow'
                      : 'text-stone-300 hover:text-white'
                  }`}
                >
                  <Icon icon={isTimetable ? CalendarDays : ImageIcon} class="w-3.5 h-3.5" />
                  <span>{isTimetable ? 'タイテ表' : 'フライヤー'}</span>
                </button>
              );
            })}
          </div>
        ) : (
          <div />
        )}

        {/* アクションボタン */}
        <div class="flex items-center gap-2">
          {currentImage?.url && (
            <a
              href={currentImage.url}
              target="_blank"
              rel="noreferrer noopener"
              onClick={(e) => e.stopPropagation()}
              class="px-3 py-1.5 rounded-full bg-stone-800 border border-white/20 text-stone-200 font-bold text-xs hover:bg-stone-700 hover:text-white transition-colors"
            >
              元画像 ↗
            </a>
          )}
          <button
            onClick={onClose}
            class="px-3 py-1.5 rounded-full bg-stone-800 border border-white/20 text-white font-bold text-xs hover:bg-stone-700 transition-colors"
          >
            ✕ 閉じる
          </button>
        </div>
      </div>

      {/* 画像プレビュー本体 */}
      <div class="flex-1 flex items-center justify-center p-2 cursor-pointer">
        {currentImage?.url && (
          <img
            src={currentImage.url}
            referrerPolicy="no-referrer"
            alt={currentImage.label || '拡大画像'}
            onClick={(e) => e.stopPropagation()}
            class="max-w-[94vw] max-h-[80vh] object-contain rounded-2xl shadow-2xl ring-1 ring-white/20 cursor-default"
          />
        )}
      </div>

      <div class="text-center text-xs text-stone-400 pb-2 pointer-events-none">
        周囲をタップすると閉じます
      </div>
    </div>
  );
}
