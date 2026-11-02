import { chromium } from 'playwright';
import fs from 'node:fs/promises';
import path from 'node:path';

const TARGET_ACCOUNT = process.env.TARGET_ACCOUNT || 'Ribbon_Clock';
const OUTPUT_DIR = process.env.OUTPUT_DIR || 'data/raw/runs';
const MAX_POSTS = parseInt(process.env.MAX_POSTS || '10', 10);

async function crawl() {
  console.log(`Starting headless crawler for @${TARGET_ACCOUNT} (max: ${MAX_POSTS})`);

  const browser = await chromium.launch({
    headless: process.env.HEADLESS !== 'false',
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--disable-blink-features=AutomationControlled'
    ]
  });

  const context = await browser.newContext({
    userAgent:
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36',
    viewport: { width: 1280, height: 800 },
    locale: 'ja-JP',
    timezoneId: 'Asia/Tokyo'
  });

  await context.addInitScript(() => {
    Object.defineProperty(navigator, 'webdriver', {
      get: () => undefined
    });
  });

  if (process.env.X_AUTH_TOKEN) {
    console.log('Injecting auth_token session cookie');
    await context.addCookies([
      {
        name: 'auth_token',
        value: process.env.X_AUTH_TOKEN,
        domain: '.x.com',
        path: '/',
        httpOnly: true,
        secure: true,
        sameSite: 'None'
      }
    ]);
    if (process.env.X_CT0) {
      await context.addCookies([
        {
          name: 'ct0',
          value: process.env.X_CT0,
          domain: '.x.com',
          path: '/',
          secure: true,
          sameSite: 'Lax'
        }
      ]);
    }
  }

  const page = await context.newPage();
  const capturedPosts = [];

  // GraphQL API Interception (ADR-0018)
  page.on('response', async (response) => {
    const url = response.url();
    if (url.includes('/graphql/') || url.includes('/api/')) {
      console.log(`[HTTP ${response.status()}] ${url.slice(0, 100)}`);
    }
    if (url.includes('/graphql/') && (url.includes('UserMedia') || url.includes('UserTweets') || url.includes('UserByScreenName') || url.includes('TweetDetail'))) {
      try {
        const json = await response.json();
        const instructions =
          json?.data?.user?.result?.timeline_v2?.timeline?.instructions ||
          json?.data?.user?.result?.timeline?.instructions ||
          [];

        for (const inst of instructions) {
          const entries = inst.entries || [];
          for (const entry of entries) {
            const tweetResult = entry?.content?.itemContent?.tweet_results?.result;
            if (!tweetResult) continue;

            const legacy = tweetResult.legacy || tweetResult.tweet?.legacy;
            const restId = tweetResult.rest_id || tweetResult.tweet?.rest_id;
            if (!legacy || !restId) continue;

            const mediaEntities = legacy.extended_entities?.media || legacy.entities?.media || [];
            const images = mediaEntities
              .filter((m) => m.type === 'photo')
              .map((m) => m.media_url_https);

            capturedPosts.push({
              schema_version: 1,
              source: 'x',
              source_id: restId,
              url: `https://x.com/${TARGET_ACCOUNT}/status/${restId}`,
              author: tweetResult.core?.user_results?.result?.legacy?.name || TARGET_ACCOUNT,
              author_url: `https://x.com/${TARGET_ACCOUNT}`,
              text: legacy.full_text || '',
              images: images.length > 0 ? images : null,
              fetched_at: new Date().toISOString()
            });
          }
        }
      } catch {
        // Silently skip non-JSON or unrelated GraphQL responses
      }
    }
  });

  const targetURL = `https://x.com/${TARGET_ACCOUNT}/media`;
  console.log(`Navigating to ${targetURL} ...`);
  try {
    const res = await page.goto(targetURL, {
      waitUntil: 'domcontentloaded',
      timeout: 30000
    });
    console.log(`HTTP Status: ${res?.status()}, URL: ${page.url()}, Title: ${await page.title()}`);
    const html = await page.content();
    console.log(`HTML Length: ${html.length}, Preview: ${html.slice(0, 200).replace(/\n/g, ' ')}`);
  } catch (err) {
    console.log(`Navigation error: ${err.message}`);
  }

  await browser.close();

  // Deduplicate captured posts by source_id
  const uniquePosts = [];
  const seenIds = new Set();
  for (const p of capturedPosts) {
    if (!seenIds.has(p.source_id)) {
      seenIds.add(p.source_id);
      uniquePosts.push(p);
      if (uniquePosts.length >= MAX_POSTS) break;
    }
  }

  console.log(`Captured ${uniquePosts.length} post(s) from @${TARGET_ACCOUNT}`);

  if (uniquePosts.length === 0) {
    console.log('No posts captured in this cycle. Exiting.');
    return;
  }

  // Format run_id: YYYYMMDDTHHMMSS+0900 (JST)
  const now = new Date();
  const jstOffset = 9 * 60;
  const jstDate = new Date(now.getTime() + (jstOffset + now.getTimezoneOffset()) * 60000);
  const pad = (n) => String(n).padStart(2, '0');
  const runId = `${jstDate.getFullYear()}${pad(jstDate.getMonth() + 1)}${pad(jstDate.getDate())}T${pad(jstDate.getHours())}${pad(jstDate.getMinutes())}${pad(jstDate.getSeconds())}+0900`;

  const runDir = path.join(OUTPUT_DIR, runId);
  const postsDir = path.join(runDir, 'posts');
  await fs.mkdir(postsDir, { recursive: true });

  const savedPaths = [];
  for (const p of uniquePosts) {
    const postFile = path.join(postsDir, `${p.source_id}.json`);
    await fs.writeFile(postFile, JSON.stringify(p, null, 2) + '\n');
    savedPaths.push(postFile);
  }

  const manifest = {
    schema_version: 1,
    run_id: runId,
    source: 'x',
    fetched_at: now.toISOString(),
    input_count: uniquePosts.length,
    success_count: uniquePosts.length,
    failed_count: 0,
    saved_posts: savedPaths
  };

  const manifestFile = path.join(runDir, 'manifest.json');
  await fs.writeFile(manifestFile, JSON.stringify(manifest, null, 2) + '\n');
  console.log(`Run ${runId} saved with ${savedPaths.length} post(s) in ${runDir}`);
}

crawl().catch((err) => {
  console.error('Crawler error:', err);
  process.exit(1);
});
