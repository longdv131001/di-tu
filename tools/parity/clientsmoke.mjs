// Headless smoke test of the client modules: builds the scene graph, avatars and every
// animation pose, and checks camera-collision raycasts. (No WebGL needed.)
import { readFileSync } from 'node:fs';
const canvasStub = () => ({ width: 0, height: 0, getContext: () => new Proxy({}, { get: (_, k) => (k === 'measureText' ? () => ({ width: 10 }) : () => {}), set: () => true }) });
globalThis.document = { createElement: canvasStub };
const THREE = await import('../../web/static/vendor/three.module.min.js');
const { buildWorld } = await import('../../web/static/js/world.js');
const { Avatar, ANIM } = await import('../../web/static/js/avatar.js');

const map = JSON.parse(readFileSync(process.argv[2], 'utf8')).map;
const scene = new THREE.Scene();
const w = buildWorld(map, scene);
let tris = 0;
for (const m of [w.ground, w.upper]) tris += m.geometry.index.count / 3;
console.log(`scene: ${scene.children.length} objects, ${tris} merged triangles, ${w.doors.size} doors, ${w.marks.length} markers`);
if (tris < 500 || w.doors.size !== map.doors.length) throw new Error('scene looks wrong');

// camera collision: from inside the hall looking through the north wall must hit it
scene.updateMatrixWorld(true);
const from = new THREE.Vector3(20, 1.5, 10), dir = new THREE.Vector3(0, 0, -1);
const hit = w.obstacle(from, dir, 20);
if (!(hit > 5.5 && hit < 6.5)) throw new Error('expected north wall ~6m away, got ' + hit);
if (w.obstacle(new THREE.Vector3(20, 1.5, 28), new THREE.Vector3(1, 0, 0), 5) !== Infinity) throw new Error('open yard should be clear');
w.setDoor('d_b1_main', true);
for (let i = 0; i < 40; i++) w.update(0.05, 20, 0, 10);
if (w.upper.visible) throw new Error('upper floor should be cut away inside the cell block');
w.update(0.05, 20, 4, 10);
if (!w.upper.visible) throw new Error('upper floor should show when standing on it');

// avatars: every outfit option and every animation pose must run
let n = 0;
for (let shirt = 0; shirt < 12; shirt++) for (let hair = 0; hair < 4; hair++) for (let hat = 0; hat < 3; hat++) {
  const a = new Avatar({ name: 'Test ' + shirt, outfit: { shirt, pants: shirt % 8, skin: shirt % 6, hair, hairc: shirt % 8, hat } });
  for (const code of Object.values(ANIM)) { a.animate(code, 0.016, 1.234); n++; }
  for (let f = 0; f < 5; f++) a.setFace(f);
  a.dispose();
}
const g = new Avatar({ name: 'Cai ngục', guard: true });
g.animate(ANIM.walk, 0.016, 0);
console.log(`avatar poses OK (${n} combinations), client smoke test passed`);
