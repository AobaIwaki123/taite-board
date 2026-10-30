import { Promotion } from '../types/schedule';
import { Icon } from './Icon';
import { ExternalLink } from '../utils/icons';

interface PromotionBannerProps {
  promotions?: Promotion[];
}

export function PromotionBanner({ promotions }: PromotionBannerProps) {
  if (!promotions || promotions.length === 0) return null;
  const promo = promotions[0];

  return (
    <div class="rounded-2xl overflow-hidden soft-card border border-rose-500/40 bg-stone-900 shadow-xl my-2">
      <div class="w-full h-36 overflow-hidden bg-stone-950">
        <img
          src={promo.image_url}
          referrerpolicy="no-referrer"
          alt={promo.title}
          class="w-full h-full object-cover object-center"
        />
      </div>

      <div class="p-3.5 space-y-2 bg-stone-900">
        <div class="flex items-center justify-between text-xs">
          <span class="px-2 py-0.5 rounded-md font-bold bg-red-600 text-white shadow">
            {promo.title.includes('クラファン') ? 'クラウドファンディング' : 'お知らせ'}
          </span>
          {promo.deadline && (
            <span class="text-rose-300 font-medium">受付：{promo.deadline}</span>
          )}
        </div>

        <div class="space-y-0.5">
          <h3 class="text-sm font-extrabold text-white">{promo.title}</h3>
          {promo.subtitle && (
            <p class="text-xs text-stone-300 leading-relaxed">{promo.subtitle}</p>
          )}
        </div>

        <a
          href={promo.cta_url}
          target="_blank"
          rel="noopener noreferrer"
          class="flex items-center justify-center gap-1.5 w-full py-2.5 rounded-xl bg-gradient-to-r from-red-600 to-rose-600 hover:from-red-500 hover:to-rose-500 text-white font-bold text-xs shadow-md transition-all"
        >
          <span>CAMPFIREで支援する</span>
          <Icon icon={ExternalLink} class="w-3.5 h-3.5" />
        </a>
      </div>
    </div>
  );
}
