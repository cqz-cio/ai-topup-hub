<template>
  <div class="brand-scene-motion" :data-ready="ready" :data-loading-state="loadingState" aria-hidden="true">
    <canvas :key="canvasVersion" ref="canvas" />
    <div class="brand-scene-motion__wash" />
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, onBeforeUnmount, ref } from 'vue'
import type { JellyfishingScene } from '../utils/jellyfishing/renderer.js'

const canvas = ref<HTMLCanvasElement>()
const ready = ref(false)
const loadingState = ref('waiting')
const canvasVersion = ref(0)
let stop = () => {}

onMounted(() => {
  const preference = window.matchMedia('(prefers-reduced-motion: reduce)')
  let scene: JellyfishingScene | undefined
  let generation = 0

  const reset = () => {
    generation++
    ready.value = false
    loadingState.value = 'waiting'
    scene?.dispose()
    scene = undefined
    // A disposed WebGL context cannot be reused when motion is enabled again.
    canvasVersion.value++
  }
  const update = async () => {
    reset()
    if (preference.matches) { loadingState.value = 'reduced-motion'; return }
    const current = generation
    try {
      loadingState.value = 'loading'
      await nextTick()
      // Motion is part of the visible scene, regardless of connection speed.
      // Start its assets without waiting for unrelated page images or idle time.
      const { mountJellyfishing } = await import('../utils/jellyfishing/renderer.js')
      if (generation !== current || !canvas.value) return
      scene = mountJellyfishing(canvas.value, {
        onReady: () => {
          if (generation === current) { ready.value = true; loadingState.value = 'ready' }
        },
        onFallback: () => {
          if (generation === current) { ready.value = false; loadingState.value = 'fallback' }
        },
      })
    } catch {
      // Keep the existing CSS wallpaper if WebGL or the lazy chunk is unavailable.
      if (generation === current) { ready.value = false; loadingState.value = 'fallback' }
    }
  }

  preference.addEventListener('change', update)
  stop = () => {
    preference.removeEventListener('change', update)
    reset()
  }
  void update()
})

onBeforeUnmount(() => stop())
</script>

<style scoped>
.brand-scene-motion {
  position: fixed;
  inset: 0;
  z-index: -1;
  overflow: hidden;
  pointer-events: none;
  opacity: 0;
  filter: saturate(0.9);
}

.brand-scene-motion[data-ready='true'] {
  opacity: 1;
}

canvas,
.brand-scene-motion__wash {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.brand-scene-motion__wash {
  background: rgb(255 255 255 / 24%);
}

.dark .brand-scene-motion {
  filter: brightness(0.4) saturate(0.88);
}

.dark .brand-scene-motion__wash {
  background: transparent;
}

@media (prefers-reduced-motion: reduce) {
  .brand-scene-motion {
    display: none;
  }
}
</style>
