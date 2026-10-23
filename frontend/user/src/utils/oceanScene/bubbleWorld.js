// Continuous stochastic steering; no wrap, respawn, or fixed repeating path.
export class BubbleWorld {
  constructor(specs, width, height, seed = 1) {
    this.seed = seed >>> 0;
    this.width = width; this.height = height; this.elapsed = 0; this.collisions = 0;
    this.items = specs.filter(s => s.radius >= 10).map(s => {
      const angle = this.random() * Math.PI * 2, speed = 13 + this.random() * 17;
      const b = { ...s, x: s.x * width, y: s.y * height,
        vx: Math.cos(angle) * speed, vy: Math.sin(angle) * speed,
        speed, target: angle, nextTurn: 1 + this.random() * 4, lock: 0, hits: 0 };
      this.size(b);
      // Keep each complete bubble inside the viewport, including on narrow screens.
      b.x = b.r + this.random() * Math.max(0, width - 2 * b.r);
      b.y = b.r + this.random() * Math.max(0, height - 2 * b.r);
      return b;
    });
  }
  random() { let x = this.seed || 1; x ^= x << 13; x ^= x >>> 17; x ^= x << 5; this.seed = x >>> 0; return this.seed / 4294967296; }
  size(b) { const scale = Math.max(this.width / 1672, this.height / 941); b.r = Math.max(3, Math.min(b.padding * scale, this.width * .095, this.height * .095)); }
  resize(width, height) {
    for (const b of this.items) { b.x = b.x / this.width * width; b.y = b.y / this.height * height; }
    this.width = width; this.height = height;
    for (const b of this.items) { this.size(b); b.x = Math.max(b.r, Math.min(width - b.r, b.x)); b.y = Math.max(b.r, Math.min(height - b.r, b.y)); }
  }
  step(dt) {
    dt = Math.max(0, Math.min(dt, .05)); this.elapsed += dt;
    for (const b of this.items) {
      b.lock = Math.max(0, b.lock - dt);
      if (this.elapsed >= b.nextTurn && !b.lock) {
        b.target = Math.atan2(b.vy, b.vx) + (this.random() - .5) * 1.4;
        b.nextTurn = this.elapsed + 2 + this.random() * 4;
      }
      if (!b.lock) {
        b.vx += (Math.cos(b.target) * b.speed - b.vx) * Math.min(1, dt * .75);
        b.vy += (Math.sin(b.target) * b.speed - b.vy) * Math.min(1, dt * .75);
      }
      b.x += b.vx * dt; b.y += b.vy * dt;
      let bounced = false;
      if (b.x < b.r) { b.x = Math.min(this.width - b.r, 2*b.r-b.x); b.vx = Math.abs(b.vx); bounced = true; }
      else if (b.x > this.width-b.r) { b.x = Math.max(b.r, 2*(this.width-b.r)-b.x); b.vx = -Math.abs(b.vx); bounced = true; }
      if (b.y < b.r) { b.y = Math.min(this.height - b.r, 2*b.r-b.y); b.vy = Math.abs(b.vy); bounced = true; }
      else if (b.y > this.height-b.r) { b.y = Math.max(b.r, 2*(this.height-b.r)-b.y); b.vy = -Math.abs(b.vy); bounced = true; }
      if (bounced) { b.target = Math.atan2(b.vy,b.vx); b.lock = .9; b.nextTurn = this.elapsed + 1.5 + this.random()*3; b.hits++; this.collisions++; }
    }
  }
}
