/* Ambient motion has its own clock and random stream; approved jelly steering is unchanged. */
const W = 1672, H = 941, clamp = (x, a, b) => Math.max(a, Math.min(b, x)), smooth = t => { t = clamp(t, 0, 1); return t * t * (3 - 2 * t); };
const plants = [
    { name: '左侧前景海草', x: 66, top: 609, root: 887, halfWidth: 71, amplitude: 14, phase: .2 },
    { name: '左侧远处枝叶', x: 224, top: 425, root: 506, halfWidth: 39, amplitude: 5.8, phase: 1.1 },
    { name: '拱石后的水草', x: 409, top: 573, root: 665, halfWidth: 102, amplitude: 4.1, phase: 1.8 },
    { name: '右侧前景海草', x: 1635, top: 734, root: 904, halfWidth: 65, amplitude: 12.5, phase: 2.7 },
    { name: '右侧小花', x: 1593, top: 701, root: 769, halfWidth: 34, amplitude: 3.2, phase: 3.9 }
];
const marks = [
    { x: 532, y: 99, r: 15 }, { x: 492, y: 211, r: 9 }, { x: 463, y: 238, r: 11 },
    { x: 675, y: 348, r: 8 }, { x: 695, y: 383, r: 11 }, { x: 1154, y: 331, r: 14 },
    { x: 1209, y: 538, r: 11 }, { x: 1121, y: 256, r: 12 }, { x: 1617, y: 208, r: 11 }, { x: 1616, y: 310, r: 10 }
];
function rng(seed) { let s = seed >>> 0; return () => { s = (s + 0x6D2B79F5) >>> 0; let t = s; t = Math.imul(t ^ (t >>> 15), t | 1); t ^= t + Math.imul(t ^ (t >>> 7), t | 61); return ((t ^ (t >>> 14)) >>> 0) / 4294967296; }; }
function plantOffset(p, x, y, t, amount = 1) {
    const h = clamp((p.root - y) / (p.root - p.top), 0, 1), edge = 1 - smooth((Math.abs(x - p.x) / p.halfWidth - .65) / .35), tip = smooth((y - (p.top - 26)) / 24);
    const wave = Math.sin(t * .67 + p.phase - h * .75) + .24 * Math.sin(t * 1.07 + p.phase * 1.8 - h * 1.4);
    return p.amplitude * Math.pow(h, 1.65) * edge * tip * wave * amount;
}
class MarineEnvironment {
    constructor(seed) {
        this.random = rng((seed ^ 0x61C88647) >>> 0);
        this.time = 0;
        this.enabled = true;
        this.amount = 1;
        this.respawns = 0;
        this.items = [];
        for (let i = 0; i < 20; i++) {
            const mark = marks[i % marks.length], b = { id: i + 1, variant: i % marks.length };
            this.reset(b);
            if (i < marks.length) {
                b.x = mark.x;
                b.y = mark.y;
                b.r = mark.r;
                b.age = 5;
            }
            else {
                b.x = this.range(28, W - 28);
                b.y = this.range(80, H - 90);
                b.age = 5;
            }
            this.items.push(b);
        }
    }
    range(a, b) { return a + (b - a) * this.random(); }
    reset(b) { b.x = this.range(24, W - 24); b.y = H + this.range(4, 38); b.r = this.range(4, 12); b.speed = this.range(12, 25); b.phase = this.range(0, Math.PI * 2); b.drift = this.range(-2, 2); b.sway = this.range(4, 12); b.age = 0; this.respawns++; }
    step(dt) {
        dt = clamp(dt, 0, 1 / 30);
        const goal = this.enabled ? 1 : 0;
        this.amount += (goal - this.amount) * Math.min(1, dt * 5);
        if (Math.abs(goal - this.amount) < .001)
            this.amount = goal;
        if (!this.enabled && this.amount === 0)
            return;
        this.time += dt;
        for (const b of this.items) {
            b.y -= b.speed * dt;
            b.x += b.drift * dt;
            b.age += dt;
            if (b.y < -35 || b.x < -45 || b.x > W + 45)
                this.reset(b);
        }
    }
    bubblePose(b) { const rise = 1 - smooth((40 - b.y) / 70), fade = smooth(b.age / 2); return { x: b.x + Math.sin(this.time * .63 + b.phase) * b.sway, y: b.y, r: b.r, opacity: fade * rise * (b.r < 7 ? .42 : .68) }; }
    snapshot() { return { time: this.time, enabled: this.enabled, amount: this.amount, respawns: this.respawns, plants: plants.map(p => ({ name: p.name, rootOffset: plantOffset(p, p.x, p.root, this.time, this.amount), tipOffset: plantOffset(p, p.x, p.top + 7, this.time, this.amount) })), bubbles: this.items.map(b => ({ id: b.id, ...this.bubblePose(b) })) }; }
}
// Extract the existing white bubble rims; restore only their small water circles.
// Surrounding water is fitted to a local colour plane, avoiding moving rectangles.
function prepareBubbles(source) {
    const make = (w, h) => { const c = document.createElement('canvas'); c.width = w; c.height = h; return c; }, water = make(W, H), g = water.getContext('2d');
    g.drawImage(source, 0, 0, W, H);
    const original = g.getImageData(0, 0, W, H), pixels = new Uint8ClampedArray(original.data), sprites = [];
    for (const m of marks) {
        const pad = Math.ceil(m.r * 1.8), size = pad * 2 + 1, cut = make(size, size), ctx = cut.getContext('2d'), data = ctx.createImageData(size, size), coeff = [];
        for (let c = 0; c < 3; c++) {
            let n = 0, s = 0, sx = 0, sy = 0, xx = 0, yy = 0;
            for (let dy = -pad; dy <= pad; dy++)
                for (let dx = -pad; dx <= pad; dx++) {
                    const d = Math.hypot(dx, dy);
                    if (d < m.r + 4 || d > pad)
                        continue;
                    const x = m.x + dx, y = m.y + dy;
                    if (x < 0 || y < 0 || x >= W || y >= H)
                        continue;
                    const v = pixels[(y * W + x) * 4 + c];
                    n++;
                    s += v;
                    sx += dx * v;
                    sy += dy * v;
                    xx += dx * dx;
                    yy += dy * dy;
                }
            coeff.push([s / n, sx / xx, sy / yy]);
        }
        for (let dy = -pad; dy <= pad; dy++)
            for (let dx = -pad; dx <= pad; dx++) {
                const x = m.x + dx, y = m.y + dy;
                if (x < 0 || y < 0 || x >= W || y >= H)
                    continue;
                const d = Math.hypot(dx, dy), k = (y * W + x) * 4, q = ((dy + pad) * size + dx + pad) * 4, base = coeff.map(c => clamp(c[0] + c[1] * dx + c[2] * dy, 0, 255));
                const mask = 1 - smooth((d - (m.r + 1)) / 3), alpha = clamp((pixels[k] - base[0]) / Math.max(16, 255 - base[0]), 0, 1) * mask;
                data.data[q] = 241;
                data.data[q + 1] = 255;
                data.data[q + 2] = 255;
                data.data[q + 3] = Math.round(alpha * 255);
                for (let c = 0; c < 3; c++)
                    original.data[k + c] = pixels[k + c] * (1 - mask) + base[c] * mask;
            }
        ctx.putImageData(data, 0, 0);
        sprites.push({ image: cut, size, pivot: [pad, pad], r: m.r, x: m.x, y: m.y });
    }
    g.putImageData(original, 0, 0);
    return { water, sprites };
}
export { MarineEnvironment, plants, marks, plantOffset, prepareBubbles };
