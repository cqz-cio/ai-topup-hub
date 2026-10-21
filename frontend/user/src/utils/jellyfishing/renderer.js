import { JellyWorld } from './simulation.js';
import * as MarineMotion from './environment.js';
import { StationaryActors } from './actors.js';
import { loadSceneArtwork } from './artwork.js';
import { actorCamera } from './composition.js';
const ART_W = 1672, ART_H = 941;
const configs = [
    { rect: [875, 94, 239, 226], pivot: [83, 80], unit: 1, polygon: [[875, 96], [1034, 94], [1066, 174], [1111, 208], [1114, 305], [1060, 320], [968, 318], [911, 256], [884, 207]] },
    { rect: [1177, 200, 164, 153], pivot: [58, 60], unit: .67, polygon: [[1181, 205], [1278, 200], [1302, 259], [1338, 267], [1341, 334], [1279, 353], [1217, 349], [1177, 284]] },
    { rect: [1484, 207, 132, 101], pivot: [51, 38], unit: .45, polygon: [[1492, 209], [1556, 207], [1610, 251], [1616, 296], [1541, 308], [1484, 259]] },
    { rect: [42, 397, 70, 84], pivot: [38, 35], unit: .28, polygon: [[42, 402], [105, 397], [112, 456], [86, 481], [43, 476]] },
    { rect: [107, 439, 59, 64], pivot: [31, 26], unit: .23, polygon: [[109, 442], [159, 439], [166, 482], [145, 503], [107, 489]] }
];
function trace(ctx, poly) { ctx.beginPath(); poly.forEach(([x, y], i) => i ? ctx.lineTo(x, y) : ctx.moveTo(x, y)); ctx.closePath(); }
function surface(w = ART_W, h = ART_H) { const c = document.createElement('canvas'); c.width = w; c.height = h; return c; }
function crop(source, c) { const [x, y, w, h] = c.rect, part = surface(w, h), g = part.getContext('2d'); g.translate(-x, -y); trace(g, c.polygon); g.clip(); g.drawImage(source, 0, 0); return part; }
// Match each clean-plate patch to the surrounding original water. Feather a
// broad rectangle rather than the silhouette, so no stationary jelly outline remains.
function restoreWater(original, clean) {
    const out = surface(), g = out.getContext('2d');
    g.drawImage(original, 0, 0, ART_W, ART_H);
    const plate = surface(), p = plate.getContext('2d');
    p.drawImage(clean, 0, 0, ART_W, ART_H);
    const src = g.getImageData(0, 0, ART_W, ART_H), dst = p.getImageData(0, 0, ART_W, ART_H);
    for (const cfg of configs) {
        const [rx, ry, rw, rh] = cfg.rect, pad = 32, feather = 28, x0 = Math.max(0, rx - pad), y0 = Math.max(0, ry - pad), x1 = Math.min(ART_W, rx + rw + pad), y1 = Math.min(ART_H, ry + rh + pad), delta = [0, 0, 0];
        let samples = 0;
        for (let y = y0; y < y1; y += 3)
            for (let x = x0; x < x1; x += 3) {
                if (x > rx - 8 && x < rx + rw + 8 && y > ry - 8 && y < ry + rh + 8)
                    continue;
                const k = (y * ART_W + x) * 4;
                for (let c = 0; c < 3; c++)
                    delta[c] += src.data[k + c] - dst.data[k + c];
                samples++;
            }
        for (let c = 0; c < 3; c++)
            delta[c] /= samples || 1;
        for (let y = y0; y < y1; y++)
            for (let x = x0; x < x1; x++) {
                const edge = Math.min(x - x0, x1 - 1 - x, y - y0, y1 - 1 - y), t = Math.min(1, Math.max(0, edge / feather)), a = t * t * (3 - 2 * t), k = (y * ART_W + x) * 4;
                for (let c = 0; c < 3; c++)
                    src.data[k + c] = src.data[k + c] * (1 - a) + Math.max(0, Math.min(255, dst.data[k + c] + delta[c])) * a;
            }
    }
    g.putImageData(src, 0, 0);
    return out;
}
function restoreActors(still, clean, fixed) {
    const out = surface(), g = out.getContext('2d');
    g.drawImage(still, 0, 0);
    const plate = surface(), c = plate.getContext('2d');
    c.drawImage(clean, 0, 0, ART_W, ART_H);
    const src = g.getImageData(0, 0, ART_W, ART_H), dst = c.getImageData(0, 0, ART_W, ART_H);
    const raw = fixed.getContext('2d').getImageData(0, 0, ART_W, ART_H), solid = surface(), sc = solid.getContext('2d');
    for (let k = 0; k < raw.data.length; k += 4) {
        raw.data[k] = raw.data[k + 1] = raw.data[k + 2] = 255;
        raw.data[k + 3] = raw.data[k + 3] > 4 ? 255 : 0;
    }
    sc.putImageData(raw, 0, 0);
    const expanded = surface(), e = expanded.getContext('2d');
    for (let dy = -18; dy <= 18; dy += 6)
        for (let dx = -18; dx <= 18; dx += 6)
            if (dx * dx + dy * dy <= 400)
                e.drawImage(solid, dx, dy);
    const mask = surface(), m = mask.getContext('2d');
    m.filter = 'blur(5px)';
    m.drawImage(expanded, 0, 0);
    m.filter = 'none';
    m.drawImage(solid, 0, 0);
    const alpha = m.getImageData(0, 0, ART_W, ART_H).data;
    // Match nearby colors without treating the original characters as water samples.
    const size = 80, cols = Math.ceil(ART_W / size), rows = Math.ceil(ART_H / size), tiles = [];
    for (let ty = 0; ty < rows; ty++)
        for (let tx = 0; tx < cols; tx++) {
            const sums = [0, 0, 0];
            let n = 0;
            for (let y = ty * size; y < Math.min(ART_H, (ty + 1) * size); y += 4)
                for (let x = tx * size; x < Math.min(ART_W, (tx + 1) * size); x += 4) {
                    const k = (y * ART_W + x) * 4;
                    if (alpha[k + 3] > 1)
                        continue;
                    for (let z = 0; z < 3; z++)
                        sums[z] += src.data[k + z] - dst.data[k + z];
                    n++;
                }
            tiles.push(n > 10 ? sums.map(v => v / n) : null);
        }
    for (let index = 0; index < tiles.length; index++)
        if (!tiles[index]) {
            const x = index % cols, y = Math.floor(index / cols);
            let best = Infinity, value = [0, 0, 0];
            for (let k = 0; k < tiles.length; k++)
                if (tiles[k]) {
                    const distance = (k % cols - x) ** 2 + (Math.floor(k / cols) - y) ** 2;
                    if (distance < best) {
                        best = distance;
                        value = tiles[k];
                    }
                }
            tiles[index] = value;
        }
    for (let y = 260; y < 880; y++)
        for (let x = 790; x < 1608; x++) {
            const k = (y * ART_W + x) * 4, a = alpha[k + 3] / 255;
            if (!a)
                continue;
            const u = Math.max(0, Math.min(cols - 1.001, x / size - .5)), v = Math.max(0, Math.min(rows - 1.001, y / size - .5)), ix = Math.floor(u), iy = Math.floor(v), fx = u - ix, fy = v - iy;
            for (let z = 0; z < 3; z++) {
                const top = tiles[iy * cols + ix][z] * (1 - fx) + tiles[iy * cols + ix + 1][z] * fx, bottom = tiles[(iy + 1) * cols + ix][z] * (1 - fx) + tiles[(iy + 1) * cols + ix + 1][z] * fx;
                const target = Math.max(0, Math.min(255, dst.data[k + z] + top * (1 - fy) + bottom * fy));
                src.data[k + z] = src.data[k + z] * (1 - a) + target * a;
            }
        }
    g.putImageData(src, 0, 0);
    return out;
}
/** Owns one canvas, its animation clock and GPU resources. No global DOM hooks. */
export function mountJellyfishing(canvas, { onReady, onFallback }) {
    const gl = canvas.getContext('webgl', { alpha: false, antialias: true });
    if (!gl) {
        onFallback();
        return { dispose() { } };
    }
    const resources = { shaders: [], programs: [], textures: [], buffers: [] };
    const pendingLoads = new Set();
    let disposed = false, ready = false, suspended = false;
    let width = 1, height = 1, world, environment, actorField;
    let background, jellies, actorMeshes = [], bubbleMeshes = [];
    let raf = 0, last = 0, accumulator = 0;
    const seed = crypto.getRandomValues(new Uint32Array(1))[0];
    const observer = new ResizeObserver(resize);
    function dispose() {
        if (disposed)
            return;
        disposed = true;
        cancelAnimationFrame(raf);
        observer.disconnect();
        document.removeEventListener('visibilitychange', sync);
        window.removeEventListener('pagehide', pageHide);
        window.removeEventListener('pageshow', pageShow);
        canvas.removeEventListener('webglcontextlost', contextLost);
        for (const cancel of [...pendingLoads])
            cancel();
        for (const item of resources.buffers)
            gl.deleteBuffer(item);
        for (const item of resources.textures)
            gl.deleteTexture(item);
        for (const item of resources.programs)
            gl.deleteProgram(item);
        for (const item of resources.shaders)
            gl.deleteShader(item);
        gl.getExtension('WEBGL_lose_context')?.loseContext();
    }
    function fallback() { if (!disposed) {
        onFallback();
        dispose();
    } }
    function contextLost(event) { event.preventDefault(); fallback(); }
    function pageHide() { suspended = true; sync(); }
    function pageShow() { suspended = false; sync(); }
    function load(src) {
        return loadSceneArtwork(src, pendingLoads);
    }
    function resize() {
        if (disposed)
            return;
        const rect = canvas.getBoundingClientRect();
        width = Math.max(1, rect.width);
        height = Math.max(1, rect.height);
        const dpr = Math.min(devicePixelRatio || 1, 1.5, Math.sqrt(2400000 / (width * height)));
        canvas.width = Math.round(width * dpr);
        canvas.height = Math.round(height * dpr);
        if (world)
            world.resize(width, height);
        if (!document.hidden && !suspended)
            render();
    }
    function tick(now) {
        if (disposed || document.hidden || suspended)
            return;
        if (last) {
            accumulator += Math.min((now - last) / 1000, .1);
            const { scale, ox, oy } = actorCamera(width, height);
            while (accumulator >= 1 / 120) {
                world.step(1 / 120);
                environment.step(1 / 120);
                actorField.step(1 / 120, world.items.map(j => ({ x: (j.x - ox) / scale, y: (j.y - oy) / scale })));
                accumulator -= 1 / 120;
            }
        }
        last = now;
        render();
        raf = requestAnimationFrame(tick);
    }
    function sync() {
        cancelAnimationFrame(raf);
        last = 0;
        accumulator = 0;
        if (ready && !disposed && !document.hidden && !suspended)
            raf = requestAnimationFrame(tick);
    }
    // The shader setup and image preparation can fail independently. Both leave
    // the existing CSS wallpaper visible and release everything acquired so far.
    let render = () => { };
    try {
        const plantExpressions = MarineMotion.plants.map(p => 'plant(p,' + [p.x, p.top, p.root, p.halfWidth, p.amplitude, p.phase].map(n => n.toFixed(4)).join(',') + ')').join('+');
        const vertex = `precision highp float;attribute vec2 aPos;attribute vec2 aUV;varying vec2 vUV;
uniform vec2 uWorld,uOrigin,uPivot,uVelocity;uniform float uScale,uAngle,uCycle,uPeriod,uMode,uUnit,uEnvironmentTime,uEnvironmentAmount;
float curve(float p){p=clamp(p,0.,1.);return p*p*(3.-2.*p);}
float contraction(float p){p=fract(p);if(p<.23)return curve(p/.23);if(p<.80)return 1.-curve((p-.23)/.57);return 0.;}
float plant(vec2 p,float center,float top,float root,float halfWidth,float amplitude,float phase){
 float h=clamp((root-p.y)/(root-top),0.,1.),edge=1.-curve((abs(p.x-center)/halfWidth-.65)/.35),tip=curve((p.y-(top-26.))/24.);
 float wave=sin(uEnvironmentTime*.67+phase-h*.75)+.24*sin(uEnvironmentTime*1.07+phase*1.8-h*1.4);
 return amplitude*pow(h,1.65)*edge*tip*wave*uEnvironmentAmount;
}
void main(){vec2 p=aPos-uPivot;
 if(uMode>1.5){float dx=${plantExpressions};p.x+=dx;}
 else if(uMode>.5){float theta=-.46,c=cos(theta),s=sin(theta);vec2 q=vec2(c*p.x+s*p.y,-s*p.x+c*p.y)/uUnit;
  float pulse=contraction(uCycle),rim=curve((q.y+10.)/45.),width=1.035-.34*pulse;
  float rootY=-65.+100.*(.98+.14*pulse)-5.*pulse;
  if(q.y<=35.){q.x*=width*(1.-.11*pulse*rim);q.y=-65.+(q.y+65.)*(.98+.14*pulse)-5.*pulse*rim;}
  else{float depth=q.y-35.,f=clamp(depth/115.,0.,1.),delayed=contraction(uCycle-.16*f/uPeriod);q.x*=mix(width*(1.-.11*pulse),1.-.13*delayed,curve(depth/85.));q.x+=sin(uCycle*6.283185-depth*.035)*9.*f*f;q.y=rootY+depth*(1.+.14*delayed);}
  p=vec2(c*q.x-s*q.y,s*q.x+c*q.y)*uUnit;
 }
 float ca=cos(uAngle),sa=sin(uAngle);p=vec2(ca*p.x-sa*p.y,sa*p.x+ca*p.y)*uScale+uOrigin;
 if(uMode>.5&&uMode<1.5){float f=curve((aPos.y-uPivot.y-25.*uUnit)/(120.*uUnit));p-=uVelocity*.07*f*f;}
 vec2 clip=p/uWorld*2.-1.;gl_Position=vec4(clip.x,-clip.y,0.,1.);vUV=aUV;}`;
        const fragment = `precision mediump float;varying vec2 vUV;uniform sampler2D uTexture;uniform float uOpacity;void main(){gl_FragColor=texture2D(uTexture,vUV)*uOpacity;}`;
        function shader(type, code) { const s = gl.createShader(type); resources.shaders.push(s); gl.shaderSource(s, code); gl.compileShader(s); if (!gl.getShaderParameter(s, gl.COMPILE_STATUS))
            throw Error(gl.getShaderInfoLog(s)); return s; }
        const program = gl.createProgram();
        resources.programs.push(program);
        gl.attachShader(program, shader(gl.VERTEX_SHADER, vertex));
        gl.attachShader(program, shader(gl.FRAGMENT_SHADER, fragment));
        gl.linkProgram(program);
        if (!gl.getProgramParameter(program, gl.LINK_STATUS))
            throw Error(gl.getProgramInfoLog(program));
        gl.useProgram(program);
        const un = Object.fromEntries(['World', 'Origin', 'Pivot', 'Velocity', 'Scale', 'Angle', 'Cycle', 'Period', 'Mode', 'Unit', 'Texture', 'Opacity', 'EnvironmentTime', 'EnvironmentAmount'].map(n => [n, gl.getUniformLocation(program, 'u' + n)])), pos = gl.getAttribLocation(program, 'aPos'), uv = gl.getAttribLocation(program, 'aUV');
        gl.enableVertexAttribArray(pos);
        gl.enableVertexAttribArray(uv);
        gl.enable(gl.BLEND);
        gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA);
        gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, true);
        function mesh(img, w, h, grid = 1, pivot = [0, 0], unit = 1) {
            const texture = gl.createTexture();
            resources.textures.push(texture);
            gl.bindTexture(gl.TEXTURE_2D, texture);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
            gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, img);
            const vertices = [], indices = [];
            for (let y = 0; y <= grid; y++)
                for (let x = 0; x <= grid; x++)
                    vertices.push(x / grid * w, y / grid * h, x / grid, y / grid);
            for (let y = 0; y < grid; y++)
                for (let x = 0; x < grid; x++) {
                    const k = y * (grid + 1) + x;
                    indices.push(k, k + 1, k + grid + 1, k + 1, k + grid + 2, k + grid + 1);
                }
            const vb = gl.createBuffer();
            resources.buffers.push(vb);
            gl.bindBuffer(gl.ARRAY_BUFFER, vb);
            const base = new Float32Array(vertices), data = new Float32Array(vertices);
            gl.bufferData(gl.ARRAY_BUFFER, data, gl.DYNAMIC_DRAW);
            const ib = gl.createBuffer();
            resources.buffers.push(ib);
            gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, ib);
            gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, new Uint16Array(indices), gl.STATIC_DRAW);
            return { texture, vb, ib, count: indices.length, w, h, pivot, unit, base, data };
        }
        function draw(m, { origin = [0, 0], scale = 1, angle = 0, phase = 0, period = 2, deform = false, velocity = [0, 0], opacity = 1, ambient = false, actor = null } = {}) {
            gl.bindBuffer(gl.ARRAY_BUFFER, m.vb);
            if (actor) {
                const pose = actorField.pose(actor);
                for (let i = 0; i < m.base.length; i += 4) {
                    const q = actorField.warp(actor, [m.base[i] + actor.rect[0], m.base[i + 1] + actor.rect[1]], pose);
                    m.data[i] = q[0] - actor.rect[0];
                    m.data[i + 1] = q[1] - actor.rect[1];
                }
                gl.bufferSubData(gl.ARRAY_BUFFER, 0, m.data);
            }
            gl.vertexAttribPointer(pos, 2, gl.FLOAT, false, 16, 0);
            gl.vertexAttribPointer(uv, 2, gl.FLOAT, false, 16, 8);
            gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, m.ib);
            gl.bindTexture(gl.TEXTURE_2D, m.texture);
            gl.uniform1i(un.Texture, 0);
            gl.uniform2f(un.World, width, height);
            gl.uniform2f(un.Origin, ...origin);
            gl.uniform2f(un.Pivot, ...m.pivot);
            gl.uniform2f(un.Velocity, ...velocity);
            gl.uniform1f(un.Scale, scale);
            gl.uniform1f(un.Angle, angle);
            gl.uniform1f(un.Cycle, phase);
            gl.uniform1f(un.Period, period);
            gl.uniform1f(un.Mode, ambient ? 2 : deform ? 1 : 0);
            gl.uniform1f(un.Unit, m.unit);
            gl.uniform1f(un.Opacity, opacity);
            gl.uniform1f(un.EnvironmentTime, environment?.time || 0);
            gl.uniform1f(un.EnvironmentAmount, environment?.amount || 0);
            gl.drawElements(gl.TRIANGLES, m.count, gl.UNSIGNED_SHORT, 0);
        }
        render = function () {
            if (!ready || disposed)
                return;
            gl.viewport(0, 0, canvas.width, canvas.height);
            gl.clearColor(.4, .8, .9, 1);
            gl.clear(gl.COLOR_BUFFER_BIT);
            const scale = Math.max(width / ART_W, height / ART_H), align = width < 768 ? .75 : .5, origin = [(width - ART_W * scale) * align, (height - ART_H * scale) / 2];
            draw(background, { origin, scale, ambient: true });
            // The original bubbles crossfade to the live field when the comparison switch changes.
            for (let i = 0; i < bubbleMeshes.length; i++) {
                const m = bubbleMeshes[i], b = MarineMotion.marks[i];
                if (environment.amount < 1)
                    draw(m, { origin: [origin[0] + b.x * scale, origin[1] + b.y * scale], scale, opacity: 1 - environment.amount });
            }
            for (const b of environment.items) {
                const pose = environment.bubblePose(b), m = bubbleMeshes[b.variant];
                draw(m, { origin: [origin[0] + pose.x * scale, origin[1] + pose.y * scale], scale: scale * pose.r / MarineMotion.marks[b.variant].r, opacity: pose.opacity * environment.amount });
            }
            for (const j of [...world.items].sort((a, b) => a.depth - b.depth)) {
                const m = jellies[j.variant];
                draw(m, { origin: [j.x, j.y], scale: j.size / m.w, angle: j.angle + Math.PI / 2 + .46, phase: j.phase, period: j.period, deform: true, velocity: [j.vx, j.vy], opacity: .6 + .35 * Math.min(1, j.depth) });
            }
            // Root coordinates stay fixed. Only small upper-body and net gestures change.
            const camera = actorCamera(width, height);
            actorMeshes.forEach((m, i) => { const a = actorField.items[i]; draw(m, { origin: [camera.ox + a.rect[0] * camera.scale, camera.oy + a.rect[1] * camera.scale], scale: camera.scale, actor: a }); });
        };
        canvas.addEventListener('webglcontextlost', contextLost);
        document.addEventListener('visibilitychange', sync);
        window.addEventListener('pagehide', pageHide);
        window.addEventListener('pageshow', pageShow);
        observer.observe(canvas);
        Promise.all([
            load('/media/storefront-20261020/clean-plate.webp'),
            load('/media/storefront-20261020/foreground.webp'),
            load('/media/storefront-20261020/jellyfishing.webp'),
        ]).then(([clean, fg, original]) => {
            if (disposed)
                return;
            const source = surface(), g = source.getContext('2d');
            g.drawImage(fg, 0, 0, ART_W, ART_H);
            g.globalCompositeOperation = 'source-in';
            g.drawImage(original, 0, 0, ART_W, ART_H);
            const cuts = configs.map(c => crop(source, c));
            const still = restoreWater(original, clean);
            const fixedActors = surface(), a = fixedActors.getContext('2d');
            a.drawImage(source, 0, 0);
            a.globalCompositeOperation = 'destination-out';
            for (const c of configs) {
                trace(a, c.polygon);
                a.fill();
            }
            const ambient = MarineMotion.prepareBubbles(restoreActors(still, clean, fixedActors));
            actorField = new StationaryActors(seed);
            environment = new MarineMotion.MarineEnvironment(seed);
            actorMeshes = actorField.items.map(a => { const [x, y, w, h] = a.rect, img = surface(w, h); img.getContext('2d').drawImage(fixedActors, -x, -y); return mesh(img, w, h, 64); });
            background = mesh(ambient.water, ART_W, ART_H, 160);
            bubbleMeshes = ambient.sprites.map(s => mesh(s.image, s.size, s.size, 1, s.pivot));
            jellies = cuts.slice(0, 3).map((part, i) => { const c = configs[i]; return mesh(part, c.rect[2], c.rect[3], 32, c.pivot, c.unit); });
            resize();
            world = new JellyWorld({ width, height, count: width < 600 ? 6 : 9, seed });
            ready = true;
            render();
            onReady();
            sync();
        }).catch(fallback);
    }
    catch {
        fallback();
    }
    return { dispose };
}
