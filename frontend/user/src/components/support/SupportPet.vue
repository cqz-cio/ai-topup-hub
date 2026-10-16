<template>
  <span class="support-pet-art" :data-motion="phase" :data-renderer="rendererReady ? 'connected-mesh' : 'original'">
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
import { createPetRenderer } from './petRenderer'
import { actionKeys, ease, mixPose, neutralPose, samplePose, type PetAction } from './petMotion'

const props = defineProps<{ action: PetAction; sequence: number }>()
const emit = defineEmits<{ phase: [phase: PetAction] }>()
const canvas = ref<HTMLCanvasElement>(), poseElement = ref<HTMLElement>()
const rendererReady = ref(false), blinkVisible = ref(false), glintScale = ref(1), phase = ref<PetAction>('idle')
let renderer: ReturnType<typeof createPetRenderer> = null
let frame = 0, elapsed = 0, previous = 0, clock = 0, transitionTime = 0
let pose = neutralPose(), transition: ReturnType<typeof neutralPose> | null = null
let keys = actionKeys('idle'), disposed = false
let media: MediaQueryList | undefined, image: HTMLImageElement | undefined

function setPhase(value: PetAction) { phase.value = value; emit('phase', value) }
function render() {
  const breath = media?.matches ? 0 : Math.sin(clock / 4500 * Math.PI * 2)
  if (poseElement.value) poseElement.value.style.transform = `translate3d(0,${pose.bob - breath * .65}px,0) rotate(${pose.lean}deg) scaleY(${pose.stretch + breath * .0025})`
  renderer?.draw(pose)
  blinkVisible.value = pose.blink > .001
  glintScale.value = 1 - .96 * pose.blink
}
function schedule() { if (!disposed && !document.hidden && !media?.matches && renderer && !frame) frame = requestAnimationFrame(tick) }
function tick(time: number) {
  frame = 0
  const dt = previous ? Math.min(80, time - previous) : 0
  previous = time; clock += dt
  if (transition) {
    transitionTime += dt; pose = mixPose(transition, neutralPose(), ease(transitionTime / 220))
    if (transitionTime >= 220) transition = null
  } else {
    elapsed += dt; pose = samplePose(keys, elapsed)
    if (elapsed >= keys[keys.length - 1]!.at) {
      const next = phase.value === 'idle' ? 'blink' : (props.action === 'think' && phase.value === 'think' ? 'think' : 'idle')
      keys = actionKeys(next); elapsed = 0; setPhase(next)
    }
  }
  render(); schedule()
}
function play() {
  if (!renderer) return
  cancelAnimationFrame(frame); frame = 0; previous = 0; elapsed = 0
  keys = actionKeys(props.action); setPhase(props.action)
  transition = media?.matches ? null : { ...pose }; transitionTime = 0
  if (media?.matches) { pose = neutralPose(); render() } else schedule()
}
function initialize() {
  if (disposed || !canvas.value || !image?.complete) return
  renderer?.dispose(); renderer = createPetRenderer(canvas.value, image)
  rendererReady.value = !!renderer; render(); play()
}
function lost(event: Event) { event.preventDefault(); cancelAnimationFrame(frame); frame = 0; rendererReady.value = false; renderer?.dispose(); renderer = null }
function visibility() { cancelAnimationFrame(frame); frame = 0; previous = 0; schedule() }
function preference() { pose = neutralPose(); play() }
watch(() => [props.action, props.sequence], play)
onMounted(() => {
  media = matchMedia('(prefers-reduced-motion: reduce)'); media.addEventListener('change', preference)
  document.addEventListener('visibilitychange', visibility)
  canvas.value?.addEventListener('webglcontextlost', lost)
  canvas.value?.addEventListener('webglcontextrestored', initialize)
  image = new Image(); image.onload = initialize; image.src = rig
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
.support-pet-pose { transform-origin: 50% 96%; will-change: transform; }
.support-pet-layer { background-repeat: no-repeat; background-size: 100% 100%; object-fit: contain; }
.support-pet-glint-cover { background: #090a0e; }
.support-pet-glint-left { clip-path: polygon(42.283% 26.128%,47.535% 26.331%,47.027% 32.702%,41.944% 32.702%); transform-origin: 44.824297188755025% 29.448795180722893%; }
.support-pet-glint-right { clip-path: polygon(57.277% 28.771%,62.360% 29.110%,61.598% 34.803%,57.023% 34.667%); transform-origin: 59.73393574297189% 31.753012048192772%; }
</style>
