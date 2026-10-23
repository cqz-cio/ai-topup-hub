<template>
  <div class="ocean-scene-motion" :data-scene="scene" :data-ready="ready" :data-loading-state="loadingState" aria-hidden="true">
    <canvas :key="`art-${version}`" ref="art" class="ocean-scene-motion__art" />
    <div class="ocean-scene-motion__wash" />
    <canvas :key="`bubbles-${version}`" ref="bubbles" />
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { OceanScene, OceanSceneController } from '../utils/oceanScene/renderer.js'

const props = defineProps<{ scene: OceanScene }>()
const art = ref<HTMLCanvasElement>()
const bubbles = ref<HTMLCanvasElement>()
const ready = ref(false)
const loadingState = ref('waiting')
const version = ref(0)
let stop = () => {}

onMounted(() => {
  const preference = window.matchMedia('(prefers-reduced-motion: reduce)')
  let controller: OceanSceneController | undefined
  let generation = 0
  const reset = () => {
    generation++
    ready.value = false
    controller?.dispose()
    controller = undefined
    version.value++
  }
  const update = async () => {
    reset()
    if (preference.matches) { loadingState.value = 'reduced-motion'; return }
    const current = generation
    const scene = props.scene
    loadingState.value = 'loading'
    try {
      await nextTick()
      const { mountOceanScene } = await import('../utils/oceanScene/renderer.js')
      if (current !== generation || !art.value || !bubbles.value) return
      controller = mountOceanScene(art.value, bubbles.value, scene, {
        onReady: () => { if (current === generation) { ready.value = true; loadingState.value = 'ready' } },
        onFallback: () => { if (current === generation) { ready.value = false; loadingState.value = 'fallback' } },
      })
    } catch {
      if (current === generation) { ready.value = false; loadingState.value = 'fallback' }
    }
  }
  const unwatch = watch(() => props.scene, update)
  preference.addEventListener('change', update)
  stop = () => { unwatch(); preference.removeEventListener('change', update); reset() }
  void update()
})
onBeforeUnmount(() => stop())
</script>

<style scoped>
.ocean-scene-motion { position: fixed; inset: 0; z-index: -1; overflow: hidden; pointer-events: none; opacity: 0; }
.ocean-scene-motion[data-ready='true'] { opacity: 1; }
canvas, .ocean-scene-motion__wash { position: absolute; inset: 0; width: 100%; height: 100%; }
.ocean-scene-motion__wash { background: rgb(255 255 255 / 14%); }
.dark .ocean-scene-motion { filter: brightness(.4) saturate(.88); }
.dark .ocean-scene-motion__wash { background: transparent; }
@media (prefers-reduced-motion: reduce) { .ocean-scene-motion { display: none; } }
</style>
