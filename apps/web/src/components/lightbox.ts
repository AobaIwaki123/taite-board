/**
 * Lightbox Modal Component
 */

export function openLightbox(): void {
  const modal = document.getElementById('lightbox-modal');
  if (modal) {
    modal.classList.add('active');
  }
}

export function closeLightbox(): void {
  const modal = document.getElementById('lightbox-modal');
  if (modal) {
    modal.classList.remove('active');
  }
}

export function initLightboxEvents(): void {
  const modal = document.getElementById('lightbox-modal');
  if (!modal) return;

  modal.addEventListener('click', (e) => {
    // モーダル背景クリック時のみ閉じる
    if (e.target === modal) {
      closeLightbox();
    }
  });

  const closeBtn = document.getElementById('lightbox-close-btn');
  if (closeBtn) {
    closeBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      closeLightbox();
    });
  }

  // Escapeキーで閉じる
  window.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      closeLightbox();
    }
  });
}
