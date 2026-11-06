import type { PetPose } from './petMotion'

export type PetRenderer = { draw(pose: PetPose): void; dispose(): void }

// The original illustration stays connected at the shoulder. Depth makes
// the moving forearm cover the torso; transparent texels must not write depth.
export function createPetRenderer(canvas: HTMLCanvasElement, image: HTMLImageElement): PetRenderer | null {
  const gl = canvas.getContext('webgl', { alpha: true, antialias: true, depth: true, stencil: false, preserveDrawingBuffer: true })
  if (!gl) return null
  const shaders: WebGLShader[] = []
  const buffers: WebGLBuffer[] = []
  const textures: WebGLTexture[] = []
  const program = gl.createProgram()
  let disposed = false
  const dispose = () => {
    if (disposed) return
    disposed = true
    buffers.forEach(buffer => gl.deleteBuffer(buffer)); textures.forEach(texture => gl.deleteTexture(texture))
    shaders.forEach(shader => gl.deleteShader(shader)); gl.deleteProgram(program)
  }
  try {
    if (!program) throw new Error('WebGL program unavailable')
    const shader = (type: number, source: string) => {
      const value = gl.createShader(type)
      if (!value) throw new Error('WebGL shader unavailable')
      shaders.push(value); gl.shaderSource(value, source); gl.compileShader(value)
      if (!gl.getShaderParameter(value, gl.COMPILE_STATUS)) throw new Error('Pet shader compilation failed')
      return value
    }
    gl.attachShader(program, shader(gl.VERTEX_SHADER, `
      attribute vec2 a_point;
      uniform vec2 u_arm;
      uniform mediump vec4 u_detail;
      uniform vec4 u_turn;
      varying vec2 v_uv;
      void main(){
        vec2 p=a_point;
        float shoulder=326.0+0.28*(p.y-308.0);
        float weight=smoothstep(0.0,55.0,p.x-shoulder)
          *smoothstep(0.0,20.0,(p.y-275.0)+0.7*(p.x-340.0))
          *(1.0-smoothstep(372.0,402.0,p.y));
        vec2 pivot=vec2(343.0,343.0);
        vec2 local=(p-pivot)*u_arm.y;
        float c=cos(u_arm.x),s=sin(u_arm.x);
        vec2 moved=pivot+vec2(c*local.x-s*local.y,s*local.x+c*local.y);
        vec2 position=mix(p,moved,weight);
        // Small, local deformations animate the drawing without rebuilding it.
        vec2 belly=(p-vec2(258.0,385.0))/vec2(108.0,57.0);
        float bellyWeight=exp(-dot(belly,belly)*2.0);
        position += vec2(belly.x*6.0, belly.y*3.0)*u_detail.x*bellyWeight;
        float headWeight=1.0-smoothstep(260.0,365.0,p.y);
        position += vec2(u_detail.y*3.0,u_detail.z*2.0)*headWeight;
        float footWeight=smoothstep(495.0,548.0,p.y);
        position.y -= footWeight * mix(u_turn.z,u_turn.w,smoothstep(225.0,265.0,p.x))*5.0;
        // A shallow rounded relief gives gentle parallax, never a side/back view.
        vec2 rounded=(p-vec2(254.0,345.0))/vec2(180.0,310.0);
        float relief=13.0*sqrt(max(0.0,1.0-dot(rounded,rounded)));
        position.x=254.0+(position.x-254.0)*cos(u_turn.x)+relief*sin(u_turn.x);
        position.y+=relief*sin(u_turn.y);
        gl_Position=vec4(position.x/240.0-1.0,1.0-position.y/300.0,-0.5*weight,1.0);
        v_uv=p/vec2(480.0,600.0);
      }`))
    gl.attachShader(program, shader(gl.FRAGMENT_SHADER, `
      precision mediump float;
      uniform sampler2D u_image;
      uniform mediump vec4 u_detail;
      varying vec2 v_uv;
      void main(){
        vec4 color=texture2D(u_image,v_uv);
        if(color.a<0.01) discard;
        // Blink only the painted lens glints; all ink and colours stay original.
        vec2 left=(v_uv-vec2(.448,.294))/vec2(.033,.041);
        vec2 right=(v_uv-vec2(.597,.318))/vec2(.033,.041);
        float lenses=1.0-smoothstep(.72,1.0,min(dot(left,left),dot(right,right)));
        float highlight=smoothstep(.65,.90,min(color.r,min(color.g,color.b)));
        color.rgb=mix(color.rgb,vec3(.035,.039,.055),lenses*highlight*u_detail.w);
        gl_FragColor=color;
      }`))
    gl.linkProgram(program)
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) throw new Error('Pet shader linking failed')
    gl.useProgram(program)
    const points: number[] = [], triangles: number[] = []
    for (let y = 0; y <= 100; y++) for (let x = 0; x <= 80; x++) points.push(x * 6, y * 6)
    for (let y = 0; y < 100; y++) for (let x = 0; x < 80; x++) { const a = y * 81 + x, b = a + 1, c = a + 81, d = c + 1; triangles.push(a, c, b, b, c, d) }
    const vertices = gl.createBuffer(), indices = gl.createBuffer(), texture = gl.createTexture()
    if (vertices) buffers.push(vertices)
    if (indices) buffers.push(indices)
    if (texture) textures.push(texture)
    if (!vertices || !indices || !texture) throw new Error('Pet graphics allocation failed')
    gl.bindBuffer(gl.ARRAY_BUFFER, vertices); gl.bufferData(gl.ARRAY_BUFFER, new Float32Array(points), gl.STATIC_DRAW)
    const location = gl.getAttribLocation(program, 'a_point'); gl.enableVertexAttribArray(location); gl.vertexAttribPointer(location, 2, gl.FLOAT, false, 0, 0)
    gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, indices); gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, new Uint16Array(triangles), gl.STATIC_DRAW)
    gl.bindTexture(gl.TEXTURE_2D, texture)
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR); gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE); gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, image)
    gl.uniform1i(gl.getUniformLocation(program, 'u_image'), 0)
    const arm = gl.getUniformLocation(program, 'u_arm'), detail = gl.getUniformLocation(program, 'u_detail'), turn = gl.getUniformLocation(program, 'u_turn')
    gl.viewport(0, 0, 480, 600); gl.clearColor(0, 0, 0, 0); gl.clearDepth(1)
    gl.enable(gl.DEPTH_TEST); gl.depthFunc(gl.LEQUAL); gl.enable(gl.BLEND)
    gl.blendFuncSeparate(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA, gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
    return {
      draw(pose: PetPose) {
        if (disposed) return
        gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
        gl.uniform2f(arm, pose.arm * Math.PI / 180, pose.armScale)
        gl.uniform4f(detail, pose.belly, pose.lookX, pose.lookY, pose.blink)
        gl.uniform4f(turn, Math.max(-10, Math.min(10, pose.yaw)) * Math.PI / 180,
          Math.max(-5, Math.min(5, pose.pitch)) * Math.PI / 180, pose.leftFoot, pose.rightFoot)
        gl.drawElements(gl.TRIANGLES, triangles.length, gl.UNSIGNED_SHORT, 0)
      },
      dispose,
    }
  } catch {
    dispose()
    return null
  }
}
