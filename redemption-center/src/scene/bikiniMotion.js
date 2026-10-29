const ART_WIDTH = 1672;
const ART_HEIGHT = 941;
const ART_URL = '/assets/bikini-bottom-afternoon.png';

// All motion is applied to the approved illustration, in its original pixel coordinates.
const sceneVertex = `
precision highp float;
attribute vec2 aPoint;
uniform vec2 uView, uOffset;
uniform float uTime;
varying vec2 vUv;
varying float vLight;
float oval(vec2 p,vec2 c,vec2 r){return 1.-smoothstep(.58,1.,length((p-c)/r));}
float plant(vec2 p,float x,float top,float root,float width,float phase){
  float h=clamp((root-p.y)/(root-top),0.,1.);
  float edge=1.-smoothstep(width*.42,width,abs(p.x-x));
  float cap=smoothstep(top-22.,top+15.,p.y);
  return 3.6*h*h*edge*cap*(sin(uTime*.72+phase-h)+.22*sin(uTime*1.17+phase));
}
void main(){
  vec2 p=aPoint*uView+uOffset;
  vec2 drift=vec2(0.);
  // Feet stay planted; upper bodies breathe and lean on different rhythms.
  float sponge=oval(p,vec2(125.,653.),vec2(128.,150.))*(1.-smoothstep(726.,789.,p.y));
  drift.x+=sponge*(2.7*sin(uTime*.74)+.006*(p.x-126.)*sin(uTime*1.43));
  drift.y+=sponge*(p.y-788.)*.012*sin(uTime*1.43);
  float wave=oval(p,vec2(245.,598.),vec2(47.,46.));
  float gesture=pow(.5+.5*sin(uTime*.39),3.);
  drift+=wave*vec2((p.y-646.)*.085,(205.-p.x)*.07)*sin(uTime*2.3)*gesture;

  float patrick=oval(p,vec2(1496.,664.),vec2(135.,179.))*(1.-smoothstep(765.,818.,p.y));
  drift.x+=patrick*(3.3*sin(uTime*.63+1.2)+.007*(p.x-1496.)*sin(uTime*1.2+2.));
  drift.y+=patrick*(p.y-815.)*.011*sin(uTime*1.2+2.);
  float arm=oval(p,vec2(1417.,626.),vec2(55.,61.));
  drift+=arm*vec2(1.8,2.9)*sin(uTime*1.47+2.4);

  float squid=oval(p,vec2(1625.,624.),vec2(43.,110.))*(1.-smoothstep(698.,758.,p.y));
  drift+=squid*vec2(1.8*sin(uTime*.54+2.4),1.3*sin(uTime*.97));
  float gary=oval(p,vec2(294.,717.),vec2(43.,42.));
  drift+=gary*vec2(1.6*sin(uTime*.91+1.),1.2*sin(uTime*1.4));

  drift.x+=plant(p,298.,467.,696.,31.,.8)+plant(p,1465.,465.,651.,30.,2.3)
          +plant(p,1648.,780.,941.,28.,1.2)+plant(p,18.,779.,941.,24.,3.1);
  float water=smoothstep(290.,440.,p.x)*(1.-smoothstep(1250.,1410.,p.x))*(1.-smoothstep(570.,710.,p.y));
  drift+=water*vec2(.9*sin(p.y*.019+uTime*.44),.6*sin(p.x*.014+uTime*.31));
  vUv=clamp((p+drift)/vec2(1672.,941.),vec2(.0002),vec2(.9998));
  float surface=1.-smoothstep(50.,620.,p.y);
  float ray=sin(p.x*.009+p.y*.003+uTime*.19)*sin(p.x*.016-p.y*.002-uTime*.27);
  vLight=.018*ray*surface+.006*sin(p.x*.015+p.y*.018-uTime*.38)*water;
  gl_Position=vec4(aPoint.x*2.-1.,1.-aPoint.y*2.,0.,1.);
}`;

const sceneFragment = `
precision mediump float;
uniform sampler2D uArtwork;
varying vec2 vUv;
varying float vLight;
void main(){vec4 c=texture2D(uArtwork,vUv);gl_FragColor=vec4(clamp(c.rgb+vLight,0.,1.),1.);}`;

const bubbleVertex = `
precision highp float;
attribute vec4 aBubble;
varying float vAlpha;
void main(){gl_Position=vec4(aBubble.xy,0.,1.);gl_PointSize=aBubble.z;vAlpha=aBubble.w;}`;
const bubbleFragment = `
precision mediump float;
varying float vAlpha;
void main(){
  vec2 p=gl_PointCoord*2.-1.;float r=length(p);
  float rim=smoothstep(.74,.84,r)*(1.-smoothstep(.88,.99,r));
  float shine=exp(-length(p-vec2(-.37,-.42))*13.);
  float alpha=(rim*.53+shine*.72+(1.-smoothstep(.0,.85,r))*.025)*vAlpha;
  gl_FragColor=vec4(.92,1.,1.,alpha);
}`;

export function mountBikiniMotion(canvas, onStatus) {
  const gl = canvas.getContext('webgl', { alpha: false, antialias: false, powerPreference: 'low-power' });
  if (!gl) { onStatus('unavailable'); return () => {}; }
  const resources = { shaders: [], programs: [], buffers: [], textures: [] };
  const picture = new Image();
  let disposed = false, ready = false, suspended = false;
  let raf = 0, previous = 0, lastDraw = 0, time = 0, announced = -1;
  let width = 1, height = 1, ratio = 1, indexCount = 0;
  let scene, bubbleProgram, gridBuffer, indexBuffer, bubbleBuffer, texture;
  let scenePoint, bubblePoint, timeUniform, viewUniform, offsetUniform;
  const bubbles = Array.from({ length: 22 }, (_, i) => ({
    x: ((i * .618034 + .11) % 1),
    phase: ((i * .381966 + .17) % 1),
    speed: 9 + (i * 7 % 14),
    size: 9 + (i * 11 % 18),
    sway: 7 + (i * 3 % 13),
  }));
  const bubbleData = new Float32Array(bubbles.length * 4);
  const observer = new ResizeObserver(resize);

  function shader(kind, source) {
    const handle = gl.createShader(kind);
    resources.shaders.push(handle);
    gl.shaderSource(handle, source); gl.compileShader(handle);
    if (!gl.getShaderParameter(handle, gl.COMPILE_STATUS)) throw Error(gl.getShaderInfoLog(handle));
    return handle;
  }
  function program(vertex, fragment) {
    const handle = gl.createProgram(); resources.programs.push(handle);
    gl.attachShader(handle, shader(gl.VERTEX_SHADER, vertex));
    gl.attachShader(handle, shader(gl.FRAGMENT_SHADER, fragment));
    gl.linkProgram(handle);
    if (!gl.getProgramParameter(handle, gl.LINK_STATUS)) throw Error(gl.getProgramInfoLog(handle));
    return handle;
  }
  function buffer(target, data, mode = gl.STATIC_DRAW) {
    const handle = gl.createBuffer(); resources.buffers.push(handle);
    gl.bindBuffer(target, handle); gl.bufferData(target, data, mode); return handle;
  }
  function dispose() {
    if (disposed) return;
    disposed = true;
    cancelAnimationFrame(raf); observer.disconnect();
    picture.onload = null; picture.onerror = null;
    document.removeEventListener('visibilitychange', sync);
    window.removeEventListener('pagehide', pageHide);
    window.removeEventListener('pageshow', pageShow);
    canvas.removeEventListener('webglcontextlost', contextLost);
    resources.buffers.forEach(b => gl.deleteBuffer(b));
    resources.textures.forEach(t => gl.deleteTexture(t));
    resources.programs.forEach(p => gl.deleteProgram(p));
    resources.shaders.forEach(s => gl.deleteShader(s));
    // React StrictMode may immediately mount on this canvas again; delete our
    // resources without permanently losing its reusable browser context.
  }
  function fallback() { if (!disposed) { canvas.dataset.ready = 'false'; onStatus('unavailable'); dispose(); } }
  function contextLost(event) { event.preventDefault(); fallback(); }
  function pageHide() { suspended = true; sync(); }
  function pageShow() { suspended = false; sync(); }
  function sync() {
    cancelAnimationFrame(raf); raf = 0; previous = 0; lastDraw = 0;
    if (!disposed && ready && !document.hidden && !suspended) raf = requestAnimationFrame(tick);
  }
  function resize() {
    if (disposed) return;
    const rect = canvas.getBoundingClientRect();
    width = Math.max(1, rect.width); height = Math.max(1, rect.height);
    ratio = Math.min(window.devicePixelRatio || 1, 2, Math.sqrt(3200000 / (width * height)));
    canvas.width = Math.round(width * ratio); canvas.height = Math.round(height * ratio);
    if (ready && !document.hidden && !suspended) render();
  }
  function tick(now) {
    if (disposed || document.hidden || suspended) return;
    if (previous) time += Math.min((now - previous) / 1000, .08);
    previous = now;
    if (now - lastDraw >= 1000 / 30) { render(); lastDraw = now; }
    raf = requestAnimationFrame(tick);
  }
  function render() {
    if (!ready || disposed) return;
    const scale = Math.max(width / ART_WIDTH, height / ART_HEIGHT);
    const vw = width / scale, vh = height / scale;
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.disable(gl.BLEND);
    gl.useProgram(scene);
    gl.bindBuffer(gl.ARRAY_BUFFER, gridBuffer);
    gl.enableVertexAttribArray(scenePoint); gl.vertexAttribPointer(scenePoint, 2, gl.FLOAT, false, 0, 0);
    gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, indexBuffer);
    gl.activeTexture(gl.TEXTURE0); gl.bindTexture(gl.TEXTURE_2D, texture);
    gl.uniform2f(viewUniform, vw, vh);
    // This matches the CSS background's desktop center and mobile bottom anchor.
    gl.uniform2f(offsetUniform, (ART_WIDTH - vw) / 2, (ART_HEIGHT - vh) * (width <= 720 ? 1 : .5));
    gl.uniform1f(timeUniform, time);
    gl.drawElements(gl.TRIANGLES, indexCount, gl.UNSIGNED_SHORT, 0);
    gl.disableVertexAttribArray(scenePoint);

    const count = width <= 720 ? 10 : bubbles.length;
    for (let i = 0; i < count; i++) {
      const b = bubbles[i], travel = height + 80;
      const y = height + 40 - ((time * b.speed + b.phase * travel) % travel);
      const x = b.x * width + Math.sin(time * .42 + i * 1.7) * b.sway;
      const fade = Math.min(1, Math.max(0, (height + 25 - y) / 65), Math.max(0, (y + 30) / 80));
      const center = x > width * .25 && x < width * .78;
      bubbleData.set([x / width * 2 - 1, 1 - y / height * 2, b.size * ratio, fade * (center ? .37 : .65)], i * 4);
    }
    gl.enable(gl.BLEND); gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
    gl.useProgram(bubbleProgram);
    gl.bindBuffer(gl.ARRAY_BUFFER, bubbleBuffer);
    gl.bufferSubData(gl.ARRAY_BUFFER, 0, bubbleData);
    gl.enableVertexAttribArray(bubblePoint); gl.vertexAttribPointer(bubblePoint, 4, gl.FLOAT, false, 0, 0);
    gl.drawArrays(gl.POINTS, 0, count);
    gl.disableVertexAttribArray(bubblePoint);
    if (Math.floor(time) !== announced) {
      announced = Math.floor(time); canvas.dataset.motionTime = time.toFixed(2);
    }
  }

  try {
    scene = program(sceneVertex, sceneFragment); bubbleProgram = program(bubbleVertex, bubbleFragment);
    scenePoint = gl.getAttribLocation(scene, 'aPoint'); bubblePoint = gl.getAttribLocation(bubbleProgram, 'aBubble');
    timeUniform = gl.getUniformLocation(scene, 'uTime');
    viewUniform = gl.getUniformLocation(scene, 'uView'); offsetUniform = gl.getUniformLocation(scene, 'uOffset');
    const columns = 96, rows = 54, points = [], indices = [];
    for (let y = 0; y <= rows; y++) for (let x = 0; x <= columns; x++) points.push(x / columns, y / rows);
    for (let y = 0; y < rows; y++) for (let x = 0; x < columns; x++) {
      const a = y * (columns + 1) + x, b = a + columns + 1;
      indices.push(a, b, a + 1, a + 1, b, b + 1);
    }
    indexCount = indices.length;
    gridBuffer = buffer(gl.ARRAY_BUFFER, new Float32Array(points));
    indexBuffer = buffer(gl.ELEMENT_ARRAY_BUFFER, new Uint16Array(indices));
    bubbleBuffer = buffer(gl.ARRAY_BUFFER, bubbleData, gl.DYNAMIC_DRAW);
    texture = gl.createTexture(); resources.textures.push(texture);
    picture.onload = () => {
      if (disposed) return;
      try {
        gl.bindTexture(gl.TEXTURE_2D, texture);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, picture);
        ready = true; resize();
        canvas.dataset.ready = 'true'; onStatus('playing'); sync();
      } catch { fallback(); }
    };
    picture.onerror = fallback;
    observer.observe(canvas);
    document.addEventListener('visibilitychange', sync);
    window.addEventListener('pagehide', pageHide); window.addEventListener('pageshow', pageShow);
    canvas.addEventListener('webglcontextlost', contextLost);
    picture.src = ART_URL;
  } catch { fallback(); }
  return dispose;
}
