import { GroupConfig } from '../config/group';

export function updatePageMetadata(group: GroupConfig) {
  if (typeof document === 'undefined') return;

  const title = `${group.name}｜ライブ・タイテ情報`;
  const desc = `${group.name}（${group.shortName}）のライブ出番時間、特典会、会場、チケット、タイムテーブルを確認できるボード。`;

  document.title = title;

  // Meta description
  const metaDesc = document.querySelector('meta[name="description"]');
  if (metaDesc) metaDesc.setAttribute('content', desc);

  // Apple mobile web app title
  const appleTitle = document.querySelector('meta[name="apple-mobile-web-app-title"]');
  if (appleTitle) appleTitle.setAttribute('content', group.shortName);

  // OGP
  const ogTitle = document.querySelector('meta[property="og:title"]');
  if (ogTitle) ogTitle.setAttribute('content', title);

  const ogSiteName = document.querySelector('meta[property="og:site_name"]');
  if (ogSiteName) ogSiteName.setAttribute('content', group.name);

  const ogDesc = document.querySelector('meta[property="og:description"]');
  if (ogDesc) ogDesc.setAttribute('content', desc);

  // Twitter
  const twTitle = document.querySelector('meta[name="twitter:title"]');
  if (twTitle) twTitle.setAttribute('content', title);

  const twDesc = document.querySelector('meta[name="twitter:description"]');
  if (twDesc) twDesc.setAttribute('content', desc);
}
