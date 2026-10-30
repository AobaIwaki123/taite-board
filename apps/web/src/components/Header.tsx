import { useState } from 'preact/hooks';

interface HeaderProps {
  onLogoClick: () => void;
  isFallback: boolean;
  generatedAt?: string;
}

export function Header({ onLogoClick, isFallback, generatedAt }: HeaderProps) {
  const [isPopping, setIsPopping] = useState(false);

  const handleClick = () => {
    setIsPopping(true);
    setTimeout(() => setIsPopping(false), 450);
    onLogoClick();
  };

  // 鮮度チェック: オフライン時または24時間経過時に警告ラベル表示
  let freshnessLabel: string | null = null;
  if (isFallback) {
    freshnessLabel = 'オフライン保存版を表示中';
  } else if (generatedAt) {
    try {
      const diffHours = (Date.now() - new Date(generatedAt).getTime()) / (1000 * 60 * 60);
      if (diffHours >= 24) {
        const days = Math.floor(diffHours / 24);
        freshnessLabel = `${days}日前の情報です`;
      }
    } catch {
      // ignore
    }
  }

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
          🎀
        </span>
        <span class="text-base font-extrabold text-white tracking-wide group-hover:text-rose-300 transition-colors">
          リボンクロック
        </span>
      </button>

      {freshnessLabel && (
        <div class="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-amber-500/15 border border-amber-500/30 text-amber-300 text-xs font-bold">
          <span class="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse" />
          <span>{freshnessLabel}</span>
        </div>
      )}
    </header>
  );
}
