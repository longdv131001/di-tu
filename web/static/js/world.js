// Builds the Three.js scene from the map the server sends. Static geometry is merged into
// one mesh per floor (vertex colours) to keep draw calls low; all decoration is generated
// here on the client from the map, so the server only has to describe walls, rooms and objects.
import * as THREE from '../vendor/three.module.min.js';

const FACES = [
  [[0, 1, 0], [[-1, 1, -1], [-1, 1, 1], [1, 1, 1], [1, 1, -1]], 1.0],
  [[0, -1, 0], [[-1, -1, 1], [-1, -1, -1], [1, -1, -1], [1, -1, 1]], 0.5],
  [[1, 0, 0], [[1, -1, -1], [1, 1, -1], [1, 1, 1], [1, -1, 1]], 0.84],
  [[-1, 0, 0], [[-1, -1, 1], [-1, 1, 1], [-1, 1, -1], [-1, -1, -1]], 0.74],
  [[0, 0, 1], [[1, -1, 1], [1, 1, 1], [-1, 1, 1], [-1, -1, 1]], 0.92],
  [[0, 0, -1], [[-1, -1, -1], [-1, 1, -1], [1, 1, -1], [1, -1, -1]], 0.8],
];

const rnd = (x, z) => { const h = Math.sin(x * 12.9898 + z * 78.233) * 43758.5453; return h - Math.floor(h); };

function mergeBoxes(list) {
  const pos = [], nor = [], col = [], idx = [];
  const c = new THREE.Color();
  for (const b of list) {
    c.setHex(b.c);
    const j = 1 + (rnd(b.x, b.z + b.y) - 0.5) * (b.top ? 0.03 : 0.08); // slight per-block variation
    for (const [n, corners, shade] of b.top ? [FACES[0]] : FACES) {
      const base = pos.length / 3;
      for (const k of corners) {
        pos.push(b.x + k[0] * b.w / 2, b.y + b.h / 2 + k[1] * b.h / 2, b.z + k[2] * b.d / 2);
        nor.push(...n);
        col.push(Math.min(1, c.r * shade * j), Math.min(1, c.g * shade * j), Math.min(1, c.b * shade * j));
      }
      idx.push(base, base + 1, base + 2, base, base + 2, base + 3);
    }
  }
  const g = new THREE.BufferGeometry();
  g.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
  g.setAttribute('normal', new THREE.Float32BufferAttribute(nor, 3));
  g.setAttribute('color', new THREE.Float32BufferAttribute(col, 3));
  g.setIndex(idx);
  g.computeBoundingSphere();
  return g;
}

const lam = () => new THREE.MeshLambertMaterial({ vertexColors: true });

// ---------- decoration ----------
const OUTER = new Set([0xbcd5ee, 0xd4e5f7, 0xf4cf93, 0xf2e08a]); // outer wall colours (see mapdata.go)

function makeDeco(map) {
  const out = [], props = [];
  let tgt = out;
  const box = (x0, z0, x1, z1, y, h, c, top = false) => tgt.push({ x: (x0 + x1) / 2, z: (z0 + z1) / 2, y, w: x1 - x0, d: z1 - z0, h, c, top });
  const at = (cx, cz, w, d, y, h, c) => tgt.push({ x: cx, z: cz, y, w, d, h, c, top: false });

  // wall caps, windows, fence posts
  for (const a of map.boxes) {
    const [x, y, z, w, h, d, c, , s] = a;
    const thin = Math.min(w, d), len = Math.max(w, d), alongX = w >= d;
    if (s !== 1 || thin > 0.45 || len < 0.9 || !(Math.abs(h - 3.6) < 0.01 || Math.abs(h - 3) < 0.01)) continue;
    const fence = Math.abs(h - 3) < 0.01;
    at(x, z, alongX ? w + 0.16 : thin + 0.16, alongX ? thin + 0.16 : d + 0.16, y + h, 0.14, fence ? 0x9b7b5b : 0xfbf8f1);
    if (fence) {
      for (let k = 1.5; k < len; k += 3) {
        const px = alongX ? x - len / 2 + k : x, pz = alongX ? z : z - len / 2 + k;
        at(px, pz, 0.55, 0.55, y, 3.4, 0xa88a68);
        at(px, pz, 0.7, 0.7, y + 3.4, 0.12, 0x8a6c4c);
      }
      continue;
    }
    if (OUTER.has(c) && len >= 6) {
      const n = Math.floor(len / 4);
      for (let k = 0; k < n; k++) {
        const t = -len / 2 + (k + 0.5) * len / n;
        const px = alongX ? x + t : x, pz = alongX ? z : z + t;
        const ww = (g, e) => (alongX ? [g, thin + e] : [thin + e, g]);
        at(px, pz, ...ww(1.9, 0.22), y + 1.05, 0.1, 0xffffff);          // sill
        at(px, pz, ...ww(1.75, 0.05), y + 1.1, 1.5, 0xffffff);          // frame
        at(px, pz, ...ww(1.45, 0.09), y + 1.25, 1.2, 0x8fd2ff);         // glass
        at(px, pz, ...ww(0.08, 0.11), y + 1.25, 1.2, 0xffffff);         // mullion
      }
    }
  }

  // door frames
  for (const d of map.doors) {
    const x = d.axis === 'x';
    const post = (off) => (x ? at(d.x + off, d.z, 0.22, 0.62, d.y, 3.1, 0x7a4f30) : at(d.x, d.z + off, 0.62, 0.22, d.y, 3.1, 0x7a4f30));
    post(-(d.w / 2 + 0.1)); post(d.w / 2 + 0.1);
    x ? at(d.x, d.z, d.w + 0.42, 0.62, d.y + 3.0, 0.2, 0x7a4f30) : at(d.x, d.z, 0.62, d.w + 0.42, d.y + 3.0, 0.2, 0x7a4f30);
  }

  // floors
  const checker = (x0, z0, x1, z1, y, base, dark, size = 2) => {
    box(x0, z0, x1, z1, y, 0, base, true);
    for (let i = 0, xx = x0; xx < x1 - 0.01; xx += size, i++) for (let j = 0, zz = z0; zz < z1 - 0.01; zz += size, j++) {
      if ((i + j) % 2 === 0) box(xx, zz, Math.min(xx + size, x1), Math.min(zz + size, z1), y + 0.012, 0, dark, true);
    }
  };
  const stripes = (x0, z0, x1, z1, y, base, dark) => {
    box(x0, z0, x1, z1, y, 0, base, true);
    for (let i = 0, zz = z0; zz < z1; zz += 1, i++) if (i % 2) box(x0, zz, x1, Math.min(zz + 1, z1), y + 0.012, 0, dark, true);
  };
  const floorY = 0.02, up = 4.02;
  checker(4.2, 4.2, 35.8, 23.8, floorY, 0xf1ecde, 0xdcd4c0);                   // hall
  checker(40.2, 4.2, 57.8, 23.8, floorY, 0xfbe7c3, 0xefcf98);                  // canteen
  checker(58.2, 4.2, 75.8, 23.8, floorY, 0xe3f2d6, 0xc8e2b3);                  // classroom
  checker(4.2, 34.2, 29.8, 55.8, floorY, 0xd4d0c6, 0xc2bdb0, 2);               // workshop
  box(60.2, 36.2, 67.8, 51.8, floorY, 0, 0x6f7891, true);                      // office
  box(68.2, 36.2, 75.8, 43.8, floorY, 0, 0x9aa1ac, true); box(68.2, 44.2, 75.8, 51.8, floorY, 0, 0x9aa1ac, true);
  for (let i = 0; i < 4; i++) stripes(8.2 + 7 * i, 4.2, 14.8 + 7 * i, 17.8, up, 0xe6c590, 0xd6b078); // cells
  box(8, 18.2, 36, 23.8, up, 0, 0xece3cf, true); box(4.2, 22, 8, 23.8, up, 0, 0xece3cf, true);        // corridor
  box(8, 20, 36, 22, up + 0.012, 0, 0xc2544c, true);                                                    // red runner
  tgt = props;
  // pillows and blankets
  for (let i = 0; i < 4; i++) {
    const x0 = 8 + 7 * i;
    at(x0 + 1.3, 5.45, 1.0, 0.45, 4.55, 0.12, 0xffffff);
    at(x0 + 1.3, 6.55, 1.7, 1.2, 4.55, 0.08, [0xf2a65a, 0x7bc5a4, 0xe58ea3, 0x8fb7ee][i]);
  }

  // yard: paths, lawn patches, court, flowers
  const path = 0xf4efe2, py = 0.012;
  box(40.4, 30, 43.6, 64, py, 0, path, true); box(14, 28.6, 78, 31.4, py, 0, path, true);
  box(14.8, 24, 17.2, 30, py, 0, path, true); box(47.8, 24, 50.2, 30, py, 0, path, true); box(65.8, 24, 68.2, 30, py, 0, path, true);
  box(30, 43.8, 40.4, 46.2, py, 0, path, true); box(43.6, 43.8, 60, 46.2, py, 0, path, true);
  const grass = 0xa8d985;
  for (const [x0, z0, x1, z1] of [[6, 58, 30, 63], [62, 54, 83, 63], [31, 32, 39, 42], [78, 31.5, 83, 52], [1, 31.5, 4, 56], [30.5, 47, 39, 62]]) box(x0, z0, x1, z1, py, 0, grass, true);
  // basketball court
  box(44, 47, 59, 62, py + 0.004, 0, 0xde7d52, true);
  const line = (x0, z0, x1, z1) => box(x0, z0, x1, z1, py + 0.02, 0, 0xffffff, true);
  line(44.4, 47.4, 58.6, 47.6); line(44.4, 61.4, 58.6, 61.6); line(44.4, 47.4, 44.6, 61.6); line(58.4, 47.4, 58.6, 61.6); line(44.4, 54.4, 58.6, 54.6);
  line(49, 47.4, 49.15, 51.4); line(54, 47.4, 54.15, 51.4); line(49, 51.3, 54.15, 51.45);
  line(49, 57.5, 49.15, 61.6); line(54, 57.5, 54.15, 61.6); line(49, 57.5, 54.15, 57.65);
  box(47, 39, 53, 43, py + 0.004, 0, 0x8aa6c4, true);                         // rubber mat at the pull-up bar
  const cols = [0xff6b81, 0xffd166, 0xffffff, 0xff9f43, 0xc56cf0];
  for (const [x0, z0, x1, z1] of [[36, 52, 39, 62], [5, 30.5, 11, 33]]) {
    box(x0, z0, x1, z1, py + 0.006, 0, 0x7a5a3a, true);
    for (let i = 0; i < 14; i++) { const fx = x0 + 0.3 + rnd(i, x0) * (x1 - x0 - 0.6), fz = z0 + 0.3 + rnd(z0, i) * (z1 - z0 - 0.6); at(fx, fz, 0.22, 0.22, 0.02, 0.34, cols[i % cols.length]); at(fx, fz, 0.06, 0.06, 0.0, 0.2, 0x3e8e41); }
  }
  // lamps, benches, bushes
  for (const [lx, lz] of [[20, 32.4], [30, 32.4], [52, 32.4], [62, 32.4], [72, 32.4], [39.9, 40], [39.9, 50], [39.9, 60], [10, 27]]) {
    at(lx, lz, 0.16, 0.16, 0, 3.2, 0x3b3f46); at(lx, lz, 0.5, 0.5, 3.2, 0.34, 0xfff3b8); at(lx, lz, 0.62, 0.62, 3.54, 0.08, 0x3b3f46);
  }
  for (const [bx, bz, along] of [[28, 33.6, true], [56, 33.6, true], [38.6, 46, false], [68, 33.6, true]]) {
    const w = along ? 1.8 : 0.5, d = along ? 0.5 : 1.8;
    at(bx, bz, w, d, 0.42, 0.1, 0xb98b5a);
    at(bx, bz, along ? w : 0.1, along ? 0.1 : d, 0.55, 0.45, 0xa97b4a);
    for (const s of [-1, 1]) at(along ? bx + s * 0.7 : bx, along ? bz : bz + s * 0.7, 0.12, 0.12, 0, 0.42, 0x4a4f57);
  }
  const bush = (bx, bz, s) => { at(bx, bz, 1.2 * s, 0.9 * s, 0, 0.75 * s, 0x5fae52); at(bx, bz, 0.9 * s, 0.7 * s, 0.7 * s, 0.35 * s, 0x6dc35f); };
  for (const bx of [6, 9, 23, 26, 30, 33]) bush(bx, 25.4, 1);
  for (const bx of [43, 46, 54, 57, 71, 74]) bush(bx, 25.4, 1);
  for (const bz of [38, 42, 50]) bush(2.6, bz, 1.1);

  // canteen
  box(44.5, 5.5, 53.5, 6.9, 1.12, 0, 0xffe2a8, true);
  at(49, 4.45, 6.5, 0.1, 2.0, 1.1, 0x2f4a3c); at(49, 4.4, 5.6, 0.1, 2.5, 0.06, 0xffffff);
  for (let i = 0; i < 12; i++) at(41.5 + i * 1.3, 4.7, 0.5, 0.4, 1.5, 0.4 + (i % 3) * 0.15, [0xff6b81, 0xffd166, 0x7bc5a4, 0x8fb7ee][i % 4]);
  box(43.7, 14.5, 46.3, 15.5, 0.82, 0, 0xd9534f, true); box(50.7, 14.5, 53.3, 15.5, 0.82, 0, 0x4f8fd9, true);
  // classroom desks and workshop crates
  for (let i = 0; i < 4; i++) { const cx = 61.5 + 4 * i; box(cx - 1.2, 9.5, cx + 1.2, 10.4, 0.82, 0, 0xe8c98f, true); at(cx, 12.2, 0.8, 0.8, 0, 0.5, 0xb07b4b); }
  for (let i = 0; i < 4; i++) at(5.3, 36.6 + i * 3.4, 1.1, 1.1, 0, 1.0 + (i % 2) * 0.4, [0xc99a5a, 0xb88648, 0xd7a869, 0xc99a5a][i]);
  at(27.2, 54.4, 1.4, 1.4, 0, 1.2, 0x3f7fbf); at(27.2, 52.6, 1.1, 1.1, 0, 0.9, 0xc9553f);

  // trees and hills outside the walls
  const trees = [];
  for (let i = 0; i < 70; i++) {
    const a = rnd(i, 3) * Math.PI * 2, r = 55 + rnd(i, 9) * 55;
    const tx = map.w / 2 + Math.cos(a) * r * 1.25, tz = map.d / 2 + Math.sin(a) * r;
    if (tx > -3 && tx < map.w + 3 && tz > -3 && tz < map.d + 3) continue;
    trees.push([tx, tz, 0.8 + rnd(i, 5) * 0.8]);
  }
  for (const [tx, tz, s] of trees) {
    at(tx, tz, 0.5 * s, 0.5 * s, 0, 2 * s, 0x7a5a3a);
    at(tx, tz, 3 * s, 3 * s, 1.6 * s, 2 * s, 0x4f9a4a); at(tx, tz, 2 * s, 2 * s, 3.4 * s, 1.6 * s, 0x62b35a);
  }
  return { out, props };
}

export function buildWorld(map, scene) {
  // sky dome (vertex colours) + clouds
  const skyGeo = new THREE.SphereGeometry(300, 24, 14);
  const sc = [], top = new THREE.Color(0x4a9cf0), hor = new THREE.Color(0xdff0ff), tmp = new THREE.Color();
  const p = skyGeo.attributes.position;
  for (let i = 0; i < p.count; i++) { tmp.copy(hor).lerp(top, Math.pow(Math.max(0, p.getY(i) / 300), 0.6)); sc.push(tmp.r, tmp.g, tmp.b); }
  skyGeo.setAttribute('color', new THREE.Float32BufferAttribute(sc, 3));
  const sky = new THREE.Mesh(skyGeo, new THREE.MeshBasicMaterial({ vertexColors: true, side: THREE.BackSide, fog: false, depthWrite: false }));
  sky.renderOrder = -10;
  scene.add(sky);
  scene.background = new THREE.Color(0xdff0ff);
  scene.fog = new THREE.Fog(0xdff0ff, 90, 250);

  const clouds = new THREE.Group();
  const cmat = new THREE.MeshBasicMaterial({ color: 0xffffff, fog: false, transparent: true, opacity: 0.95 });
  for (let i = 0; i < 12; i++) {
    const c = new THREE.Group();
    for (let k = 0; k < 4; k++) {
      const s = new THREE.Mesh(new THREE.SphereGeometry(7 + rnd(i, k) * 6, 8, 6), cmat);
      s.position.set(k * 9 - 14 + rnd(k, i) * 4, rnd(i, k + 4) * 3, rnd(k + 9, i) * 6);
      s.scale.y = 0.55; c.add(s);
    }
    c.position.set(-240 + rnd(i, 1) * 480, 70 + rnd(i, 2) * 40, -240 + rnd(i, 7) * 480);
    clouds.add(c);
  }
  scene.add(clouds);

  // distant low-poly hills
  const hillMat = new THREE.MeshLambertMaterial({ color: 0x86b872, flatShading: true });
  for (let i = 0; i < 14; i++) {
    const a = (i / 14) * Math.PI * 2 + rnd(i, 2), r = 190 + rnd(i, 5) * 40;
    const h = new THREE.Mesh(new THREE.ConeGeometry(40 + rnd(i, 8) * 40, 30 + rnd(i, 3) * 40, 5 + (i % 3)), hillMat);
    h.position.set(map.w / 2 + Math.cos(a) * r, 10, map.d / 2 + Math.sin(a) * r);
    scene.add(h);
  }

  scene.add(new THREE.HemisphereLight(0xdff0ff, 0xb9a98a, 1.0));
  const sun = new THREE.DirectionalLight(0xfff0d0, 1.25);
  sun.castShadow = true;
  sun.shadow.mapSize.set(2048, 2048);
  const sh = sun.shadow.camera; sh.left = -30; sh.right = 30; sh.top = 30; sh.bottom = -30; sh.near = 5; sh.far = 160;
  sun.shadow.bias = -0.0004; sun.shadow.normalBias = 0.05;
  scene.add(sun, sun.target);
  const SUN_OFF = new THREE.Vector3(34, 62, 24);

  // static geometry
  const ground = [], upper = [];
  const push = (b) => (b.y >= 3.5 ? upper : ground).push(b);
  for (const a of map.boxes) push({ x: a[0], y: a[1], z: a[2], w: a[3], h: a[4], d: a[5], c: a[6] });
  for (const r of map.ramps) {
    const n = 16;
    for (let k = 0; k < n; k++) {
      const t0 = k / n, t1 = (k + 1) / n, h = r.y0 + t1 * (r.y1 - r.y0);
      if (r.axis === 'z') {
        const z0 = r.z0 + t0 * (r.z1 - r.z0), z1 = r.z0 + t1 * (r.z1 - r.z0);
        ground.push({ x: (r.x0 + r.x1) / 2, y: 0, z: (z0 + z1) / 2, w: r.x1 - r.x0, h, d: z1 - z0, c: k % 2 ? 0xe8d9bf : 0xdccaa8 });
      } else {
        const x0 = r.x0 + t0 * (r.x1 - r.x0), x1 = r.x0 + t1 * (r.x1 - r.x0);
        ground.push({ x: (x0 + x1) / 2, y: 0, z: (r.z0 + r.z1) / 2, w: x1 - x0, h, d: r.z1 - r.z0, c: k % 2 ? 0xe8d9bf : 0xdccaa8 });
      }
    }
  }
  const base = map.slabs[0];
  ground.push({ x: map.w / 2, y: -0.5, z: map.d / 2, w: map.w + 400, h: 0.5, d: map.d + 400, c: 0x8cc56f });   // outside lawn
  ground.push({ x: map.w / 2, y: -0.12, z: map.d / 2, w: map.w, h: 0.12, d: map.d, c: base[5] });              // compound floor
  const deco = makeDeco(map);
  for (const b of deco.out) push(b);
  const pg = [], pu = [];
  for (const b of deco.props) (b.y >= 3.5 ? pu : pg).push(b);

  const groundMesh = new THREE.Mesh(mergeBoxes(ground), lam());
  const upperMesh = new THREE.Mesh(mergeBoxes(upper), lam());
  const propsGround = new THREE.Mesh(mergeBoxes(pg), lam()), propsUpper = new THREE.Mesh(mergeBoxes(pu), lam());
  for (const m of [groundMesh, upperMesh, propsGround, propsUpper]) { m.castShadow = true; m.receiveShadow = true; scene.add(m); }

  const doors = new Map();
  for (const d of map.doors) {
    const sx = d.axis === 'x' ? d.w : 0.3, sz = d.axis === 'x' ? 0.3 : d.w;
    const m = new THREE.Mesh(new THREE.BoxGeometry(sx, 3, sz), new THREE.MeshLambertMaterial({ color: d.c }));
    m.castShadow = true; m.receiveShadow = true;
    m.position.set(d.x, d.y + 1.5, d.z);
    scene.add(m);
    doors.set(d.id, { mesh: m, home: m.position.clone(), axis: d.axis, w: d.w, open: false, t: 0 });
  }

  // markers over interactables
  const markGeo = new THREE.OctahedronGeometry(0.2);
  const markMat = new THREE.MeshBasicMaterial({ color: 0xffd34d });
  const marks = map.objs.map((o) => {
    const m = new THREE.Mesh(markGeo, markMat);
    m.userData.y0 = o.y + (o.kind === 'bed' ? 1.3 : 2.2);
    m.position.set(o.x, m.userData.y0, o.z);
    scene.add(m);
    return m;
  });

  const inCellBlock = (x, y, z) => y < 2 && x > 3.5 && x < 36.5 && z > 3.5 && z < 24.5;
  const raycast = new THREE.Raycaster();
  const SNAP = 60 / 2048;

  return {
    ground: groundMesh, upper: upperMesh, doors, marks, sky, sun,
    setDoor(id, open) { const d = doors.get(id); if (d) d.open = open; },
    setQuality(low) { sun.castShadow = !low; },
    update(dt, px, py, pz, cam) {
      upperMesh.visible = propsUpper.visible = !inCellBlock(px, py, pz);
      for (const d of doors.values()) {
        d.t += ((d.open ? 1 : 0) - d.t) * Math.min(1, dt * 10);
        const off = d.t * d.w * 0.92;
        d.mesh.position.set(d.home.x + (d.axis === 'x' ? off : 0), d.home.y, d.home.z + (d.axis === 'x' ? 0 : off));
      }
      const tt = performance.now() / 1000;
      marks.forEach((m, i) => { m.rotation.y = tt * 2 + i; m.position.y = m.userData.y0 + Math.sin(tt * 2.4 + i) * 0.08; });
      for (const c of clouds.children) { c.position.x += dt * 1.2; if (c.position.x > 260) c.position.x = -260; }
      if (cam) sky.position.copy(cam);
      sun.target.position.set(Math.round(px / SNAP) * SNAP, py, Math.round(pz / SNAP) * SNAP);
      sun.position.copy(sun.target.position).add(SUN_OFF);
    },
    obstacle(from, dir, maxDist) {
      raycast.set(from, dir);
      raycast.far = maxDist;
      const targets = [groundMesh];
      if (upperMesh.visible) targets.push(upperMesh);
      for (const d of doors.values()) if (d.t < 0.5) targets.push(d.mesh);
      const hit = raycast.intersectObjects(targets, false)[0];
      return hit ? hit.distance : Infinity;
    },
  };
}
