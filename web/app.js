/**
 * Ribbon Clock taite-board Application Logic
 */

let appData = null;
let currentId = null;
let previousTab = 'week';
const TODAY_STR = "2026-10-30"; // 基準日
let currentCalYear = 2026;
let currentCalMonth = 11; // 1-12

async function init() {
  // Purge any residual telemetry keys for privacy governance
  try {
    localStorage.removeItem('taite_visitor_id');
    localStorage.removeItem('taite_visit_count');
    localStorage.removeItem('taite_telemetry_logs');
    localStorage.removeItem('taite_telemetry_endpoint');
  } catch(e) {}

  let isFallback = false;
  try {
    // schedule.json を優先取得 (フォールバックとして mock_schedule.json も試行)
    let res = await fetch('schedule.json?t=' + Date.now()).catch(() => null);
    if (!res || !res.ok) {
      res = await fetch('mock_schedule.json?t=' + Date.now()).catch(() => null);
    }
    if (res && res.ok) {
      appData = await res.json();
      try {
        localStorage.setItem('taite_cached_schedule', JSON.stringify(appData));
      } catch(e) {}
    } else {
      throw new Error('Network response not ok');
    }
  } catch (e) {
    console.warn('Network load failed, falling back to local cache', e);
    try {
      const cached = localStorage.getItem('taite_cached_schedule');
      if (cached) {
        appData = JSON.parse(cached);
        isFallback = true;
      }
    } catch(err) {}
  }

  if (!appData) {
    console.error('Fatal: No schedule data available');
    return;
  }

  updateFreshnessStatus(isFallback);
  currentId = appData.current_next_id || appData.sessions[0].id;
  renderCockpit(currentId);
  renderUpcomingList();
  renderCalendar(currentCalYear, currentCalMonth);
  renderPromotion('large');
  initCarouselSync();
  lucide.createIcons();
}

// データ鮮度インジケーター (ADR-0008)
function updateFreshnessStatus(isFallback) {
  const badge = document.getElementById('freshness-indicator');
  const text = document.getElementById('freshness-text');
  if (!badge || !text) return;

  if (!appData.generated_at) {
    badge.classList.add('hidden');
    badge.classList.remove('flex');
    return;
  }

  const genTime = new Date(appData.generated_at).getTime();
  const now = Date.now();
  const ageHours = (now - genTime) / (1000 * 60 * 60);

  // キャッシュ使用時または24時間以上経過時に表示
  if (isFallback || ageHours > 24) {
    badge.classList.remove('hidden');
    badge.classList.add('flex');
    const d = new Date(genTime);
    const dateStr = `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
    text.textContent = isFallback ? `キャッシュ表示 (${dateStr})` : `最終更新: ${dateStr}`;
  } else {
    badge.classList.add('hidden');
    badge.classList.remove('flex');
  }
}

// Multiple Images State
let currentSessionImages = [];
let currentSessionImageIdx = 0;

function switchSessionImage(idx) {
  if (!currentSessionImages[idx]) return;
  currentSessionImageIdx = idx;
  const target = currentSessionImages[idx];

  const stripImg = document.getElementById('hero-strip-img');
  const lightboxImg = document.getElementById('lightbox-img');
  if (stripImg) stripImg.src = target.url;
  if (lightboxImg) lightboxImg.src = target.url;

  const badge = document.getElementById('hero-img-count-badge');
  if (badge && currentSessionImages.length > 1) {
    badge.textContent = `${idx + 1}/${currentSessionImages.length}`;
  }

  renderImageSwitchers();
  lucide.createIcons();
}

function renderImageSwitchers() {
  const heroPills = document.getElementById('hero-image-pills');
  const lightboxPills = document.getElementById('lightbox-image-pills');
  const badge = document.getElementById('hero-img-count-badge');

  if (!heroPills || !lightboxPills) return;
  heroPills.innerHTML = '';
  lightboxPills.innerHTML = '';

  if (currentSessionImages.length <= 1) {
    heroPills.classList.add('hidden');
    lightboxPills.classList.add('hidden');
    if (badge) badge.classList.add('hidden');
    return;
  }

  heroPills.classList.remove('hidden');
  lightboxPills.classList.remove('hidden');
  if (badge) {
    badge.classList.remove('hidden');
    badge.textContent = `${currentSessionImageIdx + 1}/${currentSessionImages.length}`;
  }

  currentSessionImages.forEach((imgObj, i) => {
    const isActive = i === currentSessionImageIdx;
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

// 1. Render Cockpit
function renderCockpit(id) {
  const s = appData.sessions.find(x => x.id === id);
  if (!s) return;
  currentId = id;

  // Flyer & Timetable Multi-Image Setup
  currentSessionImages = s.images && s.images.length > 0 ? s.images : [
    { type: 'flyer', label: 'フライヤー', url: s.image_url }
  ];
  currentSessionImageIdx = 0;
  switchSessionImage(0);

  document.getElementById('live-title').textContent = s.title;
  document.getElementById('live-venue-name').textContent = s.venue.name;
  document.getElementById('live-date-header').textContent = `${s.date} (${s.day_of_week})`;

  // Relative Day Badge (直近公演バッジ)
  const relativeDayEl = document.getElementById('live-relative-day');
  if (relativeDayEl) {
    const nextId = appData.current_next_id || (appData.sessions[0] && appData.sessions[0].id);
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
  const nextId = appData.current_next_id || (appData.sessions[0] && appData.sessions[0].id);
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

  // Telemetry Times: ライブ出番 (主役) / 特典会 (サブ)
  document.getElementById('live-start').textContent = s.live.start_time;
  document.getElementById('live-end').textContent = s.live.end_time;
  document.getElementById('live-stage').textContent = s.live.stage || 'ステージ';

  document.getElementById('benefit-type-label').textContent = `特典会 (${s.benefit.type})`;
  document.getElementById('benefit-start').textContent = s.benefit.start_time;
  document.getElementById('benefit-end').textContent = s.benefit.end_time;
  document.getElementById('benefit-booth').textContent = s.benefit.booth;

  // Photo Rule (直前告知等で曲名が明記されている場合のみ表示。推測・未発表時は完全非表示)
  const photoBox = document.getElementById('photo-box');
  if (s.photo && s.photo.rule && s.photo.rule.trim() !== '' && (s.photo.rule.includes('曲目') || s.photo.rule.includes('『'))) {
    photoBox.classList.remove('hidden');
    document.getElementById('photo-status').textContent = "撮可：";
    document.getElementById('photo-desc').textContent = s.photo.rule;
  } else {
    photoBox.classList.add('hidden');
  }

  // Bonus
  document.getElementById('bonus-desc').textContent = s.admission_bonus || '特になし';

  // Special Notes
  const notesBox = document.getElementById('special-notes-box');
  if (s.special_notes) {
    notesBox.classList.remove('hidden');
    document.getElementById('special-notes-desc').textContent = s.special_notes;
  } else {
    notesBox.classList.add('hidden');
  }

  // Ticket Prices Preview
  const pricesBox = document.getElementById('ticket-prices-box');
  const pricesList = document.getElementById('ticket-prices-list');
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

  // Action Links & Primary X Source
  document.getElementById('venue-map-link').href = s.venue.map_url;
  const ticketLink = document.getElementById('ticket-link');
  const ticketBtnText = document.getElementById('ticket-btn-text');
  ticketLink.href = s.ticket.url;

  if (s.ticket.platform === '公式アナウンス待ち' || !s.ticket.url) {
    ticketBtnText.textContent = 'チケット告知待ち';
    ticketLink.className = 'flex items-center justify-center gap-1.5 py-2.5 rounded-xl bg-stone-800 text-stone-400 font-bold text-xs border border-white/10 transition-colors';
  } else {
    ticketBtnText.textContent = 'チケット購入';
    ticketLink.className = 'flex items-center justify-center gap-1.5 py-2.5 rounded-xl bg-gradient-to-r from-rose-500 to-rose-600 hover:from-rose-400 hover:to-rose-500 text-stone-950 font-bold text-xs shadow-md transition-all';
  }

  lucide.createIcons();
}

function renderSameDaySwitcher(date) {
  const sameDay = appData.sessions.filter(x => x.date === date);
  const bar = document.getElementById('session-pills-bar');
  bar.innerHTML = '';

  if (sameDay.length <= 1) {
    bar.classList.add('hidden');
    return;
  }

  bar.classList.remove('hidden');
  sameDay.forEach(item => {
    const btn = document.createElement('button');
    const isActive = item.id === currentId;
    btn.className = `px-3 py-1.5 rounded-lg text-xs font-display font-semibold transition-all shrink-0 ${
      isActive 
        ? 'bg-rose-500 text-stone-950 font-bold shadow-md' 
        : 'bg-stone-900 text-stone-400 border border-white/5'
    }`;
    const label = item.session_name.includes('昼') ? '☀️ 昼公演' : (item.session_name.includes('夜') ? '🌙 夜公演' : item.session_name);
    btn.textContent = `${label} (${item.live.start_time}〜)`;
    btn.onclick = () => renderCockpit(item.id);
    bar.appendChild(btn);
  });
}

// 2. Render Upcoming Schedule List (目が滑らないカレンダーバッジUI)
function renderUpcomingList() {
  const container = document.getElementById('upcoming-list');
  container.innerHTML = '';

  // 今後の公演のみ抽出
  const upcoming = appData.sessions.filter(s => s.date >= TODAY_STR);
  
  const countBadge = document.getElementById('upcoming-count-badge');
  if (countBadge) countBadge.textContent = `${upcoming.length}公演`;

  upcoming.forEach(s => {
    const card = document.createElement('div');
    card.className = 'soft-card rounded-2xl p-3 flex items-center gap-3.5 cursor-pointer hover:border-rose-400/40 transition-all shadow-md';
    card.onclick = () => openSessionDetail(s.id, 'week');

    // 日付・月・曜日の抽出
    const [, mStr, dStr] = s.date.split('-');
    const monthNum = parseInt(mStr, 10);
    const dayNum = parseInt(dStr, 10);
    
    // 曜日カラー
    let dayColor = 'text-stone-400';
    if (s.day_of_week === '土') dayColor = 'text-blue-400';
    if (s.day_of_week === '日' || s.day_of_week.includes('祝')) dayColor = 'text-rose-400';

    const sameDayCount = appData.sessions.filter(x => x.date === s.date).length;
    const sessionTag = sameDayCount > 1 ? (s.session_name.includes('昼') ? ' [昼]' : ' [夜]') : '';

    card.innerHTML = `
      <!-- カレンダー風・日付アンカーブロック (目が滑らない) -->
      <div class="w-13 h-14 w-[52px] h-[56px] rounded-xl bg-stone-950 border border-white/10 flex flex-col items-center justify-center shrink-0 shadow-inner">
        <span class="text-[10px] font-bold text-stone-400 leading-none">${monthNum}月</span>
        <span class="text-xl font-display font-black text-white leading-tight my-0.5">${dayNum}</span>
        <span class="text-[10px] font-bold ${dayColor} leading-none">${s.day_of_week}</span>
      </div>

      <!-- イベント情報本体 -->
      <div class="flex-1 min-w-0 space-y-1">
        <div class="flex items-center justify-between text-xs">
          <span class="text-stone-300 font-semibold truncate flex items-center gap-1">
            <i data-lucide="map-pin" class="w-3 h-3 text-rose-400 shrink-0"></i>
            <span class="truncate">${s.venue.name}</span>
          </span>
          ${sessionTag ? `<span class="px-1.5 py-0.2 rounded bg-rose-500/20 text-rose-300 font-bold text-[10px] shrink-0">${sessionTag}</span>` : ''}
        </div>

        <div class="text-sm font-extrabold text-white leading-snug line-clamp-2">${s.title}</div>

        <div class="text-xs flex items-center justify-between pt-1 border-t border-white/5 text-stone-300">
          <span class="flex items-center gap-1">
            <span class="text-rose-400 font-bold">出番</span>
            <strong class="text-white font-display font-bold">${s.live.start_time}〜</strong>
          </span>
          <span class="flex items-center gap-1 text-stone-400">
            <span>特典会</span>
            <strong class="text-stone-200 font-display">${s.benefit.start_time}〜</strong>
          </span>
        </div>
      </div>
    `;

    container.appendChild(card);
  });
  lucide.createIcons();
}

// 3. Render Calendar (月送りで過去・未来を無限に遡れる)
function changeCalendarMonth(delta) {
  currentCalMonth += delta;
  if (currentCalMonth > 12) {
    currentCalMonth = 1;
    currentCalYear++;
  } else if (currentCalMonth < 1) {
    currentCalMonth = 12;
    currentCalYear--;
  }
  renderCalendar(currentCalYear, currentCalMonth);
}

function renderCalendar(year, month) {
  const headerTitle = document.getElementById('calendar-header-title');
  if (headerTitle) headerTitle.textContent = `${year}年 ${month}月`;

  const prevLabel = document.getElementById('prev-month-label');
  const nextLabel = document.getElementById('next-month-label');
  if (prevLabel) prevLabel.textContent = `${month === 1 ? 12 : month - 1}月`;
  if (nextLabel) nextLabel.textContent = `${month === 12 ? 1 : month + 1}月`;

  const grid = document.getElementById('calendar-cells');
  grid.innerHTML = '';

  // 月の日数と1日の曜日インデックスを取得
  const firstDayIndex = new Date(year, month - 1, 1).getDay(); // 0 = Sun
  const daysInMonth = new Date(year, month, 0).getDate();

  // 前月の空白埋め
  for (let i = 0; i < firstDayIndex; i++) {
    const blank = document.createElement('div');
    blank.className = 'py-2.5 text-stone-700';
    grid.appendChild(blank);
  }

  // 日付セルの描画
  for (let day = 1; day <= daysInMonth; day++) {
    const monthStr = String(month).padStart(2, '0');
    const dayStr = String(day).padStart(2, '0');
    const dateStr = `${year}-${monthStr}-${dayStr}`;
    const sessions = appData ? appData.sessions.filter(s => s.date === dateStr) : [];
    const hasLive = sessions.length > 0;

    const cell = document.createElement('button');
    cell.className = `py-2 rounded-xl flex flex-col items-center justify-center transition-all ${
      hasLive 
        ? 'bg-rose-950/50 border-2 border-rose-500/60 text-white font-extrabold shadow-sm scale-105' 
        : 'bg-stone-900/30 text-stone-500 hover:text-stone-400'
    }`;

    cell.innerHTML = `
      <span class="text-xs font-display">${day}</span>
      ${hasLive ? `<span class="w-1.5 h-1.5 rounded-full mt-1 bg-rose-400 animate-pulse"></span>` : '<span class="h-1.5 mt-1"></span>'}
    `;

    if (hasLive) {
      cell.onclick = () => renderCalendarDetail(dateStr, sessions);
    }
    grid.appendChild(cell);
  }

  // 詳細カードのクリア
  const detailContainer = document.getElementById('calendar-day-detail');
  if (detailContainer) {
    detailContainer.innerHTML = '<div class="text-xs text-stone-400 text-center py-2">日付をタップすると詳細が表示されます</div>';
  }
  lucide.createIcons();
}

function renderCalendarDetail(dateStr, sessions) {
  const container = document.getElementById('calendar-day-detail');
  container.innerHTML = '';

  sessions.forEach(s => {
    const div = document.createElement('div');
    div.className = 'soft-card rounded-xl p-3 space-y-1.5 border-l-4 border-l-rose-500 cursor-pointer hover:border-rose-400/40 transition-all';
    div.onclick = () => openSessionDetail(s.id, 'month');

    const sessionTag = sessions.length > 1 ? (s.session_name.includes('昼') ? ' [昼公演]' : ' [夜公演]') : '';
    div.innerHTML = `
      <div class="flex items-center justify-between text-xs">
        <span class="font-display font-bold text-rose-300">${s.date}${sessionTag}</span>
        <span class="text-stone-300 text-xs">${s.venue.name}</span>
      </div>
      <div class="text-sm font-bold text-white pt-0.5">${s.title}</div>
      <div class="text-xs font-display text-stone-300 flex justify-between pt-1.5 border-t border-white/5">
        <span>出演: <strong class="text-white font-bold">${s.live.start_time}〜</strong></span>
        <span>物販: <strong class="text-white font-bold">${s.benefit.start_time}〜</strong> (${s.benefit.type})</span>
      </div>
    `;
    container.appendChild(div);
  });
  lucide.createIcons();
}

// 4. Render Promotion (大・中・小 バリエーション対応コンポーネント)
function renderPromotion(size = 'large') {
  const container = document.getElementById('promo-container');
  if (!container || !appData || !appData.promotions) return;
  const promo = appData.promotions.find(p => p.id === 'promo-ayyu-birthday') || appData.promotions[0];
  if (!promo) return;

  if (size === 'large') {
    container.innerHTML = `
      <div class="rounded-2xl overflow-hidden soft-card border-2 border-rose-500/50 bg-stone-900 shadow-xl">
        <div class="w-full h-44 overflow-hidden bg-stone-950">
          <img src="${promo.image_url}" referrerpolicy="no-referrer" onerror="handleImageFallback(this)" alt="${promo.title}" class="w-full h-full object-cover object-center">
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
          <img src="${promo.image_url}" referrerpolicy="no-referrer" onerror="handleImageFallback(this)" alt="${promo.title}" class="w-16 h-16 rounded-xl object-cover shrink-0 border border-rose-400/30 shadow">
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
  lucide.createIcons();
}

// Image Fallback Handler
function handleImageFallback(img) {
  img.onerror = null;
  img.src = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='400' height='200' viewBox='0 0 400 200'%3E%3Crect width='100%25' height='100%25' fill='%231c1917'/%3E%3Ctext x='50%25' y='50%25' fill='%2378716c' font-family='sans-serif' font-size='14' text-anchor='middle' dominant-baseline='middle'%3Eフライヤー画像準備中%3C/text%3E%3C/svg%3E";
}

// 5. Navigation & History Helpers
function handleLogoClick() {
  // 1. リボンアイコンの弾けるマイクロアニメーション
  const icon = document.getElementById('logo-icon');
  if (icon) {
    icon.classList.remove('animate-ribbon-pop');
    void icon.offsetWidth;
    icon.classList.add('animate-ribbon-pop');
  }

  // 2. コックピットカードのリフレッシュ・アニメーション
  const cockpit = document.getElementById('tab-content-next');
  if (cockpit) {
    cockpit.classList.remove('animate-card-refresh');
    void cockpit.offsetWidth;
    cockpit.classList.add('animate-card-refresh');
  }

  // 3. 直近ライブへジャンプ
  goToNextLive();
}

function openSessionDetail(id, fromTab = 'week') {
  previousTab = fromTab;
  renderCockpit(id);
  const nextSlide = document.getElementById('tab-content-next');
  if (nextSlide) nextSlide.scrollTo({ top: 0, behavior: 'smooth' });
  setTab('next');
}

function goToNextLive() {
  const nextId = appData.current_next_id || appData.sessions[0].id;
  renderCockpit(nextId);
  const nextSlide = document.getElementById('tab-content-next');
  if (nextSlide) nextSlide.scrollTo({ top: 0, behavior: 'smooth' });
  setTab('next');
}

function goBackToList() {
  setTab(previousTab || 'week');
}

// 第1タブのクリックハンドラ: 状態を勝手にリセットせず保持。再タップ時のみ直近ライブへリセット
function handleFirstTabClick() {
  const nextId = appData.current_next_id || (appData.sessions[0] && appData.sessions[0].id);
  const nextSlide = document.getElementById('tab-content-next');
  
  // すでに第1タブを表示中で、かつ別日程を閲覧中の場合のみ、タップで直近ライブへリセット
  if (currentTabName === 'next' && currentId !== nextId) {
    renderCockpit(nextId);
    if (nextSlide) nextSlide.scrollTo({ top: 0, behavior: 'smooth' });
    return;
  }
  
  // 通常のタブ遷移時は、閲覧中の状態（選んだ公演）をそのまま維持してスムーズに移動
  setTab('next');
}

// Tab Order & Active State
let currentTabName = 'next';
const tabCycle = ['next', 'week', 'month'];

// Tab Switching via Bottom Nav
function setTab(tab) {
  currentTabName = tab;
  const carousel = document.getElementById('views-carousel');
  const idx = tabCycle.indexOf(tab);
  if (carousel && idx !== -1) {
    const slideWidth = carousel.clientWidth;
    // タブボタンタップ時はスムーズにスライド位置へ移動 (インジケーターも自然に同期)
    carousel.scrollTo({
      left: idx * slideWidth,
      behavior: 'smooth'
    });
  }
  // カレンダーまたは詳細への移動時は、確実にトップが見えるように保護
  const targetSlide = document.getElementById(`tab-content-${tab}`);
  if (tab === 'month' && targetSlide && targetSlide.scrollTop > 0) {
    targetSlide.scrollTo({ top: 0, behavior: 'auto' });
  }
  updateNavHighlight(tab);
}

function updateNavHighlight(activeTab) {
  tabCycle.forEach(t => {
    const nav = document.getElementById(`nav-${t}`);
    if (!nav) return;
    if (t === activeTab) {
      nav.className = 'relative z-10 flex-1 py-2.5 rounded-xl text-center font-bold text-stone-950 transition-colors flex items-center justify-center gap-1.5';
    } else {
      nav.className = 'relative z-10 flex-1 py-2.5 rounded-xl text-center font-bold text-stone-400 hover:text-stone-200 transition-colors flex items-center justify-center gap-1.5';
    }
  });
}

// Carousel Scroll Listener: 指スワイプ中の移動量にフッターのタブインジケーターが1:1でリアルタイム追従
function initCarouselSync() {
  const carousel = document.getElementById('views-carousel');
  const indicator = document.getElementById('nav-indicator');
  if (!carousel) return;

  carousel.addEventListener('scroll', () => {
    const slideWidth = carousel.clientWidth;
    if (!slideWidth) return;

    // 横スクロール進行度 (0.0: 直近ライブ, 1.0: 全日程, 2.0: カレンダー)
    const progress = carousel.scrollLeft / slideWidth;
    
    // 1. フッターのピルインジケーターをスワイプ量に完全同期させて移動
    if (indicator) {
      indicator.style.transform = `translateX(${progress * 100}%)`;
    }

    // 2. 最も近いタブの文字色ハイライトをリアルタイム更新
    const nearestIdx = Math.max(0, Math.min(tabCycle.length - 1, Math.round(progress)));
    const detectedTab = tabCycle[nearestIdx];
    if (currentTabName !== detectedTab) {
      currentTabName = detectedTab;
      updateNavHighlight(detectedTab);
    }
  }, { passive: true });
}

// Lightbox
function openLightbox() {
  document.getElementById('lightbox-modal').classList.add('active');
}
function closeLightbox() {
  document.getElementById('lightbox-modal').classList.remove('active');
}

window.addEventListener('DOMContentLoaded', init);
