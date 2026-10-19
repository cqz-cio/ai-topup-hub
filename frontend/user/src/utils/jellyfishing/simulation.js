/* Real-time steering simulation; rendering and artwork are deliberately separate. */
const TAU = Math.PI * 2, clamp = (v, a, b) => Math.max(a, Math.min(b, v));
const smooth = t => { t = clamp(t, 0, 1); return t * t * (3 - 2 * t); };
const wrapAngle = a => Math.atan2(Math.sin(a), Math.cos(a));
function randomSource(seed) { let state = seed >>> 0; return () => { state = (state + 0x6D2B79F5) >>> 0; let t = state; t = Math.imul(t ^ (t >>> 15), t | 1); t ^= t + Math.imul(t ^ (t >>> 7), t | 61); return ((t ^ (t >>> 14)) >>> 0) / 4294967296; }; }
class JellyWorld {
    constructor({ width = 1280, height = 720, count = 9, seed = 1, speed = 1 } = {}) {
        this.width = width;
        this.height = height;
        this.seed = seed >>> 0;
        this.random = randomSource(this.seed);
        this.speed = speed;
        this.time = 0;
        this.items = [];
        this.routes = 0;
        this.boundaryCorrections = 0;
        this.maxAcceleration = 0;
        this.nextId = 1;
        this.setCount(count);
    }
    range(a, b) { return a + (b - a) * this.random(); }
    limits(j) { const r = Math.min(j.radius, Math.min(this.width, this.height) * .23); return { left: r, right: this.width - r, top: r, bottom: this.height - r }; }
    sizeFor(depth) { return clamp(Math.min(this.width, this.height) * .17, 36, 150) * depth; }
    setCount(count) {
        count = clamp(Math.round(count), 0, 16);
        if (count < this.items.length)
            this.items.length = count;
        while (this.items.length < count) {
            const index = this.items.length, depth = this.range(.58, 1.08), size = this.sizeFor(depth), angle = this.range(-Math.PI, Math.PI);
            const j = { id: this.nextId++, depth, size, radius: size * .84, variant: index % 3, period: this.range(1.55, 2.55), phase: this.random(), x: 0, y: 0, vx: 0, vy: 0, angle,
                cruise: this.range(35, 62), wanderPhase: this.range(0, TAU), wanderRate: this.range(.22, .48), turnBias: this.range(-1, 1), routeSerial: 0, targetX: 0, targetY: 0, targetUntil: 0 };
            const b = this.limits(j), cols = Math.ceil(Math.sqrt(count * this.width / this.height)), rows = Math.ceil(count / cols);
            j.x = b.left + (b.right - b.left) * clamp(((index % cols) + this.range(.22, .78)) / cols, 0, 1);
            j.y = b.top + (b.bottom - b.top) * clamp((Math.floor(index / cols) + this.range(.22, .78)) / rows, 0, 1);
            j.vx = Math.cos(angle) * j.cruise * .7;
            j.vy = Math.sin(angle) * j.cruise * .7;
            this.items.push(j);
            this.pickTarget(j);
        }
    }
    pickTarget(j) {
        const b = this.limits(j), span = Math.hypot(b.right - b.left, b.bottom - b.top), minimum = span * .20;
        let x, y;
        // A new target is sampled whenever a route finishes; there is no timeline reset.
        for (let attempt = 0; attempt < 8; attempt++) {
            x = this.range(b.left, b.right);
            y = this.range(b.top, b.bottom);
            if (Math.hypot(x - j.x, y - j.y) > minimum)
                break;
        }
        const screenFactor = clamp(Math.min(this.width, this.height) / 720, .65, 1.25);
        // Allow a long crossing to finish. Constant short deadlines pull a wide
        // screen's swimmers back into the middle before they ever reach its sides.
        const travelTime = Math.hypot(x - j.x, y - j.y) / Math.max(8, j.cruise * screenFactor * this.speed * .82);
        j.targetX = x;
        j.targetY = y;
        j.targetUntil = this.time + travelTime + this.range(4, 7);
        j.turnBias = this.range(-1, 1);
        j.routeSerial = ++this.routes;
    }
    reroute() { for (const j of this.items)
        this.pickTarget(j); }
    resize(width, height) {
        const sx = width / this.width, sy = height / this.height;
        this.width = width;
        this.height = height;
        for (const j of this.items) {
            j.x *= sx;
            j.y *= sy;
            j.targetX *= sx;
            j.targetY *= sy;
            j.size = this.sizeFor(j.depth);
            j.radius = j.size * .84;
            const b = this.limits(j);
            j.x = clamp(j.x, b.left, b.right);
            j.y = clamp(j.y, b.top, b.bottom);
            j.targetX = clamp(j.targetX, b.left, b.right);
            j.targetY = clamp(j.targetY, b.top, b.bottom);
        }
    }
    step(dt) {
        // Fixed small integration steps are supplied by the renderer.
        dt = Math.min(dt, 1 / 30);
        this.time += dt;
        const screenFactor = clamp(Math.min(this.width, this.height) / 720, .65, 1.25), states = this.items.map(j => ({ x: j.x, y: j.y, r: j.size * .38 }));
        for (let i = 0; i < this.items.length; i++) {
            const j = this.items[i], b = this.limits(j);
            if (this.time >= j.targetUntil || Math.hypot(j.targetX - j.x, j.targetY - j.y) < Math.max(30, j.size * .45))
                this.pickTarget(j);
            j.phase = (j.phase + dt / j.period) % 1;
            const pulse = j.phase < .23 ? Math.sin(Math.PI * j.phase / .23) ** 2 : 0;
            const speed = j.cruise * screenFactor * this.speed * (.78 + .72 * pulse);
            let heading = Math.atan2(j.targetY - j.y, j.targetX - j.x);
            heading += .18 * Math.sin(this.time * j.wanderRate + j.wanderPhase) + .08 * j.turnBias;
            const desiredX = Math.cos(heading) * speed, desiredY = Math.sin(heading) * speed;
            let ax = (desiredX - j.vx) / 1.35, ay = (desiredY - j.vy) / 1.35;
            // Short-range separation keeps independent routes from becoming a clump.
            for (let k = 0; k < states.length; k++)
                if (k !== i) {
                    const other = states[k], dx = j.x - other.x, dy = j.y - other.y, d = Math.hypot(dx, dy), range = (j.size * .38 + other.r) * 1.2;
                    if (d > 1e-4 && d < range) {
                        const f = 24 * (1 - d / range) ** 2;
                        ax += dx / d * f;
                        ay += dy / d * f;
                    }
                }
            const zone = Math.min(110, Math.min(this.width, this.height) * .18), edgeForce = 130 * this.speed * this.speed;
            const push = d => edgeForce * (1 - smooth(d / zone)) ** 2;
            ax += push(j.x - b.left) - push(b.right - j.x);
            ay += push(j.y - b.top) - push(b.bottom - j.y);
            const acceleration = Math.hypot(ax, ay), limit = 180;
            if (acceleration > limit) {
                ax *= limit / acceleration;
                ay *= limit / acceleration;
            }
            this.maxAcceleration = Math.max(this.maxAcceleration, Math.hypot(ax, ay));
            j.vx += ax * dt;
            j.vy += ay * dt;
            const velocity = Math.hypot(j.vx, j.vy), maxSpeed = j.cruise * screenFactor * this.speed * 1.8;
            if (velocity > maxSpeed) {
                j.vx *= maxSpeed / velocity;
                j.vy *= maxSpeed / velocity;
            }
            j.x += j.vx * dt;
            j.y += j.vy * dt;
            // Only a last-resort numerical guard; normal steering turns before this boundary.
            if (j.x < b.left) {
                j.x = b.left;
                j.vx = Math.abs(j.vx) * .2;
                this.boundaryCorrections++;
            }
            if (j.x > b.right) {
                j.x = b.right;
                j.vx = -Math.abs(j.vx) * .2;
                this.boundaryCorrections++;
            }
            if (j.y < b.top) {
                j.y = b.top;
                j.vy = Math.abs(j.vy) * .2;
                this.boundaryCorrections++;
            }
            if (j.y > b.bottom) {
                j.y = b.bottom;
                j.vy = -Math.abs(j.vy) * .2;
                this.boundaryCorrections++;
            }
            const desiredAngle = Math.atan2(j.vy, j.vx), delta = wrapAngle(desiredAngle - j.angle);
            j.angle += clamp(delta * dt * 2.1, -.85 * dt, .85 * dt);
        }
    }
    snapshot() { return { seed: this.seed, time: this.time, count: this.items.length, routes: this.routes, boundaryCorrections: this.boundaryCorrections, items: this.items.map(j => ({ id: j.id, x: j.x, y: j.y, vx: j.vx, vy: j.vy, angle: j.angle, phase: j.phase, size: j.size, radius: j.radius, targetX: j.targetX, targetY: j.targetY, routeSerial: j.routeSerial })) }; }
}
export { JellyWorld };
