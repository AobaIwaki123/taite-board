import { useState } from 'preact/hooks';
import { getFreshnessInfo } from '../utils/date';

interface HeaderProps {
  onLogoClick: () => void;
  isFallback: boolean;
  generatedAt?: string;
  title?: string;
  logoText?: string;
}

export function Header({
  onLogoClick,
  isFallback,
  generatedAt,
  title = 'リボンクロック',
  logoText = '🎀',
}: HeaderProps) {
  const [isPopping, setIsPopping] = useState(false);

  const handleClick = () => {
    setIsPopping(true);
    setTimeout(() => setIsPopping(false), 450);
    onLogoClick();
  };

  // 鮮度チェック (ADR-0008): オフライン時のみフォールバック鮮度インジケーターを表示
  const freshness = getFreshnessInfo({ isFallback, generatedAt });
  const freshnessLabel = freshness?.label ?? null;

  return (
    <header class="flex-shrink-0 flex items-center justify-between py-1 border-b border-white/5 pb-2">
      <button
        onClick={handleClick}
        class="flex items-center gap-2 py-1 px-2 -ml-2 rounded-xl hover:bg-white/5 active:scale-90 transition-all duration-150 select-none group cursor-pointer focus:outline-none"
        title="直近ライブへジャンプ"
      >
        <span
          class={`text-xl inline-block transition-transform duration-200 group-hover:scale-110 ${
            isPopping ? 'animate-ribbon-pop' : ''
          }`}
        >
          {logoText}
        </span>
        <span class="text-base font-extrabold text-white tracking-wide group-hover:text-rose-300 transition-colors">
          {title}
        </span>
      </button>

      <div class="flex items-center gap-2">
        {freshnessLabel && (
          <div class="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-amber-500/15 border border-amber-500/30 text-amber-300 text-xs font-bold">
            <span class="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse" />
            <span>{freshnessLabel}</span>
          </div>
        )}
        <a
          href={typeof window !== 'undefined' && window.location.hostname.endsWith('iorin.net') ? 'https://taite.iorin.net' : '/'}
          class="flex items-center justify-center w-8 h-8 rounded-xl bg-white/5 hover:bg-white/10 active:scale-95 text-stone-400 hover:text-white transition-all duration-150 border border-white/5"
          title="グループ一覧へ戻る"
          aria-label="グループ一覧へ戻る"
        >
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect width="7" height="7" x="3" y="3" rx="1" />
            <rect width="7" height="7" x="14" y="3" rx="1" />
            <rect width="7" height="7" x="14" y="14" rx="1" />
            <rect width="7" height="7" x="3" y="14" rx="1" />
          </svg>
        </a>
      </div>
    </header>
  );
}
