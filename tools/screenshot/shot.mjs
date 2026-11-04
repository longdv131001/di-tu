import chromium from '@sparticuz/chromium';
import puppeteer from 'puppeteer-core';
import { spawn } from 'node:child_process';

const out = process.argv[2] || 'a.png';
const script = process.argv[3] || '';          // optional JS (async) run in node with {page, sleep}
const port = 18090;
const srv = spawn('/home/claude/v2/bin/server', [], { env: { ...process.env, PORT: String(port), PBKDF2_ITER: '1000' }, stdio: 'ignore' });
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
await sleep(800);
const browser = await puppeteer.launch({ executablePath: await chromium.executablePath(), headless: 'shell',
  args: [...chromium.args, '--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--ignore-gpu-blocklist', '--no-sandbox'] });
try {
  const page = await browser.newPage();
  page.on('console', (m) => { if (['error', 'warning'].includes(m.type())) console.log('[browser]', m.type(), m.text()); });
  page.on('pageerror', (e) => console.log('[pageerror]', e.message));
  await page.setViewport({ width: 1100, height: 700 });
  await page.goto(`http://localhost:${port}/#debug`, { waitUntil: 'load' });
  await sleep(1500);
  if (process.env.AUTH_SHOT) { await page.screenshot({ path: process.env.AUTH_SHOT }); }
  await page.click('#tabReg');
  await page.type('#fUser', 'tester1');
  await page.type('#fPass', 'password123');
  await page.type('#fName', 'Long');
  await page.click('#fGo');
  await sleep(4000);
  if (script) await (new Function('page', 'sleep', `return (async()=>{${script}})()`))(page, sleep);
  await page.screenshot({ path: out });
  console.log('saved', out);
} finally { await browser.close(); srv.kill(); }
