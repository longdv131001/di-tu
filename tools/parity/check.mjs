// Replays the same scripted inputs through physics.js and compares with the Go output.
import { readFileSync } from 'node:fs';
import { Physics } from '../../web/static/js/physics.js';

const data = JSON.parse(readFileSync(process.argv[2], 'utf8'));
const phys = new Physics(data.map);
let worst = 0, checked = 0;
data.runs.forEach((r, ri) => {
  const si = Math.floor(ri / 2);
  let state = (12345 + si * 7) >>> 0;
  const rnd = () => { state = (Math.imul(state, 1664525) + 1013904223) >>> 0; return state; };
  phys.open = r.open ? new Set(data.map.doors.map((d) => d.id)) : new Set();
  const b = { x: r.start[0], y: r.start[1], z: r.start[2], vy: 0, grounded: true };
  const inp = { mx: 0, mz: 0, yaw: 0, run: false, jump: false };
  for (let step = 0; step < 4000; step++) {
    if (step % 15 === 0) {
      inp.mx = (rnd() >>> 16) % 3 - 1;
      inp.mz = (rnd() >>> 16) % 3 - 1;
      inp.yaw += (((rnd() >>> 16) % 100) - 50) / 40;
      inp.run = ((rnd() >>> 16) & 1) === 1;
    }
    inp.jump = step % 37 === 0;
    phys.step(b, inp);
    if (step % 50 === 0) {
      const s = r.samples[step / 50];
      const err = Math.max(Math.abs(b.x - s[0]), Math.abs(b.y - s[1]), Math.abs(b.z - s[2]));
      worst = Math.max(worst, err); checked++;
      if (err > 1e-6) { console.error(`DIVERGED run ${ri} step ${step}: js=(${b.x},${b.y},${b.z}) go=(${s})`); process.exit(1); }
    }
  }
});
console.log(`physics parity OK: ${checked} samples over ${data.runs.length} runs, worst error ${worst.toExponential(2)}`);
