// Procedural low-poly humanoid with outfit, face expressions and animation poses.
import * as THREE from '../vendor/three.module.min.js';

export const SHIRTS = [0xd9622b, 0x3b78c4, 0x4aa86b, 0xc9a227, 0x8e5bc9, 0xd14a6b, 0x2f9aa0, 0x777b82, 0xe0e0e0, 0x2c3e50, 0xb5651d, 0x6b8e23];
export const PANTS = [0x2c3e50, 0x444441, 0x5a4a3a, 0x3b5a8c, 0x6b6b3a, 0x7b3a3a, 0x1f1f1f, 0x8a8f94];
export const SKINS = [0xf6d3b3, 0xe8b88f, 0xd49a6a, 0xb97a4b, 0x8d5a34, 0x5e3a22];
export const HAIRS = [0x1b1b1b, 0x4a2f1b, 0x7a5230, 0xc9a24a, 0xb0b0b0, 0x8a2b2b, 0x2b4a8a, 0x6a3a8a];
export const ANIM = { idle: 0, walk: 1, run: 2, jump: 3, sleep: 4, work: 5, study: 6, sit: 7, wave: 8, dance: 9, laugh: 10, gym: 11, eat: 12, cut: 13 };

const box = (w, h, d, color) => new THREE.Mesh(new THREE.BoxGeometry(w, h, d), new THREE.MeshLambertMaterial({ color }));
const nameCache = new Map();

function nameTexture(name) {
  if (nameCache.has(name)) return nameCache.get(name);
  const c = document.createElement('canvas');
  c.width = 256; c.height = 64;
  const g = c.getContext('2d');
  g.font = '600 30px system-ui, sans-serif';
  g.textAlign = 'center'; g.textBaseline = 'middle';
  g.lineWidth = 6; g.strokeStyle = 'rgba(0,0,0,.75)';
  g.strokeText(name, 128, 34);
  g.fillStyle = '#fff';
  g.fillText(name, 128, 34);
  const t = new THREE.CanvasTexture(c);
  nameCache.set(name, t);
  return t;
}

export class Avatar {
  constructor(meta) {
    this.root = new THREE.Group();   // placed at the feet
    this.body = new THREE.Group();   // posed (lying, sitting, bouncing)
    this.root.add(this.body);
    this.guard = !!meta.guard;
    const o = meta.outfit || { shirt: 0, pants: 0, skin: 0, hair: 0, hairc: 0, hat: 0 };
    const shirt = this.guard ? 0x2457a6 : SHIRTS[o.shirt % SHIRTS.length];
    const pants = this.guard ? 0x1d2433 : PANTS[o.pants % PANTS.length];
    const skin = SKINS[o.skin % SKINS.length];

    this.legL = this.leg(-0.13, pants); this.legR = this.leg(0.13, pants);
    this.torso = box(0.52, 0.62, 0.28, shirt); this.torso.position.y = 1.1;
    this.head = new THREE.Group(); this.head.position.y = 1.55;
    const skull = box(0.34, 0.34, 0.32, skin); skull.position.y = 0.17;
    this.head.add(skull);
    this.armL = this.arm(-0.34, shirt, skin); this.armR = this.arm(0.34, shirt, skin);
    this.body.add(this.legL, this.legR, this.torso, this.head, this.armL, this.armR);

    this.eyeL = box(0.06, 0.07, 0.02, 0x111111); this.eyeR = box(0.06, 0.07, 0.02, 0x111111);
    this.eyeL.position.set(-0.08, 0.22, 0.165); this.eyeR.position.set(0.08, 0.22, 0.165);
    this.mouth = box(0.12, 0.03, 0.02, 0x7a2b2b); this.mouth.position.set(0, 0.08, 0.165);
    this.browL = box(0.09, 0.02, 0.02, 0x222222); this.browR = box(0.09, 0.02, 0.02, 0x222222);
    this.browL.position.set(-0.08, 0.28, 0.165); this.browR.position.set(0.08, 0.28, 0.165);
    this.head.add(this.eyeL, this.eyeR, this.mouth, this.browL, this.browR);

    const hair = HAIRS[o.hairc % HAIRS.length];
    if (!this.guard) {
      if (o.hair === 0) { const h = box(0.36, 0.1, 0.34, hair); h.position.y = 0.37; this.head.add(h); }
      if (o.hair === 1) {
        const h = box(0.36, 0.1, 0.34, hair); h.position.y = 0.37;
        const b = box(0.36, 0.34, 0.08, hair); b.position.set(0, 0.17, -0.18);
        this.head.add(h, b);
      }
      if (o.hair === 2) for (let k = -1; k <= 1; k++) { const s = box(0.1, 0.16, 0.1, hair); s.position.set(k * 0.12, 0.42, 0); this.head.add(s); }
    }
    const hat = this.guard ? 1 : o.hat;
    if (hat === 1) { // cap
      const top = box(0.38, 0.1, 0.36, this.guard ? 0x14284d : 0xc23b3b); top.position.y = 0.4;
      const brim = box(0.3, 0.03, 0.14, this.guard ? 0x14284d : 0xc23b3b); brim.position.set(0, 0.36, 0.22);
      this.head.add(top, brim);
    } else if (hat === 2) { const t = box(0.38, 0.14, 0.36, 0x3a7d44); t.position.y = 0.4; this.head.add(t); }

    this.label = new THREE.Sprite(new THREE.SpriteMaterial({ map: nameTexture(meta.name || ''), transparent: true }));
    this.label.scale.set(1.5, 0.375, 1);
    this.label.position.y = 2.25;
    this.root.add(this.label);
    this.face = -1;
    this.setFace(0);
    this.phase = Math.random() * 6;
  }

  leg(x, color) { const g = new THREE.Group(); g.position.set(x, 0.8, 0); const m = box(0.2, 0.8, 0.22, color); m.position.y = -0.4; g.add(m); return g; }
  arm(x, shirt, skin) {
    const g = new THREE.Group(); g.position.set(x, 1.4, 0);
    const m = box(0.16, 0.56, 0.18, shirt); m.position.y = -0.28;
    const hand = box(0.15, 0.14, 0.17, skin); hand.position.y = -0.62;
    g.add(m, hand); return g;
  }

  setFace(f) {
    if (f === this.face) return;
    this.face = f;
    // 0 neutral, 1 smile, 2 sad, 3 angry, 4 surprised
    const w = [0.12, 0.16, 0.1, 0.12, 0.07][f], h = [0.03, 0.05, 0.03, 0.03, 0.08][f];
    this.mouth.scale.set(w / 0.12, h / 0.03, 1);
    this.mouth.position.y = [0.08, 0.09, 0.06, 0.07, 0.07][f];
    this.browL.rotation.z = [0, 0, -0.35, 0.45, 0][f];
    this.browR.rotation.z = [0, 0, 0.35, -0.45, 0][f];
    this.browL.position.y = this.browR.position.y = [0.28, 0.29, 0.28, 0.27, 0.32][f];
    const e = f === 4 ? 1.5 : 1;
    this.eyeL.scale.y = this.eyeR.scale.y = e;
  }

  // anim: code from the server; speed: 0..1 movement factor used for walking cycles
  animate(anim, dt, t) {
    const A = ANIM;
    const b = this.body, ph = (this.phase += dt * (anim === A.run ? 11 : anim === A.walk ? 7 : 0));
    // reset pose
    b.position.set(0, 0, 0); b.rotation.set(0, 0, 0);
    this.legL.rotation.set(0, 0, 0); this.legR.rotation.set(0, 0, 0);
    this.armL.rotation.set(0, 0, 0); this.armR.rotation.set(0, 0, 0);
    this.head.rotation.set(0, 0, 0); this.torso.scale.set(1, 1, 1);
    const sw = Math.sin(ph);
    switch (anim) {
      case A.walk: case A.run: {
        const a = anim === A.run ? 0.95 : 0.6;
        this.legL.rotation.x = sw * a; this.legR.rotation.x = -sw * a;
        this.armL.rotation.x = -sw * a * 0.9; this.armR.rotation.x = sw * a * 0.9;
        b.position.y = Math.abs(Math.cos(ph)) * 0.05;
        break;
      }
      case A.jump:
        this.legL.rotation.x = -0.5; this.legR.rotation.x = 0.3;
        this.armL.rotation.z = -2.6; this.armR.rotation.z = 2.6;
        break;
      case A.sleep:
        b.rotation.x = -Math.PI / 2; b.position.set(0, 0.28, -0.4);
        break;
      case A.sit:
        this.legL.rotation.x = this.legR.rotation.x = -1.45; b.position.y = -0.42;
        this.armL.rotation.x = this.armR.rotation.x = -0.6;
        break;
      case A.work:
        this.armR.rotation.x = -1.3 + Math.sin(t * 9) * 0.6; this.armL.rotation.x = -1.0;
        this.torso.rotation.x = 0;
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
        this.torso.scale.set(br, br, br);
        this.armL.rotation.z = 0.05; this.armR.rotation.z = -0.05;
      }
    }
  }

  dispose() {
    this.root.traverse((o) => { if (o.geometry) o.geometry.dispose(); if (o.material && o.material.map !== undefined && o !== this.label) o.material.dispose(); });
  }
}
