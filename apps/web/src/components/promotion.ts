/**
 * Promotion Component (Banner / Crowdfunding CTA)
 */

import { state } from '../state.ts';
import { refreshLucideIcons } from '../utils/icons.ts';

export function renderPromotion(size: 'large' | 'medium' | 'small' = 'large'): void {
  const container = document.getElementById('promo-container');
  if (!container || !state.appData || !state.appData.promotions) return;
  const promo = state.appData.promotions.find(p => p.id === 'promo-ayyu-birthday') || state.appData.promotions[0];
  if (!promo) return;

  if (size === 'large') {
    container.innerHTML = `
      <div class="rounded-2xl overflow-hidden soft-card border-2 border-rose-500/50 bg-stone-900 shadow-xl">
        <div class="w-full h-44 overflow-hidden bg-stone-950">
          <img src="${promo.image_url}" referrerpolicy="no-referrer" alt="${promo.title}" class="w-full h-full object-cover object-center">
        </div>

        <div class="p-3.5 space-y-2.5 bg-stone-900">
          <div class="flex items-center justify-between">
            <span class="px-2.5 py-0.5 rounded-md text-xs font-black bg-red-600 text-white shadow">🎂 生誕祭クラファン</span>
            <span class="text-xs font-bold text-rose-300">受付期間：〜11/22まで</span>
          </div>

          <div class="space-y-1">
            <h3 class="text-base font-extrabold text-white">
              <span class="text-rose-400 font-bold">神楽あゆ 生誕祭2026 ❤️</span>
            </h3>
            <p class="text-xs text-stone-300 leading-relaxed">初めての生誕祭＆20歳の大切な節目🎀 皆さまと素敵な1日をつくるためのクラファン開催中！</p>
          </div>

          <a href="${promo.cta_url}" target="_blank" rel="noopener" class="flex items-center justify-center gap-2 w-full py-2.5 rounded-xl bg-gradient-to-r from-red-600 to-rose-600 hover:from-red-500 hover:to-rose-500 text-white font-black text-xs shadow-lg shadow-rose-950/50 transition-all">
            <span>CAMPFIREで支援する</span>
            <i data-lucide="external-link" class="w-3.5 h-3.5"></i>
          </a>
        </div>
      </div>
    `;
  } else if (size === 'medium') {
    container.innerHTML = `
      <div class="rounded-2xl p-3 soft-card border border-rose-500/40 bg-stone-900 shadow-md">
        <div class="flex items-center gap-3">
          <img src="${promo.image_url}" referrerpolicy="no-referrer" alt="${promo.title}" class="w-16 h-16 rounded-xl object-cover shrink-0 border border-rose-400/30 shadow">
          <div class="flex-1 min-w-0 space-y-1">
            <div class="flex items-center justify-between">
              <span class="px-2 py-0.5 rounded text-[11px] font-bold bg-red-600 text-white">🎂 生誕祭クラファン</span>
              <span class="text-[11px] font-medium text-rose-300">〜11/22</span>
            </div>
            <h3 class="text-xs font-bold text-white truncate">神楽あゆ 生誕祭2026 ❤️</h3>
            <a href="${promo.cta_url}" target="_blank" rel="noopener" class="inline-flex items-center gap-1 text-xs font-bold text-rose-400 hover:underline">
              <span>CAMPFIREで支援 ↗</span>
            </a>
          </div>
        </div>
      </div>
    `;
  } else {
    container.innerHTML = `
      <div class="rounded-xl px-3 py-2 soft-card border border-rose-500/30 bg-rose-950/20 flex items-center justify-between text-xs">
        <a href="${promo.cta_url}" target="_blank" rel="noopener" class="flex items-center gap-2 truncate flex-1 hover:underline">
          <span class="text-sm">🎂</span>
          <span class="font-bold text-white truncate">神楽あゆ生誕クラファン開催中 (〜11/22)</span>
          <span class="text-rose-400 shrink-0 font-bold ml-1">支援 ↗</span>
        </a>
      </div>
    `;
  }

  refreshLucideIcons();
}
