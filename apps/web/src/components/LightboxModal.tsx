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
      class="fixed inset-0 z-50 bg-black/95 backdrop-blur-md flex flex-col p-4 animate-fade-in cursor-pointer select-none"
    >
      {/* 複数画像切替ピル（中央配置） */}
      {images.length > 1 && (
        <div
          class="flex items-center justify-center pt-2 pb-1 shrink-0 z-10"
          onClick={(e) => e.stopPropagation()}
        >
          <div class="flex items-center gap-1 p-1 rounded-full bg-stone-900/90 border border-white/10 backdrop-blur shadow-lg">
            {images.map((img, idx) => {
              const isActive = idx === currentIndex;
              const isTimetable = img.type === 'timetable';
              return (
                <button
                  key={idx}
                  onClick={() => onSelectIndex(idx)}
                  class={`flex items-center gap-1.5 px-3.5 py-1.5 rounded-full text-xs font-bold transition-all cursor-pointer ${
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
          />
        )}
      </div>
    </div>
  );
}
