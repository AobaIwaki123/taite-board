export interface IntakeTicketPrice {
  tier: string;
  price: string;
  amount?: number;
}

function formatYenAmount(amount: number): string {
  return `¥${amount.toLocaleString('en-US')}`;
}

export function formatTicketPricesInput(prices?: IntakeTicketPrice[]): string {
  if (!prices || prices.length === 0) return '';
  return prices
    .map((p) => {
      const tier = (p.tier || '').trim();
      let price = (p.price || '').trim();
      if (!price && typeof p.amount === 'number') {
        price = p.amount === 0 ? '無料' : formatYenAmount(p.amount);
      }
      if (tier && price) return `${tier} ${price}`;
      return tier || price;
    })
    .filter(Boolean)
    .join(' / ');
}

export function parseTicketPricesInput(raw: string): IntakeTicketPrice[] {
  const normalized = raw
    .replace(/[０-９]/g, (c) => String.fromCharCode(c.charCodeAt(0) - 0xfee0))
    .replace(/￥/g, '¥')
    .replace(/／/g, '/')
    .replace(/（/g, '(')
    .replace(/）/g, ')')
    .replace(/＋/g, '+')
    .trim();
  if (!normalized) return [];

  const segments = normalized
    .split(/\s*(?:\/|\n|・)+\s*/)
    .map((s) => s.trim())
    .filter(Boolean);

  const results: IntakeTicketPrice[] = [];
  for (const seg of segments) {
    const tokenMatch = seg.match(
      /(\+\s*)?(¥\s*[0-9]{1,3}(?:,[0-9]{3})+|[0-9]{1,3}(?:,[0-9]{3})+\s*円|¥\s*[0-9]+|[0-9]+\s*円|無料)/,
    );
    if (!tokenMatch || tokenMatch.index === undefined) continue;

    const fullToken = tokenMatch[0];
    const startIdx = tokenMatch.index;
    const endIdx = startIdx + fullToken.length;
    const tierRaw = seg.slice(0, startIdx).trim().replace(/^[:：\-\s]+|[:：\-\s]+$/g, '');
    const suffixRaw = seg.slice(endIdx).trim();

    if (fullToken.includes('無料')) {
      results.push({
        tier: tierRaw || '通常',
        price: suffixRaw ? `無料 ${suffixRaw}` : '無料',
        amount: 0,
      });
      continue;
    }

    const digits = fullToken.replace(/[^0-9]/g, '');
    if (!digits) continue;
    const amount = parseInt(digits, 10);
    if (Number.isNaN(amount)) continue;

    const isPlus = Boolean(tokenMatch[1]);
    const formattedYen = `${isPlus ? '+' : ''}${formatYenAmount(amount)}`;
    results.push({
      tier: tierRaw || (isPlus ? '当日' : '通常'),
      price: suffixRaw ? `${formattedYen} ${suffixRaw}` : formattedYen,
      amount,
    });
  }
  return results;
}
