<template>
  <span class="support-pet-art" :data-motion="phase" :data-renderer="rendererReady ? 'illustrated-depth' : 'original'">
    <span ref="shadowElement" class="support-pet-shadow" aria-hidden="true" />
    <span ref="poseElement" class="support-pet-pose">
      <img v-show="!rendererReady" :src="rig" alt="" class="support-pet-layer" />
      <canvas v-show="rendererReady" ref="canvas" width="480" height="600" class="support-pet-layer" aria-hidden="true" />
      <span v-for="side in ['left', 'right']" :key="side" class="support-pet-layer support-pet-glint-cover" :class="`support-pet-glint-${side}`" :style="{ opacity: blinkVisible ? 1 : 0 }" />
      <span v-for="side in ['left', 'right']" :key="`glint-${side}`" class="support-pet-layer" :class="`support-pet-glint-${side}`" :style="{ backgroundImage: `url(${rig})`, opacity: blinkVisible ? 1 : 0, transform: `scaleY(${glintScale})` }" />
    </span>
  </span>
</template>

<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import rig from './patrick-rig.webp'
import type { PetRenderer } from './petRenderer'
import { actionKeys, ease, mixPose, neutralPose, samplePose, nextIdleGesture, springStep, groundShadow, type PetAction } from './petMotion'

const props = defineProps<{ action: PetAction; sequence: number; tilt?: number; look?: { x: number; y: number }; resting?: boolean; engaged?: boolean; dragging?: boolean }>()
const emit = defineEmits<{ phase: [phase: PetAction] }>()
const canvas = ref<HTMLCanvasElement>(), poseElement = ref<HTMLElement>(), shadowElement = ref<HTMLElement>()
const rendererReady = ref(false), blinkVisible = ref(false), glintScale = ref(1), phase = ref<PetAction>('idle')
let renderer: PetRenderer | null = null
let frame = 0, elapsed = 0, previous = 0, clock = 0, transitionTime = 0
let pose = neutralPose(), transition: ReturnType<typeof neutralPose> | null = null
let gazeX = { value: 0, velocity: 0 }, gazeY = { value: 0, velocity: 0 }, tilt = { value: 0, velocity: 0 }
let keys = actionKeys('idle'), disposed = false, lastIdle: PetAction = 'idle'
let nextBlink = 4000 + Math.random() * 3000, blinkStart = -1000
let media: MediaQueryList | undefined, image: HTMLImageElement | undefined, initializing = false

function begin(value: PetAction, blend = false) {
  if (blend) { transition = { ...pose }; transitionTime = 0 } else transition = null
  keys = actionKeys(value)
  if (value === 'idle') keys[keys.length - 1]!.at = 2400 + Math.random() * 3300
  elapsed = 0; phase.value = value; emit('phase', value)
}
function render() {
  const still = media?.matches || props.resting
  const breath = still ? 0 : Math.sin(clock / 4200 * Math.PI * 2)
  const blinkTime = clock - blinkStart
  const naturalBlink = !still && blinkTime >= 0 && blinkTime < 310 ? Math.sin(Math.PI * blinkTime / 310) ** 2 : 0
  const moving = { ...pose, belly: pose.belly + breath * .045,
    lookX: pose.lookX + (still ? 0 : gazeX.value), lookY: pose.lookY + (still ? 0 : gazeY.value),
    yaw: pose.yaw + (still ? 0 : gazeX.value * 8 + tilt.value * .25),
    pitch: pose.pitch + (still ? 0 : -gazeY.value * 3),
    head: pose.head + (still ? 0 : -gazeX.value * 2), blink: Math.max(pose.blink, naturalBlink) }
  const dragLean = tilt.value
  const bob = pose.bob - breath * .45, lean = pose.lean + (still ? 0 : dragLean)
  const stretch = pose.stretch + breath * .0035
  if (poseElement.value) poseElement.value.style.transform = `translate3d(${pose.shift}px,${bob}px,0) rotate(${lean}deg) scale(${1/Math.sqrt(stretch)},${stretch})`
  if (shadowElement.value) {
    const shadow = groundShadow(bob, pose.shift)
    shadowElement.value.style.transform = `translateX(${shadow.x}px) scaleX(${shadow.scale})`
    shadowElement.value.style.opacity = String(shadow.opacity)
    shadowElement.value.style.filter = `blur(${shadow.blur}px)`
  }
  renderer?.draw(moving)
  blinkVisible.value = !rendererReady.value && moving.blink > .001
  glintScale.value = 1 - .96 * moving.blink
}
function schedule() { if (!disposed && !document.hidden && !media?.matches && !props.resting && !frame) frame = requestAnimationFrame(tick) }
function tick(time: number) {
  frame = 0
  const dt = previous ? Math.min(80, time - previous) : 0
  previous = time; clock += dt; elapsed += dt
  gazeX = springStep(gazeX.value, gazeX.velocity, props.look?.x ?? 0, dt / 1000)
  gazeY = springStep(gazeY.value, gazeY.velocity, props.look?.y ?? 0, dt / 1000)
  tilt = springStep(tilt.value, tilt.velocity, props.tilt ?? 0, dt / 1000, 16)
  if (clock >= nextBlink) { blinkStart = clock; nextBlink = clock + 4200 + Math.random() * 3100 }
  const target = samplePose(keys, elapsed)
  if (transition) {
    transitionTime += dt; pose = mixPose(transition, target, ease(transitionTime / 190))
    if (transitionTime >= 190) transition = null
  } else pose = target
  if (elapsed >= keys[keys.length - 1]!.at) {
    if (props.action === 'think') begin('think')
    else if (phase.value === 'idle' && !props.engaged && !props.dragging && !props.look) {
      lastIdle = nextIdleGesture(lastIdle, Math.random()); begin(lastIdle)
    } else begin('idle')
  }
  render(); schedule()
}
function play() {
  cancelAnimationFrame(frame); frame = 0; previous = 0
  begin(props.action, true)
  if (media?.matches || props.resting) { pose = neutralPose(); gazeX = gazeY = tilt = { value: 0, velocity: 0 }; render() } else schedule()
}
async function initialize() {
  if (disposed || !canvas.value || !image?.complete || !image.naturalWidth || initializing) return
  initializing = true
  try {
    const { createPetRenderer } = await import('./petRenderer')
    if (disposed || !canvas.value) return
    renderer?.dispose(); renderer = createPetRenderer(canvas.value, image)
    rendererReady.value = !!renderer; render(); schedule()
  } catch { rendererReady.value = false }
  finally { initializing = false }
}
function lost(event: Event) { event.preventDefault(); rendererReady.value = false; renderer?.dispose(); renderer = null; render() }
function visibility() { cancelAnimationFrame(frame); frame = 0; previous = 0; schedule() }
function preference() { pose = neutralPose(); play() }
watch(() => [props.action, props.sequence, props.resting], play)
watch(() => props.engaged, engaged => {
  if (engaged && ['look', 'stretch', 'shuffle', 'bounce'].includes(phase.value)) begin('idle', true)
})
onMounted(() => {
  media = matchMedia('(prefers-reduced-motion: reduce)'); media.addEventListener('change', preference)
  document.addEventListener('visibilitychange', visibility)
  canvas.value?.addEventListener('webglcontextlost', lost)
  canvas.value?.addEventListener('webglcontextrestored', initialize)
  image = new Image(); image.onload = () => { void initialize() }; image.src = rig
  play()
})
onBeforeUnmount(() => {
  disposed = true; cancelAnimationFrame(frame); renderer?.dispose()
  if (image) image.onload = null
  media?.removeEventListener('change', preference); document.removeEventListener('visibilitychange', visibility)
  canvas.value?.removeEventListener('webglcontextlost', lost); canvas.value?.removeEventListener('webglcontextrestored', initialize)
})
</script>

<style scoped>
.support-pet-art, .support-pet-pose, .support-pet-layer { display: block; position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; }
.support-pet-shadow { position: absolute; left: 23%; width: 56%; height: 4%; bottom: 1.5%; border-radius: 50%; background: #273329; pointer-events: none; transform-origin: center; }
.support-pet-pose { transform-origin: 50% 96%; will-change: transform; filter: drop-shadow(1px 2px 1.5px rgb(56 35 29 / 10%)); }
.support-pet-layer { background-repeat: no-repeat; background-size: 100% 100%; object-fit: contain; }
.support-pet-glint-cover { background: #090a0e; }
.support-pet-glint-left { clip-path: polygon(42.283% 26.128%,47.535% 26.331%,47.027% 32.702%,41.944% 32.702%); transform-origin: 44.824297188755025% 29.448795180722893%; }
.support-pet-glint-right { clip-path: polygon(57.277% 28.771%,62.360% 29.110%,61.598% 34.803%,57.023% 34.667%); transform-origin: 59.73393574297189% 31.753012048192772%; }
</style>
