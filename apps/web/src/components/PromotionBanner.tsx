import { Promotion } from '../types/schedule';
import { getDeadlineInfo } from '../utils/date';
import { Icon } from './Icon';
import { ExternalLink } from '../utils/icons';

interface PromotionBannerProps {
  promotions?: Promotion[];
}

function HeroPromotionCard({ promo }: { promo: Promotion }) {
  const isEvent = promo.id.includes('live') || promo.title.includes('開催');
  const deadlineInfo = getDeadlineInfo(
    promo.deadline,
    new Date(),
    { suffix: isEvent ? '開催' : 'まで' }
  );
  const badgeText =
    promo.badge || (promo.title.includes('生誕') ? '生誕祭クラファン' : 'お知らせ');
  const ctaText = promo.cta_label || '詳細・支援ページへ ↗';

  return (
    <div class="rounded-2xl overflow-hidden soft-card border-2 border-rose-500/50 bg-stone-900 shadow-xl my-2.5">
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
            <div class="flex items-center gap-1.5 font-bold">
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

function MediumPromotionCard({ promo }: { promo: Promotion }) {
  const isEvent = promo.id.includes('live') || promo.title.includes('開催');
  const deadlineInfo = getDeadlineInfo(
    promo.deadline,
    new Date(),
    { suffix: isEvent ? '開催' : 'まで' }
  );
  const badgeText = promo.badge || 'お知らせ';
  const ctaText = promo.cta_label || '詳細を見る ↗';

  return (
    <div class="rounded-2xl soft-card border border-rose-500/40 bg-stone-900/95 p-3.5 shadow-lg my-2.5 space-y-2">
      <div class="flex items-center justify-between text-xs">
        <div class="flex items-center gap-1.5">
          <span class="text-base leading-none">🎂</span>
          <span class="px-2 py-0.5 rounded-md font-bold bg-rose-600 text-white shadow text-xs">
            {badgeText}
          </span>
        </div>
        {deadlineInfo ? (
          <div class="flex items-center gap-1.5 font-bold">
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

      <div class="space-y-0.5">
        <h4 class="text-sm font-extrabold text-white leading-snug">{promo.title}</h4>
        {promo.subtitle && (
          <p class="text-xs text-stone-300 leading-relaxed">{promo.subtitle}</p>
        )}
      </div>

      <a
        href={promo.cta_url}
        target="_blank"
        rel="noopener noreferrer"
        class="flex items-center justify-center gap-1.5 w-full py-2 rounded-xl bg-stone-800 hover:bg-stone-700 text-stone-200 hover:text-white border border-white/10 font-bold text-xs transition-colors"
      >
        <span>{ctaText}</span>
        <Icon icon={ExternalLink} class="w-3.5 h-3.5 text-stone-400" />
      </a>
    </div>
  );
}

export function PromotionBanner({ promotions }: PromotionBannerProps) {
  if (!promotions || promotions.length === 0) return null;

  // active なプロモーションを priority 降順で抽出
  const activePromos = promotions
    .filter((p) => p.active !== false)
    .sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0));

  if (activePromos.length === 0) return null;

  return (
    <div class="space-y-2 pt-1">
      {activePromos.map((promo) =>
        promo.type === 'hero' ? (
          <HeroPromotionCard key={promo.id} promo={promo} />
        ) : (
          <MediumPromotionCard key={promo.id} promo={promo} />
        )
      )}
    </div>
  );
}
