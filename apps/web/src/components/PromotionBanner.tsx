import { Promotion } from '../types/schedule';
import { getDeadlineInfo } from '../utils/date';
import { Icon } from './Icon';
import { ExternalLink } from '../utils/icons';

interface PromotionBannerProps {
  promotions?: Promotion[];
}

export function PromotionBanner({ promotions }: PromotionBannerProps) {
  if (!promotions || promotions.length === 0) return null;
  // 生誕祭プロモーションを優先取得
  const promo =
    promotions.find((p) => p.id === 'promo-ayyu-birthday' || p.id.includes('birthday')) ||
    promotions[0];
  if (!promo) return null;

  const badgeText =
    promo.badge || (promo.title.includes('生誕') ? '生誕祭クラファン' : 'お知らせ');
  const ctaText = promo.cta_label || '詳細・支援ページへ';
  const deadlineInfo = getDeadlineInfo(promo.deadline);

  return (
    <div class="rounded-2xl overflow-hidden soft-card border-2 border-rose-500/50 bg-stone-900 shadow-xl my-3">
      {promo.image_url && (
        <div class="w-full h-44 overflow-hidden bg-stone-950">
          <img
            src={promo.image_url}
            referrerpolicy="no-referrer"
            alt={promo.title}
            class="w-full h-full object-cover object-center"
          />
        </div>
      )}

      <div class="p-3.5 space-y-2.5 bg-stone-900">
        <div class="flex items-center justify-between text-xs">
          <span class="px-2.5 py-0.5 rounded-md font-bold bg-red-600 text-white shadow text-xs">
            {badgeText}
          </span>
          {deadlineInfo ? (
            <div class="flex items-center gap-1.5">
              <span class="text-stone-300 font-medium text-xs">
                {deadlineInfo.formattedDeadline}
              </span>
              <span
                class={`px-2 py-0.5 rounded-md text-xs font-bold border ${
                  deadlineInfo.isExpired
                    ? 'bg-stone-800 text-stone-400 border-white/10'
                    : deadlineInfo.isUrgent
                    ? 'bg-rose-500 text-stone-950 border-rose-400 font-black animate-pulse'
                    : 'bg-rose-500/20 text-rose-300 border-rose-500/30'
                }`}
              >
                {deadlineInfo.countdownText}
              </span>
            </div>
          ) : (
            <span class="text-rose-300 font-bold text-xs">受付中</span>
          )}
        </div>

        <div class="space-y-1">
          <h3 class="text-base font-extrabold text-white leading-snug">{promo.title}</h3>
          {promo.subtitle && (
            <p class="text-xs text-stone-300 leading-relaxed">{promo.subtitle}</p>
          )}
        </div>

        <a
          href={promo.cta_url}
          target="_blank"
          rel="noopener noreferrer"
          class="flex items-center justify-center gap-1.5 w-full py-2.5 rounded-xl bg-gradient-to-r from-red-600 to-rose-600 hover:from-red-500 hover:to-rose-500 text-white font-black text-xs shadow-md transition-all"
        >
          <span>{ctaText}</span>
          <Icon icon={ExternalLink} class="w-3.5 h-3.5" />
        </a>
      </div>
    </div>
  );
}
