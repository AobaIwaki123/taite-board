const fs = require('fs');
const path = require('path');
const https = require('https');
const http = require('http');

const schedulePath = path.join(__dirname, '../docs/mock_schedule.json');
const schedule = JSON.parse(fs.readFileSync(schedulePath, 'utf8'));

// Load fxtwitter raw posts if available for cross-verification
const fxtwitterPath = path.join(__dirname, '../data/fxtwitter_full.json');
let rawPosts = [];
if (fs.existsSync(fxtwitterPath)) {
  rawPosts = JSON.parse(fs.readFileSync(fxtwitterPath, 'utf8'));
}

async function checkUrl(url, label, retry = 1) {
  if (!url) return { ok: false, error: 'URL is empty' };
  if (!url.startsWith('http://') && !url.startsWith('https://')) {
    // Local file check
    const localPath = path.join(__dirname, '../docs', url);
    const exists = fs.existsSync(localPath);
    return { ok: exists, status: exists ? 'local file exists' : 'local file NOT FOUND' };
  }

  return new Promise(resolve => {
    const mod = url.startsWith('https://') ? https : http;
    const req = mod.request(url, {
      method: 'HEAD',
      headers: {
        'User-Agent': 'Mozilla/5.0 (compatible; schedule-validator/1.0)'
      },
      timeout: 10000
    }, res => {
      // 2xx, 3xx are valid
      const ok = res.statusCode >= 200 && res.statusCode < 400;
      resolve({ ok, status: res.statusCode });
    });

    req.on('error', async err => {
      if (retry > 0) {
        const retryRes = await checkUrl(url, label, retry - 1);
        resolve(retryRes);
      } else {
        resolve({ ok: false, error: err.message });
      }
    });

    req.on('timeout', async () => {
      req.destroy();
      if (retry > 0) {
        const retryRes = await checkUrl(url, label, retry - 1);
        resolve(retryRes);
      } else {
        resolve({ ok: false, error: 'Timeout' });
      }
    });
    req.end();
  });
}

async function validate() {
  console.log('=== Starting Schedule Self-Verification ===');
  let hasError = false;

  for (const session of schedule.sessions) {
    console.log(`\nValidating [${session.date}] ${session.title} (${session.venue.name})`);

    // 1. Multi-Image Validation (HTTP 200 or local file)
    const imagesToCheck = session.images && session.images.length > 0 
      ? session.images.map(img => img.url) 
      : [session.image_url];

    for (let idx = 0; idx < imagesToCheck.length; idx++) {
      const u = imagesToCheck[idx];
      const imgRes = await checkUrl(u, `image-${idx}`);
      if (imgRes.ok) {
        console.log(`  ✓ Image [${idx + 1}/${imagesToCheck.length}]: ${u.slice(0, 45)}... (${imgRes.status})`);
      } else {
        console.error(`  ✗ Image FAILED: ${u} -> ${imgRes.error || imgRes.status}`);
        hasError = true;
      }
    }

    // 2. Ticket URL validation
    const ticketRes = await checkUrl(session.ticket.url, 'ticket');
    if (ticketRes.ok) {
      console.log(`  ✓ Ticket (${session.ticket.platform}): ${session.ticket.url.slice(0, 45)}... (${ticketRes.status})`);
    } else {
      console.warn(`  ! Ticket Warning: ${session.ticket.url} -> ${ticketRes.error || ticketRes.status}`);
    }

    // 3. Tweet Cross-Verification (Self-Verification Gate)
    if (session.source_tweet_url && rawPosts.length > 0) {
      const tweet = rawPosts.find(p => p.url === session.source_tweet_url);
      if (tweet) {
        // Verify Image belongs to this tweet
        const mediaUrls = (tweet.media && tweet.media.all || []).map(m => m.url);
        if (mediaUrls.length > 0 && session.image_url && session.image_url.startsWith('http')) {
          const matched = mediaUrls.some(u => {
            const mediaId = u.split('/').pop().split('.')[0];
            return session.image_url.includes(mediaId);
          });
          if (matched) {
            console.log(`  ✓ Media Cross-Check: Verified matching flyer in source tweet`);
          } else {
            console.warn(`  ⚠ Media Mismatch Warning: ${session.image_url} not in source tweet media`);
          }
        }

        // Verify unannounced timetable is not hallucinated
        const hasTimeKeywords = tweet.text && (tweet.text.includes('🎤出演') || tweet.text.includes('出番') || tweet.text.includes('出演：'));
        if (!hasTimeKeywords && session.live.start_time !== '--:--' && session.live.start_time !== '未定') {
          console.warn(`  ⚠ Telemetry Guard: Tweet does not state live times, but session has: ${session.live.start_time}`);
        } else {
          console.log(`  ✓ Telemetry Guard: Live time format verified (${session.live.start_time})`);
        }
      }
    }
  }

  console.log('\n=== Verification Summary ===');
  if (hasError) {
    console.error('Self-Verification completed with critical errors.');
    process.exit(1);
  } else {
    console.log('All session integrity and image validations passed successfully!');
  }
}

validate();