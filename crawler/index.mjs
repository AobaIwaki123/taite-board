import { chromium } from 'playwright';
import fs from 'node:fs/promises';
import path from 'node:path';

// Dynamic Group Configuration (ADR-0018 / Multi-Group Extension)
let TARGET_ACCOUNT = process.env.TARGET_ACCOUNT || 'Ribbon_Clock';
let OUTPUT_DIR = process.env.OUTPUT_DIR || 'data/raw/runs';
let CHECKPOINT_FILE = process.env.CHECKPOINT_FILE || '';
const GROUP = process.env.GROUP || '';

try {
  const groupsData = await fs.readFile('data/groups.json', 'utf-8').catch(() => null);
  if (groupsData) {
    const config = JSON.parse(groupsData);
    if (GROUP && config.groups && config.groups[GROUP]) {
      const g = config.groups[GROUP];
      if (!process.env.TARGET_ACCOUNT && g.x_account) {
        TARGET_ACCOUNT = g.x_account;
      }
      if (!process.env.OUTPUT_DIR && GROUP !== 'rincent') {
        OUTPUT_DIR = `data/groups/${GROUP}/runs`;
      }
      if (!CHECKPOINT_FILE && GROUP !== 'rincent') {
        CHECKPOINT_FILE = OUTPUT_DIR.startsWith('/var/log/ribonclock')
          ? `/var/log/ribonclock/groups/${GROUP}/checkpoint.json`
          : `data/groups/${GROUP}/checkpoint.json`;
      }
    }
  }
} catch {
  // Ignore fallback errors
}

if (!CHECKPOINT_FILE) {
  CHECKPOINT_FILE = OUTPUT_DIR.startsWith('/var/log/ribonclock')
    ? '/var/log/ribonclock/checkpoint.json'
    : 'data/checkpoint.json';
}

const MAX_POSTS = parseInt(process.env.MAX_POSTS || '10', 10);

async function loadCheckpoint() {
  try {
    const data = await fs.readFile(CHECKPOINT_FILE, 'utf-8');
    return JSON.parse(data);
  } catch {
    return null;
  }
}

async function saveCheckpoint(checkpoint) {
  try {
    await fs.mkdir(path.dirname(CHECKPOINT_FILE), { recursive: true });
    await fs.writeFile(CHECKPOINT_FILE, JSON.stringify(checkpoint, null, 2) + '\n');
    console.log(`Checkpoint saved: latest_since_id=${checkpoint.latest_since_id}`);
  } catch (err) {
    console.warn(`Failed to save checkpoint to ${CHECKPOINT_FILE}: ${err.message}`);
  }
}

async function crawl() {
  console.log(`Starting headless crawler for @${TARGET_ACCOUNT} (max: ${MAX_POSTS})`);

  const checkpoint = await loadCheckpoint();
  const sinceId = checkpoint?.latest_since_id ? BigInt(checkpoint.latest_since_id) : null;
  if (sinceId) {
    console.log(`Checkpoint active: since_id=${sinceId.toString()} (stops when reaching or older than this ID)`);
  }

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
  let reachedSinceId = false;

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

            // Check if we hit already processed checkpoint (Snowflake ID comparison)
            if (sinceId && BigInt(restId) <= sinceId) {
              console.log(`Reached since_id (${restId} <= ${sinceId.toString()}). Halting further ingestion.`);
              reachedSinceId = true;
              continue;
            }

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

    // Scroll down up to 5 times to trigger pagination until since_id is reached or MAX_POSTS captured
    for (let scroll = 0; scroll < 5; scroll++) {
      if (reachedSinceId || capturedPosts.length >= MAX_POSTS) {
        break;
      }
      console.log(`Scrolling page (iteration ${scroll + 1}/5) to load further tweets...`);
      await page.evaluate(() => window.scrollBy(0, 1500));
      await page.waitForTimeout(2000);
    }
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

  console.log(`Captured ${uniquePosts.length} new post(s) from @${TARGET_ACCOUNT}`);

  if (uniquePosts.length === 0) {
    console.log('No new posts captured in this cycle (all existing or caught by since_id). Exiting.');
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
  let newMaxSinceId = checkpoint?.latest_since_id || '';
  for (const p of uniquePosts) {
    const postFile = path.join(postsDir, `${p.source_id}.json`);
    await fs.writeFile(postFile, JSON.stringify(p, null, 2) + '\n');
    savedPaths.push(postFile);

    if (!newMaxSinceId || BigInt(p.source_id) > BigInt(newMaxSinceId)) {
      newMaxSinceId = p.source_id;
    }
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

  // Save updated checkpoint
  await saveCheckpoint({
    latest_since_id: newMaxSinceId,
    updated_at: now.toISOString(),
    processed_ids: Array.from(new Set([...(checkpoint?.processed_ids || []), ...uniquePosts.map((p) => p.source_id)]))
  });
}

crawl().catch((err) => {
  console.error('Crawler error:', err);
  process.exit(1);
});
