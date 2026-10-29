const fs = require('fs');
const path = require('path');
const https = require('https');
const http = require('http');

const schedulePath = path.join(__dirname, '../docs/mock_schedule.json');
const schedule = JSON.parse(fs.readFileSync(schedulePath, 'utf8'));

async function checkUrl(url, label) {
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
      timeout: 5000
    }, res => {
      // 2xx, 3xx are valid
      const ok = res.statusCode >= 200 && res.statusCode < 400;
      resolve({ ok, status: res.statusCode });
    });

    req.on('error', err => resolve({ ok: false, error: err.message }));
    req.on('timeout', () => {
      req.destroy();
      resolve({ ok: false, error: 'Timeout' });
    });
    req.end();
  });
}

async function validate() {
  console.log('=== Starting Schedule Validation ===');
  let hasError = false;

  for (const session of schedule.sessions) {
    console.log(`\nValidating [${session.date}] ${session.title} (${session.venue.name})`);

    // 1. Image URL validation
    const imgRes = await checkUrl(session.image_url, 'image');
    if (imgRes.ok) {
      console.log(`  ✓ Image: ${session.image_url.slice(0, 45)}... (${imgRes.status})`);
    } else {
      console.error(`  ✗ Image FAILED: ${session.image_url} -> ${imgRes.error || imgRes.status}`);
      hasError = true;
    }

    // 2. Ticket URL validation
    const ticketRes = await checkUrl(session.ticket.url, 'ticket');
    if (ticketRes.ok) {
      console.log(`  ✓ Ticket (${session.ticket.platform}): ${session.ticket.url.slice(0, 45)}... (${ticketRes.status})`);
    } else {
      console.warn(`  ! Ticket Warning: ${session.ticket.url} -> ${ticketRes.error || ticketRes.status}`);
    }
  }

  console.log('\n=== Validation Complete ===');
  if (hasError) {
    console.error('Validation completed with errors.');
    process.exit(1);
  } else {
    console.log('All image URLs validated successfully!');
  }
}

validate();
