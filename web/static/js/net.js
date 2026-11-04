// WebSocket client with automatic reconnect (the server keeps the character for 60 s).
export class Net {
  constructor(token, handlers) {
    this.token = token;
    this.h = handlers;
    this.rtt = 0;
    this.closedByUs = false;
    this.attempt = 0;
    this.connect();
    this.pinger = setInterval(() => this.send({ t: 'ping', ts: Date.now() }), 5000);
  }

  connect() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const ws = new WebSocket(`${proto}://${location.host}/ws`);
    this.ws = ws;
    ws.onopen = () => { this.attempt = 0; ws.send(JSON.stringify({ t: 'hello', token: this.token })); this.h.onStatus?.('open'); };
    ws.onmessage = (ev) => {
      let m; try { m = JSON.parse(ev.data); } catch { return; }
      if (m.t === 'pong') { this.rtt = Date.now() - m.ts; this.h.onRtt?.(this.rtt); return; }
      if (m.t === 'err') { if (['auth', 'banned', 'kicked', 'other_login'].includes(m.code)) this.closedByUs = true; }
      this.h.onMessage(m);
    };
    ws.onclose = () => {
      this.h.onStatus?.('closed');
      if (this.closedByUs) return;
      const wait = Math.min(8000, 500 * 2 ** this.attempt++) + Math.random() * 300;
      setTimeout(() => { if (!this.closedByUs) this.connect(); }, wait);
    };
  }

  send(o) { if (this.ws.readyState === 1) this.ws.send(JSON.stringify(o)); }
  close() { this.closedByUs = true; clearInterval(this.pinger); this.ws.close(); }
}
