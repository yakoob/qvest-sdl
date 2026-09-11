/* Run with node scripts/ui-smoke.cjs. Set PLAYWRIGHT_MODULE and CHROMIUM_PATH
 * when Playwright/browser binaries are installed outside the project. */
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const net = require('node:net');
const { spawn, execFileSync } = require('node:child_process');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

(async () => {
  const root = path.resolve(__dirname, '..');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'shelfmate-ui-'));
  const binary = path.join(temp, 'shelfmate');
  execFileSync('go', ['build', '-o', binary, './cmd/shelfmate'], { cwd: root });
  const port = await new Promise(resolve => {
    const socket = net.createServer();
    socket.listen(0, '127.0.0.1', () => { const p = socket.address().port; socket.close(() => resolve(p)); });
  });
  const base = `http://127.0.0.1:${port}`;
  const server = spawn(binary, ['serve', '-addr', `127.0.0.1:${port}`], { cwd: root, env: { ...process.env, SHELFMATE_LLM: 'off', SHELFMATE_TEST_DRIVER: '1', SHELFMATE_EMPTY_SESSION: '1' }, stdio: 'ignore' });
  let browser;
  const get = url => fetch(base + url).then(r => { assert.equal(r.status, 200); return r.json(); });
  try {
    for (let tries = 0; ; tries++) {
      try { await get('/api/health'); break; }
      catch (e) { if (tries > 50) throw e; await new Promise(r => setTimeout(r, 100)); }
    }
    browser = await chromium.launch({ headless: true, ...(process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH } : {}) });
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
    const errors = [];
    page.on('pageerror', e => errors.push(e.message));
    await page.goto(base, { waitUntil: 'domcontentloaded' });
    await page.locator('[data-view=outcomes]').click();
    await page.locator('[data-stat="Students helped"] strong').waitFor();
    assert.equal(await page.evaluate(() => window.engagement.view), 'outcomes');
    assert.equal(await page.locator('[data-stat="Students helped"] strong').innerText(), '0');
    assert.equal(await page.locator('#outcomes-body details[open]').count(), 0);
    assert.equal(await page.getByRole('button', { name: 'Apply dates', exact: true }).isVisible(), false);
    assert.equal(await page.locator('[data-stat="Students helped"] strong').evaluate(n => n.getBoundingClientRect().bottom > 0 && n.getBoundingClientRect().top < innerHeight), true);
    await page.getByRole('button', { name: 'Reading changes', exact: true }).click();
    await page.locator('.stack-track').first().waitFor();
    const paired = await get('/api/metrics/paired');
    assert.equal(paired.students, 8);
    for (const [i, key] of ['borrowing', 'english', 'reading'].entries()) {
      const card = page.locator('.chart-grid > .viz-card').nth(i);
      assert.deepEqual(await card.locator('.legend-item strong').allTextContents(), ['increased', 'unchanged', 'decreased'].map(k => String(paired[key][k])));
    }
    await page.locator('.legend-item').first().click();
    await page.getByRole('heading', { name: 'borrowing: increased' }).waitFor();
    assert.ok(await page.locator('.stack-fill').first().evaluate(n => n.getBoundingClientRect().height >= 44));
    await page.getByRole('button', { name: 'Show all students', exact: true }).click();
    await page.getByRole('heading', { name: 'All students in this cohort' }).waitFor();
    await page.getByRole('button', { name: 'Student trends', exact: true }).click();
    await page.getByRole('heading', { name: 'Observed borrowing across periods' }).waitFor();
    const progress = await get('/api/metrics/progress?source=scenario');
    assert.deepEqual(await page.locator('.viz-card').filter({ has: page.getByRole('heading', { name: 'Observed borrowing across periods', exact: true }) }).first().locator('.bar-row').evaluateAll(nodes => nodes.map(n => Number(n.dataset.value))), progress.periods.map(p => p.checkouts));
    await page.getByRole('combobox', { name: 'Period', exact: true }).selectOption(progress.periods[0].id);
    await page.getByText(new RegExp(progress.periods[0].start + ' to ')).waitFor();
    await page.locator('[data-view=students]').click();
    await page.getByRole('button', { name: 'Priya', exact: true }).click();
    await page.getByRole('button', { name: 'Find available books', exact: true }).click();
    await page.getByRole('button', { name: 'Copy talking point' }).first().waitFor();
    assert.equal(await page.getByRole('button', { name: 'Choose together', exact: true }).count(), 0);
    await page.getByRole('button', { name: 'Mateo', exact: true }).click();
    await page.getByRole('button', { name: 'Start conversation', exact: true }).click();
    await page.getByRole('button', { name: 'Find books together', exact: true }).click();
    await page.getByRole('button', { name: 'Choose together', exact: true }).first().click();
    await page.getByRole('button', { name: 'Check out chosen book', exact: true }).click();
    await page.getByText('Ready to wrap up?', { exact: true }).waitFor();
    await page.getByRole('button', { name: 'Finish conversation', exact: true }).click();
    await page.getByRole('button', { name: 'Finish now', exact: true }).click();
    await page.locator('dialog[open]').waitFor({ state: 'detached' });
    await page.getByText('Past conversations (1)', { exact: true }).click();
    await page.getByRole('button', { name: 'Add book feedback', exact: true }).click();
    await page.getByRole('combobox', { name: 'Reading status', exact: true }).selectOption('finished');
    await page.getByRole('combobox', { name: 'Enjoyment', exact: true }).selectOption('yes');
    await page.getByRole('button', { name: 'Save feedback', exact: true }).click();
    await page.locator('dialog[open]').waitFor({ state: 'detached' });
    await page.getByText('Past conversations (1)', { exact: true }).click();
    await page.getByRole('button', { name: 'Add book feedback', exact: true }).click();
    await page.getByRole('combobox', { name: 'Reading status', exact: true }).selectOption('finished');
    await page.getByRole('combobox', { name: 'Enjoyment', exact: true }).selectOption('yes');
    await page.route('**/api/engagement', route => route.abort());
    await page.getByRole('button', { name: 'Save feedback', exact: true }).click();
    const retry = page.locator('dialog[open]').getByRole('button', { name: 'Retry interrupted action', exact: true });
    await retry.waitFor();
    await page.unroute('**/api/engagement');
    await retry.click();
    await page.locator('dialog[open]').waitFor({ state: 'detached' });
    const metrics = await get('/api/metrics');
    assert.equal(metrics.completed, 1);
    assert.equal(metrics.linked_checkouts, 1);
    assert.equal(metrics.reading_completion.numerator, 1);
    await page.locator('[data-view=outcomes]').click();
    await page.getByRole('button', { name: 'Our work', exact: true }).click();
    await page.getByRole('heading', { name: 'Every conversation is a start.' }).waitFor();
    assert.equal(await page.locator('[data-stat="Students helped"] strong').innerText(), '1');
    // A delayed old response must not repaint a different Outcomes section.
    let release;
    await page.route('**/api/metrics?*', route => new Promise(resolve => { release = async () => { await route.continue(); resolve(); }; }));
    await page.getByRole('combobox', { name: 'Period', exact: true }).selectOption('today');
    await page.waitForFunction(() => document.querySelector('#outcomes-body').getAttribute('aria-busy') === 'true');
    await page.evaluate(() => { engagement.outcomeSection = 'paired'; engagement.loadOutcomes(); });
    await page.getByRole('heading', { name: 'What changed after we met?' }).waitFor();
    await release();
    await page.unroute('**/api/metrics?*');
    assert.equal(await page.getByRole('heading', { name: 'Every conversation is a start.' }).count(), 0);
    for (const section of ['Our work', 'Reading changes', 'Student trends']) {
      await page.getByRole('button', { name: section, exact: true }).click();
      await page.waitForFunction(() => !document.querySelector('#outcomes-body').hasAttribute('aria-busy'));
      await page.setViewportSize({ width: 390, height: 844 });
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${section} mobile overflow`);
      await page.setViewportSize({ width: 1440, height: 1000 });
    }
    // Book from actual availability so this check is not pinned to one date.
    await page.locator('[data-view=students]').click();
    const bookingDate = await page.evaluate(async () => {
      let day = engagement.day();
      for (let i = 0; i < 14; i++, day = charts.shiftDate(day, 1)) {
        const result = await fetch(`/api/availability?student_id=S-406&staff_id=L-002&date=${day}&duration=10`).then(r => r.json());
        if (result.slots?.length > 1) return day;
      }
      return null;
    });
    if (bookingDate) {
      await page.getByRole('button', { name: 'Book a time', exact: true }).click();
      const dialog = page.locator('dialog[open]');
      await dialog.locator('input[type=date]').fill(bookingDate);
      await dialog.getByRole('button', { name: 'Refresh available times', exact: true }).click();
      const slot = dialog.locator('.slot-grid button').first();
      await slot.waitFor(); await slot.focus(); await page.keyboard.press('Enter');
      await dialog.getByRole('checkbox').check();
      assert.equal(await dialog.locator('button[type=submit]').isDisabled(), false);
      await dialog.getByText('Duration', { exact: true }).click();
      await dialog.locator('input[type=number]').fill('15');
      assert.equal(await dialog.locator('button[type=submit]').isDisabled(), true);
      await dialog.locator('input[type=number]').press('Tab');
      await dialog.locator('.slot-grid button').first().click();
      await dialog.getByRole('checkbox').check();
      await dialog.locator('button[type=submit]').click();
      await dialog.waitFor({ state: 'detached' });
      await page.locator('[data-view=day]').click();
      await page.locator('#agenda-body input[type=date]').fill(bookingDate);
      await page.getByText('More actions', { exact: true }).click();
      page.once('dialog', d => d.accept());
      await page.getByRole('button', { name: 'Cancel meeting', exact: true }).click();
      await page.waitForFunction(() => engagement.snapshot.appointments.some(a => a.status === 'cancelled'));
    } else {
      console.log('SKIP live booking: no remaining availability within the configured school calendar; domain reservation tests cover its lifecycle.');
    }
    const clock = await fetch(base + '/api/test/clock', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ now: '2026-09-10T15:00:00Z' }) });
    assert.equal(clock.status, 200);
    await page.locator('[data-view=students]').click();
    await page.getByRole('button', { name: 'Aisha', exact: true }).click();
    await page.getByRole('button', { name: 'Book a time', exact: true }).click();
    const follow = page.locator('dialog[open]');
    await follow.locator('input[type=date]').fill('2026-09-11');
    await follow.getByRole('button', { name: 'Refresh available times', exact: true }).click();
    await follow.locator('.slot-grid button').first().waitFor();
    await follow.locator('.slot-grid button').first().click();
    await follow.getByRole('checkbox').check();
    await follow.locator('button[type=submit]').click();
    await follow.waitFor({ state: 'detached' });
    const early = await fetch(base + '/api/engagement', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ action: 'start', appointment_id: (await get('/api/agenda')).state.appointments.find(a => a.status === 'scheduled').id, staff_id: 'L-002', request_id: 'early', expected_revision: (await get('/api/agenda')).state.revision }) });
    assert.equal(early.status, 200);
    await page.locator('[data-view=day]').click();
    await page.getByRole('button', { name: 'Find books together', exact: true }).waitFor();
    await page.getByRole('button', { name: 'Finish conversation', exact: true }).click();
    await page.getByRole('button', { name: 'Finish now', exact: true }).click();
    await page.locator('dialog[open]').waitFor({ state: 'detached' });
    assert.deepEqual(errors, []);
    console.log('PASS: My day, chart/API parity, source/period controls, linked conversation, feedback, stale response guards, injected-clock follow-up and mobile Outcomes.');
  } finally {
    if (browser) await browser.close();
    server.kill();
    await new Promise(resolve => server.exitCode != null ? resolve() : server.once('exit', resolve));
    fs.rmSync(temp, { recursive: true, force: true });
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
