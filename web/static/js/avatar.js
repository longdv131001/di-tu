// Procedural chibi-style character: capsule limbs, big round head, outfit, face expressions,
// chat bubble and animation poses. Geometry is shared between avatars to keep memory low.
import * as THREE from '../vendor/three.module.min.js';

export const SHIRTS = [0xff8a3d, 0x4f9cf5, 0x56c27f, 0xffc933, 0xa779e6, 0xf06292, 0x3cc3c9, 0x8a919c, 0xf3f3f3, 0x34495e, 0xc8793a, 0x8bc34a];
export const PANTS = [0x34495e, 0x4a4f57, 0x7a6248, 0x4a78b5, 0x7d8a45, 0x9b4a4a, 0x26272b, 0x9aa0a8];
export const SKINS = [0xffdcc0, 0xf3c29b, 0xdba074, 0xbf8157, 0x93603a, 0x63402a];
export const HAIRS = [0x25201d, 0x5a3825, 0x8b5e34, 0xe0b24a, 0xc3c6cc, 0xb03a3a, 0x3e63b5, 0x8e4fc0];
export const ANIM = { idle: 0, walk: 1, run: 2, jump: 3, sleep: 4, work: 5, study: 6, sit: 7, wave: 8, dance: 9, laugh: 10, gym: 11, eat: 12, cut: 13 };

const geoCache = new Map();
const geo = (key, make) => { if (!geoCache.has(key)) geoCache.set(key, make()); return geoCache.get(key); };
const capsule = (r, l) => geo(`c${r}_${l}`, () => new THREE.CapsuleGeometry(r, l, 4, 10));
const sphere = (r, ...a) => geo(`s${r}_${a}`, () => new THREE.SphereGeometry(r, 16, 12, ...a));
const lamb = (color) => new THREE.MeshLambertMaterial({ color });
const mesh = (g, color) => new THREE.Mesh(g, lamb(color));

const tagCache = new Map();
function roundRect(g, x, y, w, h, r) {
  g.beginPath(); g.moveTo(x + r, y); g.arcTo(x + w, y, x + w, y + h, r); g.arcTo(x + w, y + h, x, y + h, r);
  g.arcTo(x, y + h, x, y, r); g.arcTo(x, y, x + w, y, r); g.closePath();
}
function tagTexture(name, guard) {
  const key = name + (guard ? '|g' : '');
  if (tagCache.has(key)) return tagCache.get(key);
  const c = document.createElement('canvas');
  c.width = 256; c.height = 64;
  const g = c.getContext('2d');
  g.font = '800 28px "Nunito", system-ui, sans-serif';
  g.textAlign = 'center'; g.textBaseline = 'middle';
  const w = Math.min(244, g.measureText(name).width + 34);
  g.fillStyle = guard ? 'rgba(36,87,166,.92)' : 'rgba(40,30,20,.62)';
  roundRect(g, 128 - w / 2, 8, w, 48, 24); g.fill();
  g.fillStyle = '#fff';
  g.fillText(name, 128, 33, 220);
  const t = new THREE.CanvasTexture(c);
  t.colorSpace = THREE.SRGBColorSpace;
  tagCache.set(key, t);
  return t;
}

function bubbleTexture(text) {
  const c = document.createElement('canvas');
  c.width = 384; c.height = 160;
  const g = c.getContext('2d');
  g.font = '700 26px "Nunito", system-ui, sans-serif';
  const words = text.split(' '), lines = [];
  let cur = '';
  for (const w of words) {
    const t = cur ? cur + ' ' + w : w;
    if (g.measureText(t).width > 320 && cur) { lines.push(cur); cur = w; } else cur = t;
  }
  lines.push(cur);
  if (lines.length > 3) { lines.length = 3; lines[2] = lines[2].replace(/.{0,2}$/, '…'); }
  const w = Math.min(360, Math.max(...lines.map((l) => g.measureText(l).width)) + 36);
  const h = lines.length * 32 + 26;
  const top = 150 - h - 14;
  g.fillStyle = '#fff'; g.strokeStyle = 'rgba(60,40,20,.35)'; g.lineWidth = 3;
  roundRect(g, 192 - w / 2, top, w, h, 18); g.fill(); g.stroke();
  g.beginPath(); g.moveTo(180, top + h); g.lineTo(192, top + h + 13); g.lineTo(204, top + h); g.fill();
  g.fillStyle = '#3a2a1a'; g.textAlign = 'center'; g.textBaseline = 'middle';
  lines.forEach((l, i) => g.fillText(l, 192, top + 28 + i * 32, 330));
  const t = new THREE.CanvasTexture(c);
  t.colorSpace = THREE.SRGBColorSpace;
  return t;
}

export class Avatar {
  constructor(meta) {
    this.root = new THREE.Group();   // placed at the feet
    this.body = new THREE.Group();   // posed (lying, sitting, bouncing)
    this.root.add(this.body);
    this.guard = !!meta.guard;
    const o = meta.outfit || { shirt: 0, pants: 0, skin: 0, hair: 0, hairc: 0, hat: 0 };
    const shirt = this.guard ? 0x2f6fd0 : SHIRTS[o.shirt % SHIRTS.length];
    const pants = this.guard ? 0x1f2a44 : PANTS[o.pants % PANTS.length];
    const skin = SKINS[o.skin % SKINS.length];

    const leg = (x) => {
      const g = new THREE.Group(); g.position.set(x, 0.5, 0);
      const l = mesh(capsule(0.085, 0.2), pants); l.position.y = -0.2;
      const s = mesh(sphere(0.1), this.guard ? 0x15171c : 0xf7f7f7); s.scale.set(1, 0.6, 1.45); s.position.set(0, -0.43, 0.04);
      g.add(l, s); return g;
    };
    const arm = (x) => {
      const g = new THREE.Group(); g.position.set(x, 0.98, 0);
      const a = mesh(capsule(0.058, 0.2), shirt); a.position.y = -0.18;
      const h = mesh(sphere(0.07), skin); h.position.y = -0.39;
      g.add(a, h); return g;
    };
    this.legL = leg(-0.1); this.legR = leg(0.1);
    this.torso = mesh(capsule(0.2, 0.16), shirt); this.torso.position.y = 0.78; this.torso.scale.z = 0.86;
    this.armL = arm(-0.27); this.armR = arm(0.27);
    this.head = new THREE.Group(); this.head.position.y = 1.37;
    const skull = mesh(sphere(0.3), skin); skull.scale.set(1.02, 0.96, 0.98);
    this.head.add(skull);
    this.body.add(this.legL, this.legR, this.torso, this.armL, this.armR, this.head);
    if (this.guard) { const belt = mesh(geo('belt', () => new THREE.CylinderGeometry(0.205, 0.205, 0.07, 14)), 0x2a1d12); belt.position.y = 0.62; belt.scale.z = 0.86; this.body.add(belt); }

    // face
    const basic = (color, opts = {}) => new THREE.MeshBasicMaterial({ color, ...opts });
    const circ = (r) => geo(`ci${r}`, () => new THREE.CircleGeometry(r, 14));
    const eye = (x) => {
      const g = new THREE.Group(); g.position.set(x, 0.0, 0.286);
      const w = new THREE.Mesh(circ(0.078), basic(0xffffff));
      const p = new THREE.Mesh(circ(0.05), basic(0x1b1410)); p.position.set(0, -0.006, 0.004);
      const h = new THREE.Mesh(circ(0.016), basic(0xffffff)); h.position.set(0.018, 0.018, 0.008);
      g.add(w, p, h); return g;
    };
    this.eyeL = eye(-0.108); this.eyeR = eye(0.108);
    this.browL = new THREE.Mesh(geo('brow', () => new THREE.PlaneGeometry(0.1, 0.022)), basic(0x3a2a1c));
    this.browR = this.browL.clone();
    this.browL.position.set(-0.108, 0.1, 0.27); this.browR.position.set(0.108, 0.1, 0.27);
    this.mouth = new THREE.Mesh(geo('mouth', () => new THREE.PlaneGeometry(0.075, 0.03)), basic(0x8a3a3a));
    this.mouth.position.set(0, -0.105, 0.283);
    const blush = (x) => { const b = new THREE.Mesh(circ(0.05), basic(0xff8fa3, { transparent: true, opacity: 0.55 })); b.position.set(x, -0.065, 0.245); b.rotation.y = x * 2; return b; };
    this.head.add(this.eyeL, this.eyeR, this.browL, this.browR, this.mouth, blush(-0.18), blush(0.18));

    // hair and hats
    const hair = HAIRS[o.hairc % HAIRS.length];
    if (!this.guard) {
      if (o.hair !== 3) {
        const cap = mesh(sphere(0.318, 0, Math.PI * 2, 0, 1.02), hair); cap.scale.set(1.02, 1.0, 1.0); cap.position.y = 0.01; this.head.add(cap);
      }
      if (o.hair === 1) { const back = mesh(sphere(0.3), hair); back.scale.set(1.0, 1.25, 0.62); back.position.set(0, -0.12, -0.19); this.head.add(back); }
      if (o.hair === 2) for (let k = 0; k < 6; k++) {
        const a = (k / 6) * Math.PI * 2, s = mesh(geo('spike', () => new THREE.ConeGeometry(0.075, 0.2, 6)), hair);
        s.position.set(Math.sin(a) * 0.2, 0.3, Math.cos(a) * 0.2); s.rotation.set(Math.cos(a) * 0.5, 0, -Math.sin(a) * 0.5); this.head.add(s);
      }
    }
    const hat = this.guard ? 3 : o.hat;
    if (hat === 1 || hat === 3) {
      const col = hat === 3 ? 0x1a2c52 : 0xe0453d;
      const cap = mesh(sphere(0.335, 0, Math.PI * 2, 0, 1.1), col); cap.position.y = 0.02; this.head.add(cap);
      const brim = mesh(geo('brim', () => new THREE.BoxGeometry(0.34, 0.03, 0.2)), hat === 3 ? 0x101a30 : 0xb83530); brim.position.set(0, 0.1, 0.32); brim.rotation.x = 0.15; this.head.add(brim);
      if (hat === 3) { const badge = mesh(geo('badge', () => new THREE.BoxGeometry(0.07, 0.07, 0.02)), 0xffd24d); badge.position.set(0, 0.2, 0.33); this.head.add(badge); }
    } else if (hat === 2) {
      const cap = mesh(sphere(0.33, 0, Math.PI * 2, 0, 1.25), 0x3fb26a); cap.position.y = 0.02; this.head.add(cap);
      const pom = mesh(sphere(0.07), 0xffffff); pom.position.y = 0.34; this.head.add(pom);
    }

    this.root.traverse((m) => { if (m.isMesh && m.material.isMeshLambertMaterial) m.castShadow = true; });

    this.label = new THREE.Sprite(new THREE.SpriteMaterial({ map: tagTexture(meta.name || '', this.guard), transparent: true, depthWrite: false }));
    this.label.scale.set(1.6, 0.4, 1);
    this.label.position.y = 2.0;
    this.root.add(this.label);
    this.bubble = null; this.bubbleT = 0;
    this.face = -1;
    this.setFace(0);
    this.phase = Math.random() * 6;
  }

  say(text) {
    if (this.bubble) { this.root.remove(this.bubble); this.bubble.material.map.dispose(); this.bubble.material.dispose(); }
    const s = new THREE.Sprite(new THREE.SpriteMaterial({ map: bubbleTexture(text), transparent: true, depthWrite: false }));
    s.scale.set(1.9, 0.8, 1); s.position.y = 2.65;
    this.root.add(s);
    this.bubble = s; this.bubbleT = 6;
  }

  setFace(f) {
    if (f === this.face) return;
    this.face = f;
    // 0 neutral, 1 smile, 2 sad, 3 angry, 4 surprised
    this.mouth.scale.set([1, 1.7, 0.9, 1, 0.7][f], [1, 1.5, 0.8, 0.9, 2.6][f], 1);
    this.mouth.position.y = [-0.105, -0.1, -0.12, -0.115, -0.115][f];
    this.mouth.rotation.z = f === 2 ? 0 : 0;
    this.browL.rotation.z = [0, 0, 0.35, -0.4, 0][f]; this.browR.rotation.z = [0, 0, -0.35, 0.4, 0][f];
    this.browL.position.y = this.browR.position.y = [0.1, 0.112, 0.1, 0.09, 0.128][f];
    const e = f === 4 ? 1.25 : f === 1 ? 0.8 : 1;
    this.eyeL.scale.set(1, e, 1); this.eyeR.scale.set(1, e, 1);
  }

  // anim: code from the server; dt seconds; t absolute seconds
  animate(anim, dt, t) {
    const A = ANIM;
    const b = this.body, ph = (this.phase += dt * (anim === A.run ? 11 : anim === A.walk ? 7 : 0));
    if (this.bubble && (this.bubbleT -= dt) <= 0) { this.root.remove(this.bubble); this.bubble.material.map.dispose(); this.bubble.material.dispose(); this.bubble = null; }
    else if (this.bubble) this.bubble.material.opacity = Math.min(1, this.bubbleT * 2);
    b.position.set(0, 0, 0); b.rotation.set(0, 0, 0);
    this.legL.rotation.set(0, 0, 0); this.legR.rotation.set(0, 0, 0);
    this.armL.rotation.set(0, 0, 0); this.armR.rotation.set(0, 0, 0);
    this.head.rotation.set(0, 0, 0); this.torso.scale.set(1, 1, 0.86); this.torso.rotation.set(0, 0, 0);
    const sw = Math.sin(ph);
    switch (anim) {
      case A.walk: case A.run: {
        const a = anim === A.run ? 0.95 : 0.6;
        this.legL.rotation.x = sw * a; this.legR.rotation.x = -sw * a;
        this.armL.rotation.x = -sw * a * 0.9; this.armR.rotation.x = sw * a * 0.9;
        b.position.y = Math.abs(Math.cos(ph)) * 0.05;
        this.torso.rotation.y = sw * 0.12 * (anim === A.run ? 2 : 1);
        break;
      }
      case A.jump:
        this.legL.rotation.x = -0.5; this.legR.rotation.x = 0.3;
        this.armL.rotation.z = -2.6; this.armR.rotation.z = 2.6;
        break;
      case A.sleep:
        b.rotation.x = -Math.PI / 2; b.position.set(0, 0.3, -0.45);
        break;
      case A.sit:
        this.legL.rotation.x = this.legR.rotation.x = -1.45; b.position.y = -0.34;
        this.armL.rotation.x = this.armR.rotation.x = -0.6;
        break;
      case A.work:
        this.armR.rotation.x = -1.3 + Math.sin(t * 9) * 0.6; this.armL.rotation.x = -1.0;
        break;
      case A.study:
        this.head.rotation.x = 0.35; this.armL.rotation.x = this.armR.rotation.x = -0.9;
        b.position.y = Math.sin(t * 2) * 0.01;
        break;
      case A.wave:
        this.armR.rotation.z = 2.7 + Math.sin(t * 10) * 0.35;
        break;
      case A.dance:
        b.position.y = Math.abs(Math.sin(t * 6)) * 0.12;
        b.rotation.z = Math.sin(t * 6) * 0.12;
        this.armL.rotation.z = -2.3 + Math.sin(t * 6) * 0.6; this.armR.rotation.z = 2.3 + Math.sin(t * 6 + 1.5) * 0.6;
        this.legL.rotation.x = Math.sin(t * 6) * 0.4; this.legR.rotation.x = -Math.sin(t * 6) * 0.4;
        break;
      case A.laugh:
        b.position.y = Math.abs(Math.sin(t * 14)) * 0.04; this.head.rotation.x = -0.35;
        this.armL.rotation.x = this.armR.rotation.x = -0.5;
        break;
      case A.gym:
        this.armL.rotation.z = -2.9; this.armR.rotation.z = 2.9;
        b.position.y = 0.2 + Math.sin(t * 5) * 0.18;
        break;
      case A.cut:
        this.armL.rotation.x = this.armR.rotation.x = -1.2 + Math.sin(t * 12) * 0.12;
        this.torso.rotation.x = 0.1;
        break;
      default: {
        const br = 1 + Math.sin(t * 2 + this.phase) * 0.015;
        this.torso.scale.set(br, br, 0.86 * br);
        this.armL.rotation.z = 0.06; this.armR.rotation.z = -0.06;
        this.head.rotation.z = Math.sin(t * 1.3 + this.phase) * 0.03;
      }
    }
  }

  dispose() {
    this.root.traverse((o) => {
      if (o.material) {
        if (o === this.bubble) o.material.map?.dispose?.();
        o.material.dispose();
      }
    });
  }
}
