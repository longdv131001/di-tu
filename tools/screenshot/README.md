# Headless screenshots

Renders the real game in headless Chromium (software WebGL) so UI/visual changes can be checked without a GPU.

```bash
cd tools/screenshot && npm init -y && npm i puppeteer-core @sparticuz/chromium
go build -o ../../bin/server ../../cmd/server   # web files are embedded: rebuild after every client change
node shot.mjs out.png "await page.evaluate(()=>{const s=window.__S; s.freeCam={x:-10,y:46,z:90,tx:42,ty:0,tz:28}}); await sleep(1500);"
```

The page exposes `window.__S` (game state, free camera) only when opened with `#debug`.
