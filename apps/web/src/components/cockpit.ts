/**
 * Cockpit Component (Next/Selected Live View)
 */

import { state } from '../state.ts';
import { SessionImage } from '../types/schedule.ts';
import { refreshLucideIcons } from '../utils/icons.ts';

let currentImages: SessionImage[] = [];

export function switchSessionImage(idx: number): void {
  if (!currentImages[idx]) return;
  state.selectedFlyerIdx = idx;
  const target = currentImages[idx];

  const stripImg = document.getElementById('hero-strip-img') as HTMLImageElement | null;
  const lightboxImg = document.getElementById('lightbox-img') as HTMLImageElement | null;
  if (stripImg) stripImg.src = target.url;
  if (lightboxImg) lightboxImg.src = target.url;

  const badge = document.getElementById('hero-img-count-badge');
  if (badge && currentImages.length > 1) {
    badge.textContent = `${idx + 1}/${currentImages.length}`;
  }

  renderImageSwitchers();
  refreshLucideIcons();
}

export function renderImageSwitchers(): void {
  const heroPills = document.getElementById('hero-image-pills');
  const lightboxPills = document.getElementById('lightbox-image-pills');
  const badge = document.getElementById('hero-img-count-badge');

  if (!heroPills || !lightboxPills) return;
  heroPills.innerHTML = '';
  lightboxPills.innerHTML = '';

  if (currentImages.length <= 1) {
    heroPills.classList.add('hidden');
    lightboxPills.classList.add('hidden');
    if (badge) badge.classList.add('hidden');
    return;
  }

  heroPills.classList.remove('hidden');
  lightboxPills.classList.remove('hidden');
  if (badge) {
    badge.classList.remove('hidden');
    badge.textContent = `${state.selectedFlyerIdx + 1}/${currentImages.length}`;
  }

  currentImages.forEach((imgObj, i) => {
    const isActive = i === state.selectedFlyerIdx;
    const iconName = imgObj.type === 'timetable' ? 'calendar-days' : 'image';
    const label = imgObj.type === 'timetable' ? 'タイテ表' : 'フライヤー';

    // 1. Hero Pill
    const btnHero = document.createElement('button');
    btnHero.className = `flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-bold transition-all ${
      isActive 
        ? 'bg-rose-500 text-stone-950 shadow-sm' 
        : 'bg-stone-900 text-stone-400 hover:text-stone-200 border border-white/10'
    }`;
    btnHero.innerHTML = `<i data-lucide="${iconName}" class="w-3.5 h-3.5"></i><span>${label}</span>`;
    btnHero.onclick = (e) => {
      e.stopPropagation();
      switchSessionImage(i);
    };
    heroPills.appendChild(btnHero);

    // 2. Lightbox Pill
    const btnLightbox = document.createElement('button');
    btnLightbox.className = `flex items-center gap-1 px-3 py-1 rounded-full text-xs font-bold transition-all ${
      isActive 
        ? 'bg-rose-500 text-stone-950 shadow' 
        : 'bg-stone-800 text-stone-300 hover:text-white border border-white/20'
    }`;
    btnLightbox.innerHTML = `<i data-lucide="${iconName}" class="w-3.5 h-3.5"></i><span>${label}</span>`;
    btnLightbox.onclick = (e) => {
      e.stopPropagation();
      switchSessionImage(i);
    };
    lightboxPills.appendChild(btnLightbox);
  });
}

export function renderSameDaySwitcher(date: string): void {
  const container = document.getElementById('sameday-switcher-container');
  const pillsEl = document.getElementById('sameday-pills');
  if (!container || !pillsEl || !state.appData) return;

  const sameDaySessions = state.appData.sessions.filter(s => s.date === date);
  if (sameDaySessions.length <= 1) {
    container.classList.add('hidden');
    return;
  }

  container.classList.remove('hidden');
  pillsEl.innerHTML = '';

  sameDaySessions.forEach(session => {
    const isSelected = session.id === state.currentId;
    const btn = document.createElement('button');
    const label = session.session_name || session.title.slice(0, 10);

    btn.className = `px-3 py-1 rounded-xl text-xs font-bold transition-all flex items-center gap-1.5 ${
      isSelected 
        ? 'bg-gradient-to-r from-rose-500 to-rose-600 text-stone-950 shadow-sm' 
        : 'bg-stone-900 text-stone-400 hover:text-stone-200 border border-white/10'
    }`;
    btn.innerHTML = `<span>${label}</span><span class="text-[10px] font-mono opacity-80">${session.live.start_time}~</span>`;
    btn.onclick = () => {
      renderCockpit(session.id);
    };
    pillsEl.appendChild(btn);
  });
}

export function renderCockpit(id: string): void {
  if (!state.appData) return;
  const s = state.appData.sessions.find(x => x.id === id);
  if (!s) return;
  state.currentId = id;

  // Flyer & Timetable Multi-Image Setup
  currentImages = s.images && s.images.length > 0 ? s.images : [
    { type: 'flyer', label: 'フライヤー', url: s.image_url || '' }
  ];
  state.selectedFlyerIdx = 0;
  switchSessionImage(0);

  const titleEl = document.getElementById('live-title');
  if (titleEl) titleEl.textContent = s.title;

  const venueEl = document.getElementById('live-venue-name');
  if (venueEl) venueEl.textContent = s.venue.name;

  const dateHeaderEl = document.getElementById('live-date-header');
  if (dateHeaderEl) dateHeaderEl.textContent = `${s.date} (${s.day_of_week || ''})`;

  // Relative Day Badge (直近公演バッジ)
  const relativeDayEl = document.getElementById('live-relative-day');
  if (relativeDayEl) {
    const nextId = state.appData.current_next_id || (state.appData.sessions[0] && state.appData.sessions[0].id);
    if (id === nextId) {
      relativeDayEl.textContent = '★ 直近ライブ';
      relativeDayEl.className = 'text-xs font-bold text-rose-400 bg-rose-500/15 px-2 py-0.5 rounded border border-rose-500/30';
    } else {
      relativeDayEl.textContent = '';
      relativeDayEl.className = '';
    }
  }

  // 同日2回しピル（1部/2部）の更新 (コックピット内のみ)
  renderSameDaySwitcher(s.date);

  // 第1タブのラベル同期 ＆ 直近ライブに戻るボタンの表示切替
  const nextId = state.appData.current_next_id || (state.appData.sessions[0] && state.appData.sessions[0].id);
  const isNext = id === nextId;
  const navNext = document.getElementById('nav-next');
  if (navNext) {
    navNext.innerHTML = isNext 
      ? `<i data-lucide="sparkles" class="w-3.5 h-3.5"></i><span>直近ライブ</span>`
      : `<i data-lucide="calendar" class="w-3.5 h-3.5 text-rose-400"></i><span>公演詳細</span>`;
  }

  const returnTop = document.getElementById('subpage-return-bar');
  const returnBottom = document.getElementById('subpage-return-bottom');
  if (returnTop) {
    if (isNext) returnTop.classList.add('hidden');
    else returnTop.classList.remove('hidden');
  }
  if (returnBottom) {
    if (isNext) returnBottom.classList.add('hidden');
    else returnBottom.classList.remove('hidden');
  }

  // Live / Benefit Times
  const liveStart = document.getElementById('live-start');
  if (liveStart) liveStart.textContent = s.live.start_time;

  const liveEnd = document.getElementById('live-end');
  if (liveEnd) liveEnd.textContent = s.live.end_time || '';

  const liveStage = document.getElementById('live-stage');
  if (liveStage) liveStage.textContent = s.live.stage || 'ステージ';

  const benefitType = document.getElementById('benefit-type-label');
  if (benefitType) benefitType.textContent = `特典会 (${s.benefit.type || '終演後'})`;

  const benefitStart = document.getElementById('benefit-start');
  if (benefitStart) benefitStart.textContent = s.benefit.start_time;

  const benefitEnd = document.getElementById('benefit-end');
  if (benefitEnd) benefitEnd.textContent = s.benefit.end_time || '';

  const benefitBooth = document.getElementById('benefit-booth');
  if (benefitBooth) benefitBooth.textContent = s.benefit.booth || '';

  // Photo Rule
  const photoBox = document.getElementById('photo-box');
  const photoStatus = document.getElementById('photo-status');
  const photoDesc = document.getElementById('photo-desc');
  if (photoBox && photoStatus && photoDesc) {
    if (s.photo && s.photo.rule && s.photo.rule.trim() !== '' && (s.photo.rule.includes('曲目') || s.photo.rule.includes('『'))) {
      photoBox.classList.remove('hidden');
      photoStatus.textContent = "撮可：";
      photoDesc.textContent = s.photo.rule;
    } else {
      photoBox.classList.add('hidden');
    }
  }

  // Bonus
  const bonusDesc = document.getElementById('bonus-desc');
  if (bonusDesc) bonusDesc.textContent = s.admission_bonus || '特になし';

  // Special Notes
  const notesBox = document.getElementById('special-notes-box');
  const notesDesc = document.getElementById('special-notes-desc');
  if (notesBox && notesDesc) {
    if (s.special_notes) {
      notesBox.classList.remove('hidden');
      notesDesc.textContent = s.special_notes;
    } else {
      notesBox.classList.add('hidden');
    }
  }

  // Ticket Prices Preview
  const pricesBox = document.getElementById('ticket-prices-box');
  const pricesList = document.getElementById('ticket-prices-list');
  if (pricesBox && pricesList) {
    pricesList.innerHTML = '';
    if (s.ticket && s.ticket.prices && s.ticket.prices.length > 0) {
      pricesBox.classList.remove('hidden');
      s.ticket.prices.forEach(p => {
        const badge = document.createElement('span');
        badge.className = 'px-2 py-0.5 rounded-lg bg-stone-900 border border-white/10 text-xs font-medium text-stone-200';
        badge.innerHTML = `<span class="text-stone-400 mr-1">${p.tier}:</span><strong class="text-rose-300 font-display font-bold">${p.price}</strong>`;
        pricesList.appendChild(badge);
      });
    } else {
      pricesBox.classList.add('hidden');
    }
  }

  // Action Links & Map
  const mapLink = document.getElementById('venue-map-link') as HTMLAnchorElement | null;
  if (mapLink && s.venue.map_url) {
    mapLink.href = s.venue.map_url;
  }

  const ticketLink = document.getElementById('ticket-link') as HTMLAnchorElement | null;
  const ticketBtnText = document.getElementById('ticket-btn-text');
  if (ticketLink && ticketBtnText) {
    ticketLink.href = s.ticket?.url || '#';
    if (!s.ticket?.url || s.ticket.platform === '公式アナウンス待ち') {
      ticketBtnText.textContent = 'チケット告知待ち';
      ticketLink.className = 'flex items-center justify-center gap-1.5 py-2.5 rounded-xl bg-stone-800 text-stone-400 font-bold text-xs border border-white/10 transition-colors pointer-events-none';
    } else {
      ticketBtnText.textContent = 'チケット購入';
      ticketLink.className = 'flex items-center justify-center gap-1.5 py-2.5 rounded-xl bg-gradient-to-r from-rose-500 to-rose-600 hover:from-rose-400 hover:to-rose-500 text-stone-950 font-bold text-xs shadow-md transition-all';
    }
  }

  refreshLucideIcons();
}
