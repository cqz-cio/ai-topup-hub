/* Rooted gestures: all artwork coordinates and motion envelopes match the approved preview. */
const clamp = (x, a = 0, b = 1) => Math.max(a, Math.min(b, x));
const smooth = (a, b, x) => { const t = clamp((x - a) / (b - a)); return t * t * (3 - 2 * t); };
const turn = (p, c, a) => { const x = p[0] - c[0], y = p[1] - c[1], s = Math.sin(a), k = Math.cos(a); return [c[0] + x * k - y * s, c[1] + x * s + y * k]; };
const blend = (a, b, w) => [a[0] + (b[0] - a[0]) * w, a[1] + (b[1] - a[1]) * w];
const specs = [
    { name: 'sponge', rect: [820, 289, 397, 548], anchor: [1020, 816], shoulder: [932, 596], wrist: [864, 517], net: [939, 365], phase: .3 },
    { name: 'patrick', rect: [1234, 289, 350, 571], anchor: [1350, 847], shoulder: [1307, 603], wrist: [1295, 481], net: [1420, 365], phase: 2.1 }
];
class StationaryActors {
    constructor(seed) { this.seed = (seed ^ 0x53b6ca19) >>> 0; this.time = 0; this.enabled = true; this.amount = 1; this.lastGesture = -10; this.items = specs.map((s, i) => ({ ...s, start: -100, next: 4 + i * 3.1, duration: 3, force: false, events: 0 })); }
    random() { let s = this.seed; s ^= s << 13; s ^= s >>> 17; s ^= s << 5; this.seed = s >>> 0; return this.seed / 4294967296; }
    cue() { const actor = this.items[0]; if (this.time - actor.start < actor.duration * 1.05)
        return; actor.start = this.time; actor.duration = 3; actor.events++; this.items[1].next = this.time + 3.4; this.items[1].force = true; this.lastGesture = this.time; }
    step(dt, nearby = []) {
        dt = clamp(dt, 0, .1);
        this.amount += (Number(this.enabled) - this.amount) * Math.min(1, dt * 5);
        if (Math.abs(this.amount - Number(this.enabled)) < .0001)
            this.amount = Number(this.enabled);
        if (!this.enabled && this.amount === 0)
            return;
        this.time += dt;
        for (const actor of this.items) {
            if (this.time < actor.next || this.time - this.lastGesture < 3.3)
                continue;
            const near = nearby.some(j => Math.hypot(j.x - actor.net[0], j.y - actor.net[1]) < 195);
            if (near || actor.force) {
                actor.start = this.time;
                actor.duration = 2.8 + this.random() * .6;
                actor.events++;
                actor.force = false;
                actor.next = this.time + 8 + this.random() * 6;
                this.lastGesture = this.time;
            }
            else
                actor.next = this.time + .7;
        }
    }
    pose(actor) {
        const t = this.time - actor.start, phase = t / actor.duration;
        const anticipation = smooth(0, .19, phase) * (1 - smooth(.19, .34, phase));
        const reach = smooth(.18, .47, phase) * (1 - smooth(.56, 1, phase));
        const settle = smooth(.69, .81, phase) * (1 - smooth(.81, 1.05, phase));
        const p = actor.phase, breath = Math.sin(this.time * 1.07 + p), a = this.amount;
        return { arm: a * (.008 * Math.sin(this.time * .81 + p) + .028 * anticipation - .092 * reach + .011 * settle),
            wrist: a * (.005 * Math.sin(this.time * .81 + p - .65) - .016 * reach),
            lean: a * (.0025 * Math.sin(this.time * .65 + p) + .004 * anticipation - .009 * reach),
            bob: a * (-.7 * breath + 1.2 * anticipation - 2.4 * reach + .6 * settle),
            breath: 1 + a * .002 * breath, free: a * (.013 * Math.sin(this.time * .9 + p) - .014 * reach), phase,
            action: phase >= 0 && phase < 1.05 ? (phase < .2 ? '准备' : phase < .57 ? '抬网' : '回稳') : '轻微呼吸' };
    }
    warp(actor, p, pose = this.pose(actor)) {
        const sponge = actor.name === 'sponge', x = p[0], y = p[1];
        const arm = sponge ? Math.max(1 - smooth(920, 948, x), 1 - smooth(475, 505, y)) * (1 - smooth(604, 635, y)) :
            Math.max(1 - smooth(1316, 1340, x), 1 - smooth(421, 443, y)) * (1 - smooth(580, 628, y));
        let q = blend(p, turn(p, actor.shoulder, pose.arm), arm);
        const wrist = turn(actor.wrist, actor.shoulder, pose.arm);
        const net = arm * (1 - smooth(sponge ? 513 : 484, sponge ? 541 : 518, y));
        q = blend(q, turn(q, wrist, pose.wrist), net);
        const free = sponge ? smooth(1120, 1163, x) * smooth(640, 675, y) * (1 - smooth(728, 751, y)) :
            smooth(1440, 1498, x) * smooth(543, 580, y) * (1 - smooth(647, 688, y));
        q = blend(q, turn(q, sponge ? [1120, 675] : [1438, 628], pose.free), free);
        const root = actor.anchor, r = turn(q, root, pose.lean);
        r[1] = root[1] + (r[1] - root[1]) * pose.breath + pose.bob;
        // Original feet remain planted. Upper body and arms have only a small gesture envelope.
        const weight = 1 - smooth(sponge ? 756 : 785, sponge ? 814 : 845, y);
        return blend(q, r, weight);
    }
    snapshot() { return { time: +this.time.toFixed(3), enabled: this.enabled, amount: +this.amount.toFixed(3), horizontalTravel: 0, actors: this.items.map(a => ({ name: a.name, anchor: a.anchor, events: a.events, ...this.pose(a) })) }; }
}
export { StationaryActors, specs };
