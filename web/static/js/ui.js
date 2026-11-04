// DOM user interface. Everything user-controlled is inserted with textContent (never innerHTML).
import { SHIRTS, PANTS, SKINS, HAIRS } from './avatar.js';

const $ = (id) => document.getElementById(id);
const hex = (n) => '#' + n.toString(16).padStart(6, '0');
export const ITEM_NAMES = { noodle: 'Mì tôm', cigarette: 'Thuốc lá', wire: 'Dây thép', handle: 'Cán gỗ', pliers: 'Kìm cắt' };
const ITEM_NOTE = { cigarette: 'đồ cấm', pliers: 'đồ cấm' };

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
    document.querySelectorAll('#chat .tabs button').forEach((b) => {
      b.onclick = () => { this.channel = b.dataset.ch; document.querySelectorAll('#chat .tabs button').forEach((x) => x.classList.toggle('on', x === b)); this.renderChat(); };
    });
    $('mClose').onclick = () => this.closeModal();
    $('bInv').onclick = () => this.openInventory();
    $('bOutfit').onclick = () => this.openOutfit();
    $('bBoard').onclick = () => this.openBoard('rep');
  }

  get typing() { return document.activeElement && document.activeElement.tagName === 'INPUT'; }
  get modalOpen() { return this.modal !== null; }

  // ---- auth ----
  initAuth(onToken) {
    let reg = false;
    const set = (r) => { reg = r; $('tabLogin').classList.toggle('on', !r); $('tabReg').classList.toggle('on', r); $('nameRow').hidden = !r; $('fPass').autocomplete = r ? 'new-password' : 'current-password'; $('fGo').textContent = r ? 'Tạo tài khoản' : 'Vào trại'; };
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
    $('cellTag').textContent = 'Buồng ' + me.cell;
    $('money').textContent = me.money; $('rep').textContent = me.rep; $('points').textContent = me.points; $('sentence').textContent = me.sentence + ' ngày';
    $('stam').textContent = me.stamina; $('stamBar').style.width = me.stamina + '%';
    $('disc').textContent = me.disc; $('discBar').style.width = me.disc + '%';
    const ul = $('qlist'); ul.textContent = '';
    for (const q of me.quests.daily) ul.append(el('li', { class: q.done ? 'done' : '' }, el('span', { text: q.label }), el('span', { text: q.done ? 'xong' : `${q.prog}/${q.target}` })));
    const w = me.quests.week;
    $('qweek').textContent = `Nhiệm vụ tuần: ${Math.min(w.prog, w.target)}/${w.target}${w.done ? ' ✓' : ` (+${w.money} tiền trại)`}`;
    const iso = $('iso');
    iso.hidden = !me.iso;
    if (me.iso) iso.textContent = `Biệt giam — còn ${me.iso} giây`;
    if (this.modal === 'inv') this.openInventory();
  }
  setRoom(name) { $('room').textContent = name; }
  setClock(c) { this.clock = { ...c, at: performance.now() }; }
  tick() {
    const c = this.clock;
    if (!c) return;
    const h = (c.h + ((performance.now() - c.at) / 1000) * 24 / c.len) % 24;
    $('clock').textContent = `Ngày ${c.day} · ${String(Math.floor(h)).padStart(2, '0')}:${String(Math.floor((h % 1) * 60)).padStart(2, '0')} · ${c.phl}`;
  }
  setNet(rtt, players) { $('net').textContent = `${rtt} ms · ${players} người gần`; }
  toast(kind, text) {
    const t = el('div', { class: 'toast ' + (kind === 'ok' ? '' : kind), text });
    $('toasts').append(t);
    setTimeout(() => t.remove(), 5000);
    while ($('toasts').children.length > 5) $('toasts').firstChild.remove();
  }
  setPrompt(text) { const p = $('prompt'); p.hidden = !text; if (text) p.textContent = text; }
  showAct(label, ms) {
    $('actbar').hidden = false; $('actlabel').textContent = label;
    const f = $('actfill'); f.style.transition = 'none'; f.style.width = '0'; void f.offsetWidth;
    f.style.transition = `width ${ms}ms linear`; f.style.width = '100%';
  }
  hideAct() { $('actbar').hidden = true; }

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
    inp.addEventListener('keydown', (e) => {
      e.stopPropagation();
      if (e.key === 'Escape') { inp.blur(); return; }
      if (e.key !== 'Enter') return;
      const text = inp.value.trim();
      inp.value = ''; inp.blur();
      if (text) onSend(text, this.channel);
    });
  }
  focusChat() { $('chatin').focus(); }

  // ---- modals ----
  openModal(name, ...nodes) {
    this.modal = name;
    const body = $('mBody'); body.textContent = '';
    body.append(...nodes);
    $('modal').hidden = false;
    document.exitPointerLock?.();
  }
  closeModal() { this.modal = null; $('modal').hidden = true; }

  openInventory() {
    const me = this.me; if (!me) return;
    const grid = el('div', { class: 'grid' });
    const have = Object.entries(me.inv).filter(([, n]) => n > 0);
    for (const [k, n] of have) {
      const it = el('div', { class: 'item' }, el('b', { text: `${ITEM_NAMES[k] || k} ×${n}` }));
      if (ITEM_NOTE[k]) it.append(el('small', { text: ITEM_NOTE[k] }));
      if (k === 'noodle') it.append(el('button', { text: 'Ăn (+18 thể lực)', onclick: () => this.send({ t: 'eat', item: k }) }));
      grid.append(it);
    }
    if (!have.length) grid.append(el('p', { text: 'Túi đồ trống. Làm việc ở xưởng để nhặt nguyên liệu.' }));
    const craft = el('button', { text: 'Chế tạo Kìm cắt (2 Dây thép + 1 Cán gỗ, đứng gần bàn làm việc)', onclick: () => this.send({ t: 'craft', id: 'pliers' }) });

    const to = el('input', { placeholder: 'Tên người nhận', maxlength: '16' });
    const sel = el('select'); for (const k of Object.keys(ITEM_NAMES)) sel.append(el('option', { value: k, text: ITEM_NAMES[k] }));
    const qty = el('input', { type: 'number', min: '1', value: '1', style: 'width:70px' });
    const price = el('input', { type: 'number', min: '0', value: '10', style: 'width:90px' });
    const offer = el('button', { text: 'Gửi đề nghị bán', onclick: () => this.send({ t: 'offer', to: to.value.trim(), item: sel.value, qty: +qty.value, price: +price.value }) });
    this.openModal('inv', el('h2', { text: `Túi đồ · ${me.money} tiền trại` }), grid, el('p', {}, craft),
      el('h2', { text: 'Giao dịch với người chơi gần bạn' }), el('div', { style: 'display:flex;gap:6px;flex-wrap:wrap' }, to, sel, qty, price, offer));
  }

  openShop(items) {
    const grid = el('div', { class: 'grid' });
    for (const it of items) grid.append(el('div', { class: 'item' }, el('b', { text: it.name }), el('span', { text: `${it.price} tiền trại` }),
      el('button', { text: 'Mua', onclick: () => this.send({ t: 'buy', item: it.id }) })));
    this.openModal('shop', el('h2', { text: 'Quầy căng tin' }), grid);
  }

  showOffer(o) {
    const box = $('offerBox'); box.textContent = '';
    const done = (accept) => { this.send({ t: 'offer_reply', n: o.id, accept }); box.hidden = true; };
    box.append(el('div', { text: `${o.from} muốn bán ${o.name} ×${o.qty} giá ${o.price} tiền trại` }),
      el('div', {}, el('button', { class: 'primary', text: 'Mua', onclick: () => done(true) }), el('button', { text: 'Từ chối', onclick: () => done(false) })));
    box.hidden = false;
    setTimeout(() => { box.hidden = true; }, 30000);
  }

  openOutfit() {
    const me = this.me; if (!me) return;
    const o = { ...me.outfit };
    const rows = [['Áo', 'shirt', SHIRTS], ['Quần', 'pants', PANTS], ['Màu da', 'skin', SKINS], ['Màu tóc', 'hairc', HAIRS]];
    const nodes = [el('h2', { text: 'Trang phục & biểu cảm' })];
    for (const [label, key, pal] of rows) {
      const sw = el('div', { class: 'sw' });
      pal.forEach((c, i) => {
        const b = el('button', { class: o[key] === i ? 'on' : '', style: `background:${hex(c)}`, 'aria-label': `${label} ${i + 1}`, onclick: () => { o[key] = i; sw.querySelectorAll('button').forEach((x, j) => x.classList.toggle('on', j === i)); } });
        sw.append(b);
      });
      nodes.push(el('div', { text: label }), sw);
    }
    const mk = (label, key, names) => { const s = el('select'); names.forEach((n, i) => s.append(el('option', { value: i, text: n }))); s.value = o[key]; s.onchange = () => { o[key] = +s.value; }; return el('label', { text: label }, s); };
    nodes.push(mk('Kiểu tóc', 'hair', ['Ngắn', 'Dài', 'Dựng', 'Hói']), mk('Mũ', 'hat', ['Không', 'Mũ lưỡi trai', 'Mũ len']));
    nodes.push(el('button', { class: 'primary', text: 'Áp dụng', onclick: () => { this.send({ t: 'outfit', outfit: o }); this.closeModal(); } }));
    const faces = el('div', { class: 'sw', style: 'margin-top:14px' });
    ['Bình thường', 'Cười', 'Buồn', 'Giận', 'Ngạc nhiên'].forEach((n, i) => faces.append(el('button', { text: n, style: 'width:auto;height:auto;padding:6px 10px;border-radius:8px', onclick: () => this.send({ t: 'face', n: i }) })));
    nodes.push(el('div', { text: 'Biểu cảm' }), faces);
    this.openModal('outfit', ...nodes);
  }

  async openBoard(by) {
    const tabs = el('div', { class: 'tabs' });
    [['rep', 'Uy tín'], ['money', 'Tiền trại'], ['points', 'Cải tạo'], ['escapes', 'Vượt ngục']].forEach(([k, n]) =>
      tabs.append(el('button', { class: k === by ? 'on' : '', text: n, onclick: () => this.openBoard(k) })));
    let rows = [];
    try { rows = await (await fetch('/api/leaderboard?by=' + by)).json(); } catch { /* shown empty */ }
    const t = el('table', {}, el('tr', {}, el('th', { text: '#' }), el('th', { text: 'Tên' }), el('th', { text: 'Uy tín' }), el('th', { text: 'Tiền' }), el('th', { text: 'Cải tạo' }), el('th', { text: 'Trốn' })));
    for (const r of rows) t.append(el('tr', {}, el('td', { text: r.rank }), el('td', { text: r.name }), el('td', { text: r.rep }), el('td', { text: r.money }), el('td', { text: r.points }), el('td', { text: r.escapes })));
    this.openModal('board', el('h2', { text: 'Bảng xếp hạng' }), tabs, t);
  }
}
