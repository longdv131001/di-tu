// DOM user interface. Everything user-controlled is inserted with textContent (never innerHTML).
import { SHIRTS, PANTS, SKINS, HAIRS } from './avatar.js';

const $ = (id) => document.getElementById(id);
const hex = (n) => '#' + n.toString(16).padStart(6, '0');
export const ITEM_NAMES = { noodle: 'Mì tôm', cigarette: 'Thuốc lá', wire: 'Dây thép', handle: 'Cán gỗ', pliers: 'Kìm cắt' };
const ITEM_NOTE = { cigarette: 'đồ cấm', pliers: 'đồ cấm' };
const MM = { s: 2, ox: 4, oy: 3 };

function el(tag, props = {}, ...kids) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(props)) {
    if (k === 'class') e.className = v; else if (k === 'onclick') e.onclick = v; else if (k === 'text') e.textContent = v; else e.setAttribute(k, v);
  }
  for (const k of kids) if (k != null) e.append(k);
  return e;
}

export class UI {
  constructor(send) {
    this.send = send;
    this.channel = 'global';
    this.logs = { global: [], room: [], dm: [] };
    this.me = null;
    this.modal = null;
    this.clock = null;
    this.outfitKey = '';
    this.onOutfitDraft = null;
    document.querySelectorAll('#chat [data-ch]').forEach((b) => {
      b.onclick = () => { this.channel = b.dataset.ch; document.querySelectorAll('#chat [data-ch]').forEach((x) => x.classList.toggle('on', x === b)); this.renderChat(); };
    });
    $('mClose').onclick = () => this.closeModal();
    $('bInv').onclick = () => this.openInventory();
    $('bOutfit').onclick = () => this.openOutfit();
    $('bBoard').onclick = () => this.openBoard('rep');
    $('bQuest').onclick = () => { const q = $('quests'); q.hidden = !q.hidden; $('bQuest').classList.toggle('on', !q.hidden); };
    $('chatToggle').onclick = () => { const c = $('chat'); c.classList.toggle('min'); $('chatToggle').textContent = c.classList.contains('min') ? '+' : '–'; };
    setTimeout(() => { $('hint').style.opacity = 0; }, 25000);
  }

  get typing() { return document.activeElement && document.activeElement.tagName === 'INPUT'; }
  get modalOpen() { return this.modal !== null; }

  // ---- auth ----
  initAuth(onToken) {
    let reg = false;
    const set = (r) => { reg = r; $('tabLogin').classList.toggle('on', !r); $('tabReg').classList.toggle('on', r); $('nameRow').hidden = !r; $('fPass').autocomplete = r ? 'new-password' : 'current-password'; $('fGo').textContent = r ? 'Tạo nhân vật' : 'Vào trại'; };
    $('tabLogin').onclick = () => set(false);
    $('tabReg').onclick = () => set(true);
    $('authForm').onsubmit = async (e) => {
      e.preventDefault();
      $('authMsg').textContent = '';
      $('fGo').disabled = true;
      try {
        const body = { username: $('fUser').value.trim(), password: $('fPass').value, name: $('fName').value.trim() };
        const r = await fetch(reg ? '/api/register' : '/api/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
        const j = await r.json().catch(() => ({}));
        if (!r.ok) throw new Error(j.error || 'Không thể kết nối máy chủ');
        localStorage.setItem('dtu_token', j.token);
        onToken(j.token);
      } catch (err) { $('authMsg').textContent = err.message; } finally { $('fGo').disabled = false; }
    };
  }
  showAuth(msg) { $('auth').hidden = false; $('hud').hidden = true; if (msg) $('authMsg').textContent = msg; }
  showHud() { $('auth').hidden = true; $('hud').hidden = false; }

  // ---- HUD ----
  setMe(me) {
    this.me = me;
    $('pname').textContent = me.name;
    const lvl = 1 + Math.floor(me.points / 5);
    $('lvl').textContent = 'Cấp ' + lvl;
    $('xpFill').style.width = ((me.points % 5) / 5) * 100 + '%';
    $('money').textContent = me.money; $('rep').textContent = me.rep; $('sentence').textContent = me.sentence + ' ngày';
    $('stam').textContent = me.stamina; $('stamBar').style.width = me.stamina + '%';
    $('disc').textContent = me.disc; $('discBar').style.width = me.disc + '%';
    const key = JSON.stringify(me.outfit);
    if (key !== this.outfitKey) { this.outfitKey = key; this.drawPortrait(me.outfit); }
    const ul = $('qlist'); ul.textContent = '';
    for (const q of me.quests.daily) ul.append(el('li', { class: q.done ? 'done' : '' }, el('span', { text: q.label }), el('b', { text: q.done ? 'xong' : `${q.prog}/${q.target}` })));
    const w = me.quests.week;
    $('qweek').textContent = `Nhiệm vụ tuần: ${Math.min(w.prog, w.target)}/${w.target}${w.done ? ' · hoàn thành' : ` · thưởng ${w.money} tiền trại`}`;
    const iso = $('iso');
    iso.hidden = !me.iso;
    if (me.iso) iso.textContent = `Biệt giam — còn ${me.iso} giây`;
    if (this.modal === 'inv') this.openInventory();
  }

  drawPortrait(o) {
    const c = $('portrait'), g = c.getContext('2d');
    g.clearRect(0, 0, 72, 72);
    const bg = g.createLinearGradient(0, 0, 0, 72); bg.addColorStop(0, '#cfe9ff'); bg.addColorStop(1, '#a9d8ff');
    g.fillStyle = bg; g.fillRect(0, 0, 72, 72);
    const skin = hex(SKINS[o.skin % SKINS.length]), hair = hex(HAIRS[o.hairc % HAIRS.length]);
    g.fillStyle = hex(SHIRTS[o.shirt % SHIRTS.length]);
    g.beginPath(); g.ellipse(36, 78, 26, 20, 0, 0, Math.PI * 2); g.fill();
    if (o.hair === 1) { g.fillStyle = hair; g.beginPath(); g.ellipse(36, 40, 24, 26, 0, 0, Math.PI * 2); g.fill(); }
    g.fillStyle = skin; g.beginPath(); g.arc(36, 36, 20, 0, Math.PI * 2); g.fill();
    if (o.hair !== 3) { g.fillStyle = hair; g.beginPath(); g.arc(36, 34, 21.5, Math.PI * 1.06, Math.PI * 1.94); g.lineTo(36, 28); g.fill(); }
    if (o.hair === 2) for (let k = -2; k <= 2; k++) { g.beginPath(); g.moveTo(36 + k * 8 - 4, 17); g.lineTo(36 + k * 8, 7 + Math.abs(k) * 2); g.lineTo(36 + k * 8 + 4, 17); g.fill(); }
    if (o.hat === 1) { g.fillStyle = '#e0453d'; g.beginPath(); g.arc(36, 32, 22, Math.PI, 0); g.fill(); g.fillRect(34, 28, 28, 5); }
    if (o.hat === 2) { g.fillStyle = '#3fb26a'; g.beginPath(); g.arc(36, 30, 22, Math.PI, 0); g.fill(); g.fillStyle = '#fff'; g.beginPath(); g.arc(36, 6, 5, 0, Math.PI * 2); g.fill(); }
    g.fillStyle = '#fff'; for (const x of [28, 44]) { g.beginPath(); g.arc(x, 38, 5.5, 0, Math.PI * 2); g.fill(); }
    g.fillStyle = '#1b1410'; for (const x of [28, 44]) { g.beginPath(); g.arc(x, 39, 3.4, 0, Math.PI * 2); g.fill(); }
    g.fillStyle = 'rgba(255,120,140,.55)'; for (const x of [21, 51]) { g.beginPath(); g.arc(x, 46, 4, 0, Math.PI * 2); g.fill(); }
    g.strokeStyle = '#8a3a3a'; g.lineWidth = 2; g.beginPath(); g.arc(36, 46, 5, 0.2, Math.PI - 0.2); g.stroke();
  }

  setRoom(name) { $('roomRibbon').textContent = name; }
  setClock(c) { this.clock = { ...c, at: performance.now() }; }
  tick() {
    const c = this.clock;
    if (!c) return;
    const h = (c.h + ((performance.now() - c.at) / 1000) * 24 / c.len) % 24;
    $('clockTime').textContent = `${String(Math.floor(h)).padStart(2, '0')}:${String(Math.floor((h % 1) * 60)).padStart(2, '0')}`;
    $('clockPhase').textContent = `Ngày ${c.day} · ${c.phl}`;
    const night = h < 6 || h >= 19;
    if (this.night !== night) { this.night = night; $('clockIcon').firstElementChild.setAttribute('href', night ? '#i-moon' : '#i-sun'); }
  }
  setNet(rtt, players) { $('net').textContent = `${rtt} ms · ${players} người gần bạn`; }
  toast(kind, text) {
    const t = el('div', { class: 'toast ' + (kind === 'ok' ? '' : kind), text });
    $('toasts').append(t);
    setTimeout(() => t.remove(), 5000);
    while ($('toasts').children.length > 4) $('toasts').firstChild.remove();
  }
  setCtx(text, onclick) {
    const b = $('ctx');
    b.hidden = !text;
    if (text) { $('ctxText').textContent = text; b.onclick = onclick; }
  }
  showAct(label, ms) {
    $('actbar').hidden = false; $('actlabel').textContent = label;
    const f = $('actfill'); f.style.transition = 'none'; f.style.width = '0'; void f.offsetWidth;
    f.style.transition = `width ${ms}ms linear`; f.style.width = '100%';
  }
  hideAct() { $('actbar').hidden = true; }

  // ---- minimap ----
  initMinimap(map) {
    this.map = map;
    const c = document.createElement('canvas'); c.width = 176; c.height = 134;
    const g = c.getContext('2d');
    g.fillStyle = '#9fd17a'; g.fillRect(0, 0, 176, 134);
    const R = (x0, z0, x1, z1, col) => { g.fillStyle = col; g.fillRect(MM.ox + x0 * MM.s, MM.oy + z0 * MM.s, (x1 - x0) * MM.s, (z1 - z0) * MM.s); };
    R(0, 0, map.w, map.d, '#e9e3d3');
    R(40.4, 30, 43.6, 64, '#fbf7ec'); R(14, 28.6, 78, 31.4, '#fbf7ec');
    R(44, 47, 59, 62, '#de7d52');
    const col = { hall: '#bcd5ee', corridor: '#d4e5f7', canteen: '#f4cf93', classroom: '#bfe3a8', workshop: '#f2e08a', office: '#56627a', iso1: '#7a8499', iso2: '#7a8499' };
    this.cellRooms = [];
    for (const r of map.rooms) {
      if (r.id.startsWith('cell')) { this.cellRooms.push(r); continue; }
      if (r.f === 0 || r.id === 'corridor') R(r.x0, r.z0, r.x1, r.z1, col[r.id] || '#ddd');
    }
    g.strokeStyle = '#7a5a3a'; g.lineWidth = 2; g.strokeRect(MM.ox, MM.oy, map.w * MM.s, map.d * MM.s);
    g.strokeStyle = 'rgba(80,60,40,.55)'; g.lineWidth = 1;
    for (const r of map.rooms) if (r.f === 0) g.strokeRect(MM.ox + r.x0 * MM.s, MM.oy + r.z0 * MM.s, (r.x1 - r.x0) * MM.s, (r.z1 - r.z0) * MM.s);
    g.fillStyle = '#4a3524'; g.font = '800 9px Nunito, sans-serif'; g.textAlign = 'center';
    for (const [t, x, z] of [['Buồng giam', 20, 14], ['Căng tin', 49, 14], ['Học', 67, 14], ['Xưởng', 17, 45]]) g.fillText(t, MM.ox + x * MM.s, MM.oy + z * MM.s + 3);
    this.mmBase = c;
  }

  drawMinimap(me, others) {
    const g = $('mm').getContext('2d');
    if (!this.mmBase) return;
    g.drawImage(this.mmBase, 0, 0);
    const up = me.y > 2;
    $('mmFloor').textContent = up ? 'Tầng 2' : '';
    if (up) {
      g.fillStyle = 'rgba(255,255,255,.55)'; g.fillRect(MM.ox + 4 * MM.s, MM.oy + 4 * MM.s, 32 * MM.s, 20 * MM.s);
      for (const r of this.cellRooms) { g.fillStyle = 'rgba(255,213,120,.75)'; g.fillRect(MM.ox + r.x0 * MM.s, MM.oy + r.z0 * MM.s, (r.x1 - r.x0) * MM.s, (r.z1 - r.z0) * MM.s); g.strokeStyle = '#a77b3a'; g.strokeRect(MM.ox + r.x0 * MM.s, MM.oy + r.z0 * MM.s, (r.x1 - r.x0) * MM.s, (r.z1 - r.z0) * MM.s); }
    }
    for (const o of others) {
      g.fillStyle = o.guard ? '#e5413d' : '#2f7de0'; g.strokeStyle = '#fff'; g.lineWidth = 1.5;
      g.beginPath(); g.arc(MM.ox + o.x * MM.s, MM.oy + o.z * MM.s, o.guard ? 4 : 3.2, 0, Math.PI * 2); g.fill(); g.stroke();
    }
    const px = MM.ox + me.x * MM.s, pz = MM.oy + me.z * MM.s;
    g.save(); g.translate(px, pz); g.rotate(-me.facing + Math.PI); // arrow points where the character looks
    g.fillStyle = '#ff7a1a'; g.strokeStyle = '#fff'; g.lineWidth = 2;
    g.beginPath(); g.moveTo(0, 7); g.lineTo(-5, -4); g.lineTo(0, -1.5); g.lineTo(5, -4); g.closePath(); g.stroke(); g.fill();
    g.restore();
  }

  // ---- chat ----
  addChat(m) {
    const ch = m.ch === 'sys' ? 'global' : m.ch;
    const log = this.logs[ch] || this.logs.global;
    log.push(m);
    if (log.length > 80) log.shift();
    if (ch === this.channel) this.renderChat();
  }
  renderChat() {
    const box = $('chatlog'); box.textContent = '';
    for (const m of this.logs[this.channel]) {
      const row = el('div', { class: m.ch === 'sys' ? 'sys' : m.ch === 'dm' ? 'dm' : '' });
      if (m.ch === 'sys') row.textContent = m.text;
      else {
        const mine = this.me && m.from === this.me.name;
        const who = m.ch === 'dm' ? (mine ? `Bạn → ${m.to}` : `${m.from} → bạn`) : m.from;
        const b = el('b', { text: who + ': ' });
        const target = m.ch === 'dm' ? (mine ? m.to : m.from) : m.from;
        b.onclick = () => { $('chatin').value = /\s/.test(target) ? `/w "${target}" ` : `/w ${target} `; $('chatin').focus(); };
        row.append(b, document.createTextNode(m.text));
      }
      box.append(row);
    }
    box.scrollTop = box.scrollHeight;
  }
  bindChat(onSend) {
    const inp = $('chatin');
    inp.placeholder = 'Nhấn Enter để chat · /w Tên nội dung';
    inp.addEventListener('keydown', (e) => {
      e.stopPropagation();
      if (e.key === 'Escape') { inp.blur(); return; }
      if (e.key !== 'Enter') return;
      const text = inp.value.trim();
      inp.value = ''; inp.blur();
      if (text) onSend(text, this.channel);
    });
  }
  focusChat() { $('chat').classList.remove('min'); $('chatToggle').textContent = '–'; $('chatin').focus(); }

  // ---- modals ----
  openModal(name, nodes, side = false) {
    this.modal = name;
    const body = $('mBody'); body.textContent = '';
    body.append(...nodes);
    $('mCard').classList.toggle('side', side);
    $('modal').hidden = false;
    $('modal').classList.toggle('sideov', side);
    $('modal').style.background = side ? 'transparent' : '';
    $('modal').style.pointerEvents = side ? 'none' : '';
    $('mCard').style.pointerEvents = 'auto';
    document.exitPointerLock?.();
  }
  closeModal() {
    if (this.modal === 'outfit') this.onOutfitDraft?.(null);
    this.modal = null; $('modal').hidden = true;
  }

  openInventory() {
    const me = this.me; if (!me) return;
    const grid = el('div', { class: 'grid' });
    const have = Object.entries(me.inv).filter(([, n]) => n > 0);
    for (const [k, n] of have) {
      const it = el('div', { class: 'item' }, el('b', { text: `${ITEM_NAMES[k] || k} ×${n}` }));
      if (ITEM_NOTE[k]) it.append(el('small', { text: ITEM_NOTE[k] }));
      if (k === 'noodle') it.append(el('button', { class: 'btn', text: 'Ăn (+18 thể lực)', onclick: () => this.send({ t: 'eat', item: k }) }));
      grid.append(it);
    }
    if (!have.length) grid.append(el('p', { text: 'Túi đồ trống. Làm việc ở xưởng để nhặt nguyên liệu.' }));
    const craft = el('button', { class: 'btn', text: 'Chế tạo Kìm cắt (2 Dây thép + 1 Cán gỗ, đứng gần bàn làm việc)', onclick: () => this.send({ t: 'craft', id: 'pliers' }) });
    const to = el('input', { placeholder: 'Tên người nhận', maxlength: '16' });
    const sel = el('select'); for (const k of Object.keys(ITEM_NAMES)) sel.append(el('option', { value: k, text: ITEM_NAMES[k] }));
    const qty = el('input', { type: 'number', min: '1', value: '1', style: 'width:70px' });
    const price = el('input', { type: 'number', min: '0', value: '10', style: 'width:90px' });
    const offer = el('button', { class: 'btn primary', text: 'Gửi đề nghị bán', onclick: () => this.send({ t: 'offer', to: to.value.trim(), item: sel.value, qty: +qty.value, price: +price.value }) });
    this.openModal('inv', [el('h2', { text: `Túi đồ · ${me.money} tiền trại` }), grid, el('p', {}, craft),
      el('h2', { text: 'Giao dịch với người chơi gần bạn' }), el('div', { style: 'display:flex;gap:6px;flex-wrap:wrap' }, to, sel, qty, price, offer)]);
  }

  openShop(items) {
    const grid = el('div', { class: 'grid' });
    for (const it of items) grid.append(el('div', { class: 'item' }, el('b', { text: it.name }), el('span', { text: `${it.price} tiền trại` }),
      el('button', { class: 'btn primary', text: 'Mua', onclick: () => this.send({ t: 'buy', item: it.id }) })));
    this.openModal('shop', [el('h2', { text: 'Quầy căng tin' }), grid]);
  }

  showOffer(o) {
    const box = $('offerBox'); box.textContent = '';
    const done = (accept) => { this.send({ t: 'offer_reply', n: o.id, accept }); box.hidden = true; };
    box.append(el('div', { text: `${o.from} muốn bán ${o.name} ×${o.qty} giá ${o.price} tiền trại` }),
      el('div', {}, el('button', { class: 'btn primary', text: 'Mua', onclick: () => done(true) }), el('button', { class: 'btn', text: 'Từ chối', onclick: () => done(false) })));
    box.hidden = false;
    setTimeout(() => { box.hidden = true; }, 30000);
  }

  openOutfit() {
    const me = this.me; if (!me) return;
    const o = { ...me.outfit };
    const draft = () => this.onOutfitDraft?.({ ...o });
    const rows = [['Áo', 'shirt', SHIRTS], ['Quần', 'pants', PANTS], ['Màu da', 'skin', SKINS], ['Màu tóc', 'hairc', HAIRS]];
    const nodes = [el('h2', { text: 'Trang phục & biểu cảm' })];
    for (const [label, key, pal] of rows) {
      const sw = el('div', { class: 'sw' });
      pal.forEach((c, i) => {
        sw.append(el('button', { class: o[key] === i ? 'on' : '', style: `background:${hex(c)}`, 'aria-label': `${label} ${i + 1}`, onclick: () => { o[key] = i; sw.querySelectorAll('button').forEach((x, j) => x.classList.toggle('on', j === i)); draft(); } }));
      });
      nodes.push(el('div', { text: label }), sw);
    }
    const mk = (label, key, names) => { const s = el('select'); names.forEach((n, i) => s.append(el('option', { value: i, text: n }))); s.value = o[key]; s.onchange = () => { o[key] = +s.value; draft(); }; return el('label', { text: label + ' ' }, s); };
    nodes.push(mk('Kiểu tóc', 'hair', ['Ngắn', 'Dài', 'Dựng', 'Hói']), el('br'), mk('Mũ', 'hat', ['Không', 'Mũ lưỡi trai', 'Mũ len']));
    const faces = el('div', { class: 'sw', style: 'margin-top:12px' });
    ['Bình thường', 'Cười', 'Buồn', 'Giận', 'Ngạc nhiên'].forEach((n, i) => faces.append(el('button', { class: 'btn', text: n, style: 'width:auto;height:auto;border-radius:12px;padding:5px 10px', onclick: () => this.send({ t: 'face', n: i }) })));
    nodes.push(el('div', { text: 'Biểu cảm' }), faces);
    nodes.splice(1, 0, el('button', { class: 'btn primary big', text: 'Lưu trang phục', onclick: () => { this.send({ t: 'outfit', outfit: o }); this.modal = null; $('modal').hidden = true; } }));
    this.openModal('outfit', nodes, true);
  }

  async openBoard(by) {
    const tabs = el('div', { class: 'seg' });
    [['rep', 'Uy tín'], ['money', 'Tiền trại'], ['points', 'Cải tạo'], ['escapes', 'Vượt ngục']].forEach(([k, n]) =>
      tabs.append(el('button', { class: k === by ? 'on' : '', text: n, onclick: () => this.openBoard(k) })));
    let rows = [];
    try { rows = await (await fetch('/api/leaderboard?by=' + by)).json(); } catch { /* shown empty */ }
    const t = el('table', {}, el('tr', {}, el('th', { text: '#' }), el('th', { text: 'Tên' }), el('th', { text: 'Uy tín' }), el('th', { text: 'Tiền' }), el('th', { text: 'Cải tạo' }), el('th', { text: 'Trốn' })));
    for (const r of rows) t.append(el('tr', {}, el('td', { text: r.rank }), el('td', { text: r.name }), el('td', { text: r.rep }), el('td', { text: r.money }), el('td', { text: r.points }), el('td', { text: r.escapes })));
    this.openModal('board', [el('h2', { text: 'Bảng xếp hạng' }), tabs, t]);
  }
}
