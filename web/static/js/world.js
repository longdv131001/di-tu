// Builds the Three.js scene from the map the server sends. Static geometry is merged
// into one mesh per floor (vertex colours, flat shading) to keep draw calls and memory low.
import * as THREE from '../vendor/three.module.min.js';

const FACES = [
  // normal, 4 corners (unit cube centred at origin), shade
  [[0, 1, 0], [[-1, 1, -1], [-1, 1, 1], [1, 1, 1], [1, 1, -1]], 1.0],
  [[0, -1, 0], [[-1, -1, 1], [-1, -1, -1], [1, -1, -1], [1, -1, 1]], 0.5],
  [[1, 0, 0], [[1, -1, -1], [1, 1, -1], [1, 1, 1], [1, -1, 1]], 0.82],
  [[-1, 0, 0], [[-1, -1, 1], [-1, 1, 1], [-1, 1, -1], [-1, -1, -1]], 0.72],
  [[0, 0, 1], [[1, -1, 1], [1, 1, 1], [-1, 1, 1], [-1, -1, 1]], 0.9],
  [[0, 0, -1], [[-1, -1, -1], [-1, 1, -1], [1, 1, -1], [1, -1, -1]], 0.78],
];

function mergeBoxes(list) {
  const pos = [], nor = [], col = [], idx = [];
  const c = new THREE.Color();
  for (const b of list) {
    c.setHex(b.c);
    for (const [n, corners, shade] of FACES) {
      const base = pos.length / 3;
      for (const k of corners) {
        pos.push(b.x + k[0] * b.w / 2, b.y + b.h / 2 + k[1] * b.h / 2, b.z + k[2] * b.d / 2);
        nor.push(...n);
        col.push(c.r * shade, c.g * shade, c.b * shade);
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

const mat = () => new THREE.MeshLambertMaterial({ vertexColors: true });

export function buildWorld(map, scene) {
  scene.background = new THREE.Color(0x9fc4e8);
  scene.fog = new THREE.Fog(0x9fc4e8, 60, 160);
  scene.add(new THREE.HemisphereLight(0xffffff, 0x8a8f7a, 0.85));
  const sun = new THREE.DirectionalLight(0xfff2d8, 0.9);
  sun.position.set(30, 60, 20);
  scene.add(sun);

  const ground = [], upper = [];
  for (const a of map.boxes) {
    const b = { x: a[0], y: a[1], z: a[2], w: a[3], h: a[4], d: a[5], c: a[6] };
    (a[7] === 1 ? upper : ground).push(b);
  }
  // stairs: drawn as steps (physics uses the smooth ramp)
  for (const r of map.ramps) {
    const n = 16;
    for (let k = 0; k < n; k++) {
      const t0 = k / n, t1 = (k + 1) / n;
      const h = r.y0 + t1 * (r.y1 - r.y0);
      if (r.axis === 'z') {
        const z0 = r.z0 + t0 * (r.z1 - r.z0), z1 = r.z0 + t1 * (r.z1 - r.z0);
        ground.push({ x: (r.x0 + r.x1) / 2, y: 0, z: (z0 + z1) / 2, w: r.x1 - r.x0, h, d: z1 - z0, c: 0xcfcabb });
      } else {
        const x0 = r.x0 + t0 * (r.x1 - r.x0), x1 = r.x0 + t1 * (r.x1 - r.x0);
        ground.push({ x: (x0 + x1) / 2, y: 0, z: (r.z0 + r.z1) / 2, w: x1 - x0, h, d: r.z1 - r.z0, c: 0xcfcabb });
      }
    }
  }
  const base = map.slabs[0];
  ground.push({ x: map.w / 2, y: -0.4, z: map.d / 2, w: map.w + 40, h: 0.4, d: map.d + 40, c: 0x7f9a6a }); // outside lawn
  ground.push({ x: map.w / 2, y: -0.05, z: map.d / 2, w: map.w, h: 0.05, d: map.d, c: base[5] });

  const groundMesh = new THREE.Mesh(mergeBoxes(ground), mat());
  const upperMesh = new THREE.Mesh(mergeBoxes(upper), mat());
  scene.add(groundMesh, upperMesh);

  // doors slide aside when open
  const doors = new Map();
  for (const d of map.doors) {
    const sx = d.axis === 'x' ? d.w : 0.3, sz = d.axis === 'x' ? 0.3 : d.w;
    const m = new THREE.Mesh(new THREE.BoxGeometry(sx, 3, sz), new THREE.MeshLambertMaterial({ color: d.c }));
    m.position.set(d.x, d.y + 1.5, d.z);
    scene.add(m);
    doors.set(d.id, { mesh: m, home: m.position.clone(), axis: d.axis, w: d.w, open: false, t: 0 });
  }

  // markers over interactables
  const markGeo = new THREE.OctahedronGeometry(0.18);
  const markMat = new THREE.MeshBasicMaterial({ color: 0xffd34d });
  const marks = [];
  for (const o of map.objs) {
    const m = new THREE.Mesh(markGeo, markMat);
    m.position.set(o.x, o.y + (o.kind === 'bed' ? 1.3 : 2.1), o.z);
    scene.add(m);
    marks.push(m);
  }

  // building footprint above which the upper floor is cut away
  const inCellBlock = (x, y, z) => y < 2 && x > 3.5 && x < 36.5 && z > 3.5 && z < 24.5;
  const raycast = new THREE.Raycaster();

  return {
    ground: groundMesh, upper: upperMesh, doors, marks,
    setDoor(id, open) { const d = doors.get(id); if (d) d.open = open; },
    update(dt, px, py, pz) {
      upperMesh.visible = !inCellBlock(px, py, pz);
      for (const d of doors.values()) {
        d.t += ((d.open ? 1 : 0) - d.t) * Math.min(1, dt * 10);
        const off = d.t * d.w * 0.92;
        d.mesh.position.set(d.home.x + (d.axis === 'x' ? off : 0), d.home.y, d.home.z + (d.axis === 'x' ? 0 : off));
      }
      const tt = performance.now() / 1000;
      marks.forEach((m, i) => { m.rotation.y = tt * 2 + i; });
    },
    // distance from `from` along `dir` to the first wall/door, or Infinity
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
