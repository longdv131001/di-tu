// Client-side mirror of internal/world/world.go. Keep both in sync: the server is the
// authority, this copy only predicts the local player so movement feels instant.
export const R = 0.4, H = 1.8, STEP = 0.45, WALK = 3.2, RUN = 6.0, GRAV = 22.0, JUMP = 7.2, DT = 0.05, LOW = 0.35;

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));
const toBox = (a) => ({ x: a[0], y: a[1], z: a[2], w: a[3], h: a[4], d: a[5] });

export class Physics {
  constructor(map) {
    this.map = map;
    this.solids = map.boxes.filter((b) => b[8] === 1).map(toBox);
    this.slabs = map.slabs.map((s) => ({ x0: s[0], z0: s[1], x1: s[2], z1: s[3], y: s[4] }));
    this.ramps = map.ramps;
    this.doors = map.doors.map((d) => {
      const b = { x: d.x, y: d.y, z: d.z, w: d.w, h: 3, d: 0.3 };
      if (d.axis !== 'x') { b.w = 0.3; b.d = d.w; }
      return { id: d.id, box: b };
    });
    this.open = new Set();
  }

  roomAt(x, y, z) {
    const f = y >= 2 ? 1 : 0;
    for (const r of this.map.rooms) {
      if (r.f === f && x >= r.x0 && x < r.x1 && z >= r.z0 && z < r.z1) return r.id;
    }
    return 'yard';
  }

  static pushOut(b, y, bx) {
    if (y + H <= bx.y || y + LOW >= bx.y + bx.h) return;
    const hx = bx.w / 2, hz = bx.d / 2;
    const cx = clamp(b.x, bx.x - hx, bx.x + hx), cz = clamp(b.z, bx.z - hz, bx.z + hz);
    const dx = b.x - cx, dz = b.z - cz, d2 = dx * dx + dz * dz;
    if (d2 >= R * R) return;
    if (d2 > 1e-9) {
      const d = Math.sqrt(d2), k = (R - d) / d;
      b.x += dx * k; b.z += dz * k;
      return;
    }
    const px = hx - Math.abs(b.x - bx.x), pz = hz - Math.abs(b.z - bx.z);
    if (px < pz) b.x = b.x >= bx.x ? bx.x + hx + R : bx.x - hx - R;
    else b.z = b.z >= bx.z ? bx.z + hz + R : bx.z - hz - R;
  }

  resolve(b) {
    for (let i = 0; i < 2; i++) {
      for (const s of this.solids) Physics.pushOut(b, b.y, s);
      for (const d of this.doors) if (!this.open.has(d.id)) Physics.pushOut(b, b.y, d.box);
    }
  }

  groundAt(x, z, y) {
    let best = 0;
    for (const s of this.slabs) {
      if (x >= s.x0 && x <= s.x1 && z >= s.z0 && z <= s.z1 && s.y <= y + STEP && s.y > best) best = s.y;
    }
    for (const r of this.ramps) {
      if (x < r.x0 || x > r.x1 || z < r.z0 || z > r.z1) continue;
      const t = r.axis === 'z' ? (z - r.z0) / (r.z1 - r.z0) : (x - r.x0) / (r.x1 - r.x0);
      const h = r.y0 + t * (r.y1 - r.y0);
      if (h <= y + STEP && h > best) best = h;
    }
    return best;
  }

  // body: {x,y,z,vy,grounded}; input: {mx,mz,yaw,jump,run}. Returns {facing, moving}.
  step(b, inp) {
    let mx = clamp(inp.mx, -1, 1), mz = clamp(inp.mz, -1, 1);
    const l = Math.hypot(mx, mz);
    if (l > 1) { mx /= l; mz /= l; }
    const wx = -mx * Math.cos(inp.yaw) + mz * Math.sin(inp.yaw);
    const wz = mx * Math.sin(inp.yaw) + mz * Math.cos(inp.yaw);
    const moving = Math.hypot(mx, mz) > 0.05;
    const speed = inp.run ? RUN : WALK;
    b.x += wx * speed * DT; this.resolve(b);
    b.z += wz * speed * DT; this.resolve(b);
    b.x = clamp(b.x, R, this.map.w - R);
    b.z = clamp(b.z, R, this.map.d - R);

    let was = b.grounded;
    if (was && inp.jump) { b.vy = JUMP; b.grounded = false; was = false; }
    b.vy -= GRAV * DT;
    b.y += b.vy * DT;
    const g = this.groundAt(b.x, b.z, b.y);
    if (b.y <= g) { b.y = g; b.vy = 0; b.grounded = true; }
    else if (was && b.vy <= 0 && b.y - g <= STEP) { b.y = g; b.vy = 0; b.grounded = true; }
    else b.grounded = false;
    return { facing: moving ? Math.atan2(wx, wz) : null, moving };
  }
}
