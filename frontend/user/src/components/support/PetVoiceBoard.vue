<template>
  <section class="pet-voice-board" :aria-label="copy.title">
    <div class="voice-heading"><strong>{{ copy.title }}</strong><span>{{ copy.hint }}</span></div>
    <div class="voice-clips">
      <div v-for="clip in petVoiceClips" :key="clip.id" class="voice-clip">
        <button type="button" :aria-label="`${active(clip.id) ? copy.stop : copy.play}：${clip.text[language]}`" :aria-pressed="active(clip.id)" @click="player.toggle(clip.id, clip.src)">
          <span class="voice-icon" aria-hidden="true">{{ active(clip.id) ? '■' : '▶' }}</span>
          <span>{{ clip.text[language] }}</span>
        </button>
        <a :href="clip.source" target="_blank" rel="noopener noreferrer" :aria-label="`${copy.source}：${clip.text[language]}`">{{ copy.source }} ↗</a>
      </div>
    </div>
    <p v-if="state.status !== 'idle'" class="voice-status" role="status">{{ state.status === 'error' ? copy.error : state.status === 'loading' ? copy.loading : copy.playing }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { createPetVoicePlayer, type VoiceState } from './petVoicePlayer'
import { petVoiceClips } from './petVoiceClips'

const props = defineProps<{ locale: string; paused: boolean; quoteId: string }>()
const emit = defineEmits<{ speaking: [] }>()
const state = ref<VoiceState>({ id: null, status: 'idle' })
const player = createPetVoicePlayer(next => {
  state.value = next
  if (next.status === 'playing') emit('speaking')
})
const language = computed(() => props.locale === 'en-US' ? 2 : props.locale === 'zh-TW' ? 1 : 0)
const copy = computed(() => props.locale === 'en-US' ? {
  title: 'Mandarin original clips', hint: 'Tap to listen', play: 'Play Mandarin clip', stop: 'Stop', source: 'Source', loading: 'Loading… Tap again to cancel.', playing: 'Playing · tap again to stop', error: 'Could not play. Tap the clip to retry.',
} : props.locale === 'zh-TW' ? {
  title: '中文動畫原聲', hint: '點一下，聽派大星說', play: '播放原聲', stop: '停止播放', source: '片段來源', loading: '正在載入，再點一次可取消', playing: '正在播放 · 再點一次停止', error: '暫時播放不了，點擊台詞重試。',
} : {
  title: '中文动画原声', hint: '点一下，听派大星说', play: '播放原声', stop: '停止播放', source: '片段来源', loading: '正在加载，再点一次可取消', playing: '正在播放 · 再点一次停止', error: '暂时播放不了，点击台词重试。',
})
function active(id: string) { return state.value.id === id && (state.value.status === 'loading' || state.value.status === 'playing') }
function onHidden() { if (document.hidden) player.stop() }
watch(() => props.paused, value => { if (value) player.stop() })
watch(() => [props.locale, props.quoteId], () => player.stop())
onMounted(() => {
  document.addEventListener('visibilitychange', onHidden)
  window.addEventListener('blur', player.stop)
})
onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onHidden)
  window.removeEventListener('blur', player.stop)
  player.dispose()
})
</script>

<style scoped>
.pet-voice-board { margin: 14px 0; padding: 13px; border: 1px solid #d4ddc8; border-radius: 16px; background: #f1f5e9; }
.voice-heading { display: flex; align-items: baseline; flex-wrap: wrap; gap: 4px 10px; margin-bottom: 10px; }
.voice-heading strong { font-size: 12px; color: #344b32; } .voice-heading > span { font-size: 10px; color: #6c7866; }
.voice-clips { display: grid; grid-template-columns: 1fr 1fr; gap: 9px; }
.voice-clip { min-width: 0; }
.voice-clip button { width: 100%; min-height: 48px; display: flex; align-items: center; gap: 8px; padding: 9px; border-radius: 11px; border: 1px solid #d4ddc8; background: #fff9; color: #29372c; font: inherit; font-size: 12px; text-align: left; cursor: pointer; }
.voice-clip button[aria-pressed=true] { border-color: #7d956a; background: #e0e9d6; }
.voice-icon { flex-shrink: 0; font-size: 13px; color: #567346; }
.voice-clip a { display: inline-block; margin: 5px 0 0 3px; font-size: 10px; color: #66795b; text-decoration: underline; text-underline-offset: 2px; }
.voice-clip button:focus-visible, .voice-clip a:focus-visible { outline: 2px solid #587945; outline-offset: 3px; }
.voice-status { margin: 7px 0 0; font-size: 11px; color: #59694e; }
@media (max-width: 360px) { .voice-clips { grid-template-columns: 1fr; } .voice-clip button { min-height: 42px; } }
</style>
