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
  heroImage?: string;
  dataUrl: string;
  fallbackDataUrl: string;
  storagePrefix: string;
  theme: GroupTheme;
}

export const DEFAULT_GROUP_ID = 'ribonclock';

// 後方互換エイリアスマップ (旧識別子 rincent などを ribonclock にマッピング)
export const GROUP_ALIASES: Record<string, string> = {
  rincent: 'ribonclock',
};

export const REGISTERED_GROUPS: Record<string, GroupConfig> = {
  ribonclock: {
    id: 'ribonclock',
    name: 'リボンクロック',
    shortName: 'リボクロ',
    logoText: '🎀',
    heroImage: 'https://pbs.twimg.com/media/HVi3dC9bsAAinY6?format=jpg&name=large',
    dataUrl: 'data/groups/ribonclock/schedule.json',
    fallbackDataUrl: 'schedule.json',
    storagePrefix: 'ribonclock_',
    theme: {
      primary: 'rose',
      accent: 'sky',
    },
  },
  dollykiss: {
    id: 'dollykiss',
    name: 'Dolly Kiss',
    shortName: 'ドリキス',
    logoText: '🎀',
    heroImage: 'https://pbs.twimg.com/media/HVumF5haAAAHcXU.jpg?name=large',
    dataUrl: 'data/groups/dollykiss/schedule.json',
    fallbackDataUrl: 'schedule.json',
    storagePrefix: 'dollykiss_',
    theme: {
      primary: 'pink',
      accent: 'purple',
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
 * Resolves a group identifier, taking backward compatibility aliases into account.
 */
export function resolveGroupId(rawId?: string | null): string | undefined {
  if (!rawId) return undefined;
  const normalized = rawId.toLowerCase();
  const aliased = GROUP_ALIASES[normalized] || normalized;
  return REGISTERED_GROUPS[aliased] ? aliased : undefined;
}

/**
 * Resolves active group configuration dynamically:
 * 1. Checks URL search query (?group=xxx)
 * 2. Checks subdomain (e.g. ribonclock.iorin.net or legacy rincent.iorin.net -> ribonclock)
 * 3. Falls back to DEFAULT_GROUP_ID ('ribonclock')
 */
export function resolveCurrentGroup(): GroupConfig {
  if (typeof window !== 'undefined' && window.location) {
    try {
      // 1. Query parameter
      const params = new URLSearchParams(window.location.search);
      const groupParam = params.get('group');
      const resolvedFromParam = resolveGroupId(groupParam);
      if (resolvedFromParam && REGISTERED_GROUPS[resolvedFromParam]) {
        return REGISTERED_GROUPS[resolvedFromParam];
      }

      // 2. Subdomain check (e.g. <subdomain>.iorin.net)
      const hostname = window.location.hostname;
      const parts = hostname.split('.');
      if (parts.length >= 3) {
        const sub = parts[0].toLowerCase();
        const resolvedFromSub = resolveGroupId(sub);
        if (resolvedFromSub && REGISTERED_GROUPS[resolvedFromSub]) {
          return REGISTERED_GROUPS[resolvedFromSub];
        }
      }
    } catch {
      // Fallback on any parser error
    }
  }

  return REGISTERED_GROUPS[DEFAULT_GROUP_ID];
}

/**
 * Determines whether the app should render the multi-tenant portal view:
 * 1. Hostname is taite.iorin.net
 * 2. Query param ?view=portal is present
 * 3. Hostname has no group subdomain (e.g. localhost) and ?group= is not provided
 */
export function isPortalMode(urlLocation?: { hostname: string; search: string }): boolean {
  const loc = urlLocation || (typeof window !== 'undefined' ? window.location : null);
  if (!loc) return false;

  const hostname = (loc.hostname || '').toLowerCase();
  if (hostname === 'taite.iorin.net') {
    return true;
  }

  const searchParams = new URLSearchParams(loc.search || '');
  if (searchParams.get('view') === 'portal') {
    return true;
  }

  const hasGroupParam = Boolean(searchParams.get('group'));
  if (!hasGroupParam) {
    const parts = hostname.split('.');
    const sub = parts.length >= 3 ? parts[0].toLowerCase() : '';
    const hasRegisteredSubdomain = Boolean(resolveGroupId(sub));
    if (!hasRegisteredSubdomain) {
      return true;
    }
  }

  return false;
}
