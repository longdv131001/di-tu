import * as THREE from '../vendor/three.module.min.js';
import { Physics, DT } from './physics.js';
import { buildWorld } from './world.js';
import { Avatar, ANIM } from './avatar.js';
import { Net } from './net.js';
import { UI } from './ui.js';

const $ = (id) => document.getElementById(id);
const net = { current: null };
const ui = new UI((m) => net.current?.send(m));
const ACT_ANIM = { rest: ANIM.sleep, work: ANIM.work, study: ANIM.study, gym: ANIM.gym, cut: ANIM.cut };
const EMOTE_ANIM = { 1: ANIM.wave, 2: ANIM.dance, 3: ANIM.laugh, 4: ANIM.sit };
const RENDER_DELAY = 120; // ms of interpolation delay for other players

const S = {
  started: false, map: null, phys: null, world: null, scene: null, camera: null, renderer: null,
  me: null, body: { x: 42, y: 0, z: 58, vy: 0, grounded: true }, facing: 0, moving: false,
  yaw: Math.PI, pitch: 0.35, dist: 5, first: false,
  seq: 0, acc: 0, pending: [], jump: false,
  ents: new Map(), self: null, selfKey: '', act: null, emote: 0, room: 'yard', rooms: new Map(),
  keys: {}, stick: { x: 0, y: 0 }, lowGfx: false, last: performance.now(), slow: 0, nearby: 0,
};

// ---------------- boot ----------------
ui.initAuth(start);
ui.bindChat(onChatSubmit);
const saved = localStorage.getItem('dtu_token');
if (saved) start(saved);

async function start(token) {
  if (S.started) return;
  S.started = true;
  ui.showHud();
  try {
    S.map = await (await fetch('/api/map')).json();
  } catch {
    S.started = false; ui.showAuth('Không tải được bản đồ.'); return;
  }
  S.phys = new Physics(S.map);
  for (const r of S.map.rooms) S.rooms.set(r.id, r.name);
  initRenderer();
  net.current = new Net(token, { onMessage, onStatus: () => {}, onRtt: (r) => { S.rtt = r; } });
  S.last = performance.now();
  requestAnimationFrame(frame);
}

function initRenderer() {
  const canvas = $('c');
  S.renderer = new THREE.WebGLRenderer({ canvas, antialias: false, powerPreference: 'high-performance' });
  S.renderer.setPixelRatio(Math.min(devicePixelRatio, 1.5));
  S.scene = new THREE.Scene();
  S.camera = new THREE.PerspectiveCamera(70, 1, 0.1, 220);
  S.world = buildWorld(S.map, S.scene);
  onResize();
  addEventListener('resize', onResize);
  initInput(canvas);
}

function onResize() {
  S.renderer.setSize(innerWidth, innerHeight, false);
  S.camera.aspect = innerWidth / innerHeight;
  S.camera.updateProjectionMatrix();
}

// ---------------- messages ----------------
function onMessage(m) {
  switch (m.t) {
    case 'welcome': {
      S.me = m.me;
      S.body = { x: m.pos[0], y: m.pos[1], z: m.pos[2], vy: 0, grounded: true };
      S.yaw = m.pos[3]; S.facing = m.pos[3];
      S.pending = []; S.seq = 0;
      S.phys.open = new Set(m.doors);
      for (const d of S.map.doors) S.world.setDoor(d.id, S.phys.open.has(d.id));
      for (const d of S.world.doors.values()) d.t = d.open ? 1 : 0;
      for (const e of S.ents.values()) removeEnt(e);
      S.ents.clear();
      ui.setMe(m.me); ui.setClock(m.clock); ensureSelf();
      break;
    }
    case 'me': S.me = m.me; ui.setMe(m.me); ensureSelf(); break;
    case 'snap': onSnap(m); break;
    case 'door': S.world.setDoor(m.id, m.o); m.o ? S.phys.open.add(m.id) : S.phys.open.delete(m.id); break;
    case 'tp': S.body = { x: m.pos[0], y: m.pos[1], z: m.pos[2], vy: 0, grounded: true }; S.pending = []; break;
    case 'chat': ui.addChat(m); break;
    case 'toast': ui.toast(m.k, m.text); break;
    case 'clock': ui.setClock(m); break;
    case 'shop': ui.openShop(m.items); break;
    case 'offer': ui.showOffer(m); break;
    case 'act':
      S.act = m.k ? m : null;
      m.k ? ui.showAct(m.label, m.ms) : ui.hideAct();
      break;
    case 'err':
      if (['auth', 'banned', 'kicked', 'other_login'].includes(m.code)) {
        if (m.code === 'auth') localStorage.removeItem('dtu_token');
        net.current?.close(); S.started = false;
        ui.showAuth(m.msg);
      } else ui.toast('bad', m.msg);
      break;
  }
}

function onSnap(m) {
  const [x, y, z, vy, g] = m.me;
  S.pending = S.pending.filter((p) => p.seq > m.a);
  const b = { x, y, z, vy, grounded: !!g };
  for (const p of S.pending) S.phys.step(b, p.inp);
  const err = Math.hypot(b.x - S.body.x, b.y - S.body.y, b.z - S.body.z);
  if (err > 0.0005) Object.assign(S.body, b);
  if (m.rm !== S.room) { S.room = m.rm; ui.setRoom(S.rooms.get(m.rm) || 'Sân trại'); }
  const now = performance.now();
  for (const meta of m.n || []) {
    let e = S.ents.get(meta.id);
    if (e) { e.meta = meta; rebuildAvatar(e); } else {
      e = { id: meta.id, meta, buf: [], av: null, anim: 0, face: 0 };
      S.ents.set(meta.id, e); rebuildAvatar(e);
    }
  }
  S.nearby = m.p.length;
  for (const [id, ex, ey, ez, yaw, anim, face] of m.p) {
    const e = S.ents.get(id);
    if (!e) continue;
    e.buf.push({ t: now, x: ex, y: ey, z: ez, yaw });
    if (e.buf.length > 8) e.buf.shift();
    e.anim = anim; e.face = face;
  }
  for (const id of m.x || []) { const e = S.ents.get(id); if (e) { removeEnt(e); S.ents.delete(id); } }
}

function rebuildAvatar(e) {
  if (e.av) { S.scene.remove(e.av.root); e.av.dispose(); }
  e.av = new Avatar(e.meta);
  e.av.root.visible = e.buf.length > 0;
  S.scene.add(e.av.root);
}
function removeEnt(e) { if (e.av) { S.scene.remove(e.av.root); e.av.dispose(); } }

function ensureSelf() {
  if (!S.me || !S.scene) return;
  const key = JSON.stringify(S.me.outfit) + S.me.name;
  if (S.selfKey === key) return;
  S.selfKey = key;
  if (S.self) { S.scene.remove(S.self.root); S.self.dispose(); }
  S.self = new Avatar({ name: S.me.name, outfit: S.me.outfit });
  S.self.label.visible = false;
  S.scene.add(S.self.root);
}

// ---------------- input ----------------
function initInput(canvas) {
  addEventListener('keydown', (e) => {
    if (ui.typing) return;
    const k = e.key.toLowerCase();
    if (k === 'enter') { e.preventDefault(); document.exitPointerLock?.(); ui.focusChat(); return; }
    if (k === 'escape') { ui.closeModal(); return; }
    if (k === 'tab') { e.preventDefault(); ui.modalOpen ? ui.closeModal() : ui.openInventory(); return; }
    if (ui.modalOpen) return;
    S.keys[k] = true;
    if (k === ' ') { S.jump = true; e.preventDefault(); }
    else if (k === 'e') interact();
    else if (k === 'v') toggleView();
    else if (k === 'o') ui.openOutfit();
    else if (k === 'l') ui.openBoard('rep');
    else if (k >= '1' && k <= '4') emote(+k);
  });
  addEventListener('keyup', (e) => { S.keys[e.key.toLowerCase()] = false; });
  addEventListener('blur', () => { S.keys = {}; });
  canvas.addEventListener('click', () => { if (!ui.modalOpen && !ui.typing && !S.touch) canvas.requestPointerLock?.(); });
  document.addEventListener('mousemove', (e) => {
    if (document.pointerLockElement !== canvas) return;
    S.yaw -= e.movementX * 0.0025;
    S.pitch = Math.max(-0.5, Math.min(1.2, S.pitch + e.movementY * 0.0025));
  });
  canvas.addEventListener('wheel', (e) => { S.dist = Math.max(2, Math.min(9, S.dist + Math.sign(e.deltaY) * 0.5)); }, { passive: true });
  $('bGfx').onclick = () => setGfx(!S.lowGfx);

  // touch controls
  if ('ontouchstart' in window) {
    S.touch = true;
    $('touch').hidden = false; $('help').hidden = true;
    const stick = $('stick'), knob = stick.firstElementChild;
    let sid = null;
    const move = (t) => {
      const r = stick.getBoundingClientRect();
      let dx = (t.clientX - r.left - r.width / 2) / 50, dy = (t.clientY - r.top - r.height / 2) / 50;
      const l = Math.hypot(dx, dy); if (l > 1) { dx /= l; dy /= l; }
      S.stick = { x: dx, y: dy };
      knob.style.transform = `translate(${dx * 40}px,${dy * 40}px)`;
    };
    stick.addEventListener('touchstart', (e) => { sid = e.changedTouches[0].identifier; move(e.changedTouches[0]); e.preventDefault(); }, { passive: false });
    stick.addEventListener('touchmove', (e) => { for (const t of e.changedTouches) if (t.identifier === sid) move(t); e.preventDefault(); }, { passive: false });
    const end = () => { sid = null; S.stick = { x: 0, y: 0 }; knob.style.transform = ''; };
    stick.addEventListener('touchend', end); stick.addEventListener('touchcancel', end);
    let look = null;
    canvas.addEventListener('touchstart', (e) => { const t = e.changedTouches[0]; look = { id: t.identifier, x: t.clientX, y: t.clientY }; }, { passive: true });
    canvas.addEventListener('touchmove', (e) => {
      for (const t of e.changedTouches) if (look && t.identifier === look.id) {
        S.yaw -= (t.clientX - look.x) * 0.005; S.pitch = Math.max(-0.5, Math.min(1.2, S.pitch + (t.clientY - look.y) * 0.005));
        look.x = t.clientX; look.y = t.clientY;
      }
    }, { passive: true });
    $('tJump').ontouchstart = (e) => { S.jump = true; e.preventDefault(); };
    $('tUse').ontouchstart = (e) => { interact(); e.preventDefault(); };
    $('tView').ontouchstart = (e) => { toggleView(); e.preventDefault(); };
  }
}

function toggleView() { S.first = !S.first; }
function emote(n) {
  if (S.act || ui.me?.iso) return;
  S.emote = n;
  net.current?.send({ t: 'emote', n });
}
function setGfx(low) {
  S.lowGfx = low;
  S.renderer.setPixelRatio(low ? 1 : Math.min(devicePixelRatio, 1.5));
  S.renderer.setSize(innerWidth, innerHeight, false);
  $('bGfx').textContent = 'Đồ họa: ' + (low ? 'thấp' : 'cao');
}

function onChatSubmit(text, channel) {
  const send = (m) => net.current?.send(m);
  // names may contain spaces: use /w "Tên có dấu cách" nội dung, otherwise the first word is the name
  const pm = text.match(/^\/(?:w|dm)\s+(?:"([^"]+)"|(\S+))\s+(.+)$/i);
  const rp = text.match(/^\/report\s+(?:"([^"]+)"|(\S+))\s+(.+)$/i);
  if (pm) {
    send({ t: 'chat', ch: 'dm', to: pm[1] || pm[2], text: pm[3] });
  } else if (rp) {
    send({ t: 'report', to: rp[1] || rp[2], text: rp[3] });
  } else send({ t: 'chat', ch: channel === 'dm' ? 'global' : channel, text });
}

// nearest interactable (object preferred over door)
function nearest() {
  const b = S.body;
  let best = null, bd = 2.8 * 2.8;
  for (const o of S.map.objs) {
    const d = (o.x - b.x) ** 2 + (o.z - b.z) ** 2;
    if (d < bd && Math.abs(o.y - b.y) < 1.6) { best = { kind: 'obj', o }; bd = d; }
  }
  if (best) return best;
  bd = 2.8 * 2.8;
  for (const d of S.map.doors) {
    const dd = (d.x - b.x) ** 2 + (d.z - b.z) ** 2;
    if (dd < bd && Math.abs(d.y - b.y) < 1.5) { best = { kind: 'door', d }; bd = dd; }
  }
  return best;
}
const OBJ_PROMPT = { bed: 'Nghỉ ngơi', bench: 'Lao động', desk: 'Học tập', gym: 'Tập xà đơn', counter: 'Mở quầy căng tin', fence: 'Cắt hàng rào (cần 3 người + Kìm cắt)' };
function interact() {
  const n = nearest();
  if (!n) return;
  if (n.kind === 'obj') net.current?.send({ t: 'use', id: n.o.id });
  else net.current?.send({ t: 'door', id: n.d.id });
}

// ---------------- main loop ----------------
function readInput() {
  const k = S.keys;
  let mx = 0, mz = 0;
  if (k.w || k.arrowup) mz += 1;
  if (k.s || k.arrowdown) mz -= 1;
  if (k.d || k.arrowright) mx += 1;
  if (k.a || k.arrowleft) mx -= 1;
  mx += S.stick.x; mz -= S.stick.y;
  const run = !!k.shift || Math.hypot(S.stick.x, S.stick.y) > 0.85;
  const jump = S.jump; S.jump = false;
  return { mx, mz, yaw: S.yaw, run, jump };
}

function simulate(dt) {
  if (!S.me) return;
  S.acc += dt;
  if (S.acc > 0.25) S.acc = 0.05;
  while (S.acc >= DT) {
    S.acc -= DT;
    const inp = ui.me?.iso || ui.modalOpen || ui.typing ? { mx: 0, mz: 0, yaw: S.yaw, run: false, jump: false } : readInput();
    const r = S.phys.step(S.body, inp);
    S.moving = r.moving;
    if (r.moving) S.facing = r.facing;
    if (r.moving || inp.jump) S.emote = 0;
    const seq = ++S.seq;
    S.pending.push({ seq, inp });
    if (S.pending.length > 60) S.pending.shift();
    net.current?.send({ t: 'in', s: seq, mx: +inp.mx.toFixed(3), mz: +inp.mz.toFixed(3), yaw: +inp.yaw.toFixed(4), j: inp.jump || undefined, r: inp.run || undefined });
  }
}

function lerpAngle(a, b, t) { let d = ((b - a + Math.PI) % (Math.PI * 2) + Math.PI * 2) % (Math.PI * 2) - Math.PI; return a + d * t; }

function updateEntities(now, dt) {
  const rt = now - RENDER_DELAY;
  for (const e of S.ents.values()) {
    const buf = e.buf;
    if (!buf.length || !e.av) continue;
    let a = buf[0], b = buf[buf.length - 1];
    for (let i = 0; i < buf.length - 1; i++) if (buf[i].t <= rt && rt < buf[i + 1].t) { a = buf[i]; b = buf[i + 1]; break; }
    const t = b.t === a.t ? 1 : Math.max(0, Math.min(1, (rt - a.t) / (b.t - a.t)));
    const r = e.av.root;
    r.position.set(a.x + (b.x - a.x) * t, a.y + (b.y - a.y) * t, a.z + (b.z - a.z) * t);
    r.rotation.y = lerpAngle(a.yaw, b.yaw, t);
    r.visible = true;
    e.av.setFace(e.face);
    e.av.animate(e.anim, dt, now / 1000);
  }
}

function updateSelf(now, dt) {
  const s = S.self;
  if (!s) return;
  const b = S.body;
  s.root.position.set(b.x, b.y, b.z);
  const target = S.facing;
  s.root.rotation.y = lerpAngle(s.root.rotation.y, target, Math.min(1, dt * 14));
  s.root.visible = !S.first || S.act !== null;
  let anim = ANIM.idle;
  if (S.act) anim = ACT_ANIM[S.act.k] ?? ANIM.idle;
  else if (S.emote && !S.moving) anim = EMOTE_ANIM[S.emote];
  else if (!b.grounded) anim = ANIM.jump;
  else if (S.moving) anim = S.keys.shift || Math.hypot(S.stick.x, S.stick.y) > 0.85 ? ANIM.run : ANIM.walk;
  s.setFace(S.me?.face ?? 0);
  s.animate(anim, dt, now / 1000);
}

const camTarget = new THREE.Vector3(), camDir = new THREE.Vector3(), camPos = new THREE.Vector3();
function updateCamera() {
  const b = S.body, fx = Math.sin(S.yaw), fz = Math.cos(S.yaw);
  if (S.first) {
    S.camera.position.set(b.x, b.y + 1.62, b.z);
    const cp = Math.cos(-S.pitch * 0.9 + 0.2);
    S.camera.lookAt(b.x + fx * cp, b.y + 1.62 + Math.sin(-S.pitch * 0.9 + 0.2), b.z + fz * cp);
    return;
  }
  camTarget.set(b.x, b.y + 1.5, b.z);
  const cp = Math.cos(S.pitch), sp = Math.sin(S.pitch);
  camDir.set(-fx * cp, sp, -fz * cp).normalize();
  const hit = S.world.obstacle(camTarget, camDir, S.dist + 0.3);
  const d = Math.min(S.dist, hit - 0.3);
  camPos.copy(camTarget).addScaledVector(camDir, Math.max(0.6, d));
  S.camera.position.copy(camPos);
  S.camera.lookAt(camTarget);
}

function frame(now) {
  requestAnimationFrame(frame);
  const dt = Math.min(0.1, (now - S.last) / 1000);
  S.last = now;
  if (!S.started) return;
  if (!S.lowGfx) { // adaptive quality: fall back to low graphics when frames are slow
    S.slow = dt > 0.034 ? S.slow + dt : Math.max(0, S.slow - dt);
    if (S.slow > 4) { setGfx(true); ui.toast('warn', 'Đã chuyển sang đồ họa thấp để chạy mượt hơn.'); }
  }
  simulate(dt);
  updateEntities(now, dt);
  updateSelf(now, dt);
  S.world.update(dt, S.body.x, S.body.y, S.body.z);
  updateCamera();
  S.renderer.render(S.scene, S.camera);

  const n = ui.modalOpen || ui.typing ? null : nearest();
  ui.setPrompt(S.act || !n ? '' : n.kind === 'obj' ? `E: ${OBJ_PROMPT[n.o.kind] || n.o.label}` : `E: ${S.phys.open.has(n.d.id) ? 'Đóng' : 'Mở'} cửa${n.d.locked ? ' (khóa)' : ''}`);
  ui.tick();
  if ((S.frames = (S.frames || 0) + 1) % 30 === 0) ui.setNet(S.rtt || 0, Math.max(0, S.nearby - (S.ents.has(-1) ? 1 : 0)));
}
