import { BubbleWorld } from './bubbleWorld.js';
import spriteData from './bubbleSprites.json';
// Approved ocean wallpaper motion, 2026-10-23.
const scenes = {
 garden: {file:'coral-garden-20261022.png', title:'珊瑚花园 · 珊迪与派大星', actors:[[.065,.59,.075,.15],[.92,.71,.072,.15]], hands:[[.116,.58,.026,.045],[.965,.744,.035,.05]], note:'珊迪轻轻招手，派大星呼吸轻摆；两侧海草随水流摆动。'},
 street: {file:'krusty-street-20261022.png', title:'蟹堡街道 · 蟹老板与珍珍', actors:[[.069,.72,.071,.12],[.935,.66,.068,.16]], hands:[[.112,.694,.025,.05],[.975,.59,.025,.045]], note:'蟹老板轻抬蟹钳，珍珍缓缓摆动；店铺与地面保持稳定。'},
 home: {file:'pineapple-home-20261022.png', title:'菠萝屋 · 海绵宝宝与小蜗', actors:[[.096,.75,.085,.14],[.927,.79,.073,.09]], hands:[[.164,.71,.032,.04],[.893,.756,.033,.045]], note:'海绵宝宝轻摆手臂，小蜗眼柄微动；菠萝叶轻轻摇曳。'},
 detail: {file:'bikini-bottom-detail-20261022.png', title:'商品详情 · 蟹老板、章鱼哥与小蜗', actors:[[.096,.67,.091,.17],[.921,.61,.063,.19],[.913,.91,.077,.09]], hands:[[.145,.59,.04,.05],[.918,.48,.04,.04]], note:'人物保持原位呼吸，蟹钳与小蜗眼柄微动；背景固定在视口。'}
};
const vertex=`attribute vec2 pos; varying vec2 uv; void main(){ uv=vec2((pos.x+1.)*.5,(1.-pos.y)*.5); gl_Position=vec4(pos,0.,1.); }`;
const fragment=`precision highp float;
varying vec2 uv; uniform sampler2D art; uniform float t; uniform vec2 view; uniform vec2 artSize; uniform float alignment; uniform vec4 actors[3]; uniform vec4 hands[2]; uniform vec2 actorMotion[3]; uniform vec2 handMotion[2]; uniform float homeSway;
float area(vec2 p,vec4 r){return 1.-smoothstep(.48,1.,length((p-r.xy)/max(r.zw,vec2(.0001))));}
void main(){
 float scale=max(view.x/artSize.x,view.y/artSize.y); vec2 size=artSize*scale; vec2 offset=(view-size)*vec2(alignment,.5); vec2 p=(uv*view-offset)/size; vec2 d=vec2(0.);
 for(int i=0;i<3;i++){
   vec4 a=actors[i]; float weight=area(p,a); float height=clamp((a.y+a.w-p.y)/(a.w*2.),0.,1.);
   d.x+=weight*height*actorMotion[i].x;
   d.y+=weight*height*actorMotion[i].y;
 }
 for(int i=0;i<2;i++){float weight=area(p,hands[i]); d+=weight*handMotion[i];}
 float plant=area(p,vec4(.055,.90,.09,.14))+area(p,vec4(.962,.90,.07,.15));
 float root=1.-smoothstep(.84,1.,p.y); d.x+=plant*root*.0045*sin(t*.68-p.y*5.);
 d.x+=area(p,vec4(.05,.035,.08,.12))*homeSway;
 vec3 color=texture2D(art,clamp(p-d,vec2(.001),vec2(.999))).rgb;
 float water=(1.-smoothstep(.05,.65,p.y))*smoothstep(.12,.3,p.x)*(1.-smoothstep(.7,.88,p.x));
 float light=water*.013*(sin(p.x*30.+p.y*8.+t*.19)+sin(p.x*17.-p.y*12.-t*.15));
 gl_FragColor=vec4(color+light,1.);
}`;
/** Mount one approved scene. The CSS wallpaper remains visible until ready. */
export function mountOceanScene(canvas, bubbles, key, { onReady, onFallback }) {
  let gl, ctx, program, texture, buffer, observer, raf = 0;
  let disposed = false, ready = false, suspended = false, time = 0, last = null, paintAt = null;
  let width = 1, height = 1, world, imagesLoaded = 0;
  const shaders = [], uniforms = {}, image = new Image(), atlas = new Image(), config = scenes[key];
  const actors = config.actors.flat();
  while (actors.length < 12) actors.push(0);
  const actorMotion = new Float32Array(6), handMotion = new Float32Array(4);


  function dispose() {
    if (disposed) return;
    disposed = true;
    ready = false;
    cancelAnimationFrame(raf);
    observer?.disconnect();
    document.removeEventListener('visibilitychange', sync);
    window.removeEventListener('pagehide', hide);
    window.removeEventListener('pageshow', show);
    canvas.removeEventListener('webglcontextlost', contextLost);
    image.onload = image.onerror = null;
    image.src = '';
    atlas.onload = atlas.onerror = null;
    atlas.src = '';
    world = null;
    if (gl) {
      if (buffer) gl.deleteBuffer(buffer);
      if (texture) gl.deleteTexture(texture);
      if (program) gl.deleteProgram(program);
      shaders.forEach(s => gl.deleteShader(s));
      gl.getExtension('WEBGL_lose_context')?.loseContext();
    }
  }
  function fallback() {
    if (disposed) return;
    onFallback();
    dispose();
  }
  function contextLost(event) { event.preventDefault(); fallback(); }
  function hide() { suspended = true; sync(); }
  function show() { suspended = false; sync(); }
  function sync() {
    cancelAnimationFrame(raf);
    last = null; paintAt = null;
    if (ready && !disposed && !document.hidden && !suspended) raf = requestAnimationFrame(tick);
  }
  function compile(type, source) {
    const shader = gl.createShader(type);
    if (!shader) throw Error('Scene shader unavailable');
    shaders.push(shader);
    gl.shaderSource(shader, source);
    gl.compileShader(shader);
    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) throw Error('Scene shader compilation failed');
    return shader;
  }
  function resize() {
    if (disposed) return;
    const rect = canvas.getBoundingClientRect();
    width = Math.max(1, rect.width); height = Math.max(1, rect.height);
    const dpr = Math.min(devicePixelRatio || 1, 1.5, Math.sqrt(2400000 / (width * height)));
    const pixelWidth = Math.max(1, Math.round(width * dpr));
    const pixelHeight = Math.max(1, Math.round(height * dpr));
    if (canvas.width !== pixelWidth || canvas.height !== pixelHeight) {
      canvas.width = bubbles.width = pixelWidth;
      canvas.height = bubbles.height = pixelHeight;
    }
    world?.resize(width, height);
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.uniform2f(uniforms.view, width, height);
    gl.uniform2f(uniforms.artSize, image.naturalWidth, image.naturalHeight);
    // Keep the static fallback's existing mobile crop, including detail's 70%.
    gl.uniform1f(uniforms.alignment, width < 768 ? (key === 'garden' ? .1 : key === 'detail' ? .7 : .85) : .5);
    if (!document.hidden && !suspended) draw();
  }
  function draw() {
    if (!ready || disposed) return;
    gl.useProgram(program);
    gl.uniform1f(uniforms.t, time);
    // Time-only trigonometry is identical for every pixel: calculate once per paint.
    for (let i = 0; i < 3; i++) {
      const phase = i * 2.3;
      actorMotion[i * 2] = Math.sin(time * .72 + phase) * .0017 + Math.sin(time * .29 + phase) * .0004;
      actorMotion[i * 2 + 1] = Math.sin(time * 1.12 + phase) * .0013;
    }
    for (let i = 0; i < 2; i++) {
      const sway = Math.sin(time * .91 + i * 2.1), wave = sway * .5 + .5;
      handMotion[i * 2] = .0019 * wave * wave;
      handMotion[i * 2 + 1] = .0017 * sway;
    }
    gl.uniform2fv(uniforms['actorMotion[0]'], actorMotion);
    gl.uniform2fv(uniforms['handMotion[0]'], handMotion);
    gl.uniform1f(uniforms.homeSway, key === 'home' ? .0018 * Math.sin(time * .63) : 0);
    gl.drawArrays(gl.TRIANGLES, 0, 6);
    ctx.clearRect(0, 0, bubbles.width, bubbles.height);
    ctx.save(); ctx.scale(bubbles.width / width, bubbles.height / height);
    ctx.globalAlpha = .93;
    for (const b of world.items) {
      ctx.drawImage(atlas, b.sx, b.sy, b.cell, b.cell, b.x - b.r, b.y - b.r, b.r * 2, b.r * 2);
    }
    ctx.restore();
  }
  function tick(now) {
    if (disposed || document.hidden || suspended) return;
    if (last !== null) {
      const dt = Math.min((now - last) / 1000, .05);
      time += dt;
      world.step(dt);
    }
    last = now;
    // Cap GPU painting at 60 Hz on 120/144/240 Hz displays; simulation keeps elapsed time.
    // Normal 60 Hz devices retain every frame and the existing resolution/motion.
    const interval = 1000 / 60;
    if (paintAt === null || now >= paintAt - 0.8) {
      paintAt = paintAt === null ? now + interval
        : paintAt + (Math.floor(Math.max(0, now - paintAt) / interval) + 1) * interval;
      try { draw(); } catch { fallback(); return; }
    }
    raf = requestAnimationFrame(tick);
  }
  try {
    gl = canvas.getContext('webgl', { alpha: false, antialias: false });
    ctx = bubbles.getContext('2d');
    if (!gl || !ctx) throw Error('Scene canvas unavailable');
    program = gl.createProgram();
    if (!program) throw Error('Scene program unavailable');
    gl.attachShader(program, compile(gl.VERTEX_SHADER, vertex));
    gl.attachShader(program, compile(gl.FRAGMENT_SHADER, fragment));
    gl.linkProgram(program);
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) throw Error('Scene linking failed');
    gl.useProgram(program);
    buffer = gl.createBuffer();
    if (!buffer) throw Error('Scene buffer unavailable');
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1,-1,1,-1,-1,1,-1,1,1,-1,1,1]), gl.STATIC_DRAW);
    const pos = gl.getAttribLocation(program, 'pos');
    gl.enableVertexAttribArray(pos); gl.vertexAttribPointer(pos, 2, gl.FLOAT, false, 0, 0);
    for (const name of ['t','view','artSize','alignment','actors[0]','hands[0]','actorMotion[0]','handMotion[0]','homeSway']) uniforms[name] = gl.getUniformLocation(program, name);
    gl.uniform4fv(uniforms['actors[0]'], actors);
    gl.uniform4fv(uniforms['hands[0]'], config.hands.flat());
    texture = gl.createTexture();
    if (!texture) throw Error('Scene texture unavailable');
    gl.bindTexture(gl.TEXTURE_2D, texture);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    canvas.addEventListener('webglcontextlost', contextLost);
    document.addEventListener('visibilitychange', sync);
    window.addEventListener('pagehide', hide);
    window.addEventListener('pageshow', show);
    observer = new ResizeObserver(() => { try { resize(); } catch { fallback(); } });
    observer.observe(canvas);
    const loaded = () => {
      if (disposed || ++imagesLoaded !== 2) return;
        try {
        gl.bindTexture(gl.TEXTURE_2D, texture);
        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, image);
        resize();
        world = new BubbleWorld(spriteData[key].sprites, width, height, Math.floor(Math.random() * 4294967296));
        ready = true; resize(); onReady(); sync();
      } catch { fallback(); }
    };
    image.onload = atlas.onload = loaded;
    image.onerror = atlas.onerror = fallback;
    // Slow downloads may finish later; only actual image errors trigger fallback.
    image.decoding = atlas.decoding = 'async';
    image.fetchPriority = atlas.fetchPriority = 'low';
    image.src = `/media/ocean-20261023/${key}-clean.webp`;
    atlas.src = `/media/ocean-20261023/${key}-bubbles.webp`;
  } catch { fallback(); }
  return { dispose };
}
