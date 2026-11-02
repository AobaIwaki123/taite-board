/**
 * Group & Multi-Tenant App Configuration
 * Enables Ribbon Clock to scale across multiple idol groups and subdomains (*.iorin.net)
 * with isolated data endpoints, themes, and storage partitions.
 */

export interface GroupTheme {
  primary: string; // Tailwind color token or CSS color
  accent: string;
}

export interface GroupConfig {
  id: string;
  name: string;
  shortName: string;
  logoText?: string;
  dataUrl: string;
  fallbackDataUrl: string;
  storagePrefix: string;
  theme: GroupTheme;
}

export const DEFAULT_GROUP_ID = 'rincent';

export const REGISTERED_GROUPS: Record<string, GroupConfig> = {
  rincent: {
    id: 'rincent',
    name: 'リボンクロック',
    shortName: 'RiNCENT#',
    logoText: '🎀',
    dataUrl: 'schedule.json',
    fallbackDataUrl: 'mock_schedule.json',
    storagePrefix: 'ribonclock_',
    theme: {
      primary: 'rose',
      accent: 'sky',
    },
  },
  // 拡張用テンプレート (新規グループ追加時はここに設定を追加し data/groups/<id>/schedule.json を配置)
  sample: {
    id: 'sample',
    name: 'アイドルタイテ',
    shortName: 'Sample',
    logoText: '✨',
    dataUrl: 'data/groups/sample/schedule.json',
    fallbackDataUrl: 'schedule.json',
    storagePrefix: 'taite_sample_',
    theme: {
      primary: 'emerald',
      accent: 'sky',
    },
  },
};

/**
 * Resolves active group configuration dynamically:
 * 1. Checks URL search query (?group=xxx)
 * 2. Checks subdomain (e.g. rincent.iorin.net -> rincent)
 * 3. Falls back to DEFAULT_GROUP_ID ('rincent')
 */
export function resolveCurrentGroup(): GroupConfig {
  if (typeof window !== 'undefined' && window.location) {
    try {
      // 1. Query parameter
      const params = new URLSearchParams(window.location.search);
      const groupParam = params.get('group');
      if (groupParam && REGISTERED_GROUPS[groupParam]) {
        return REGISTERED_GROUPS[groupParam];
      }

      // 2. Subdomain check (e.g. <subdomain>.iorin.net)
      const hostname = window.location.hostname;
      const parts = hostname.split('.');
      if (parts.length >= 3) {
        const sub = parts[0].toLowerCase();
        if (REGISTERED_GROUPS[sub]) {
          return REGISTERED_GROUPS[sub];
        }
      }
    } catch {
      // Fallback on any parser error
    }
  }

  return REGISTERED_GROUPS[DEFAULT_GROUP_ID];
}
