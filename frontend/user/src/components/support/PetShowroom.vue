<template>
  <Teleport to="body">
    <dialog ref="dialog" class="pet-showroom" :aria-label="copy.title" @cancel.prevent="close" @click="backdrop">
      <div class="pet-showroom-inner">
        <header><div><span class="pet-eyebrow">MEET PI</span><h2>{{ copy.title }}</h2></div><button class="pet-close" :aria-label="copy.close" @click="close">×</button></header>
        <p class="pet-instruction">{{ copy.hint }}</p>
        <div ref="stage" class="pet-stage" role="img" :aria-label="copy.preview" @pointermove="follow" @pointerleave="resetLook" @pointercancel="resetLook">
          <div class="pet-orbit" aria-hidden="true" />
          <div class="pet-model"><SupportPet :action="action" :sequence="sequence" :look="paused ? undefined : look" :resting="paused" engaged /></div>
        </div>
        <PetVoiceBoard :locale="locale" :paused="paused" :quote-id="quote.id" @speaking="speak" />
        <section class="pet-quote" aria-live="polite" aria-atomic="true">
          <div><small>{{ quote.label }}</small><p>{{ quote.text }}</p></div>
          <button type="button" @click="nextQuote">{{ copy.quote }}</button>
        </section>
        <div class="pet-showroom-actions">
          <button class="pet-pause" :aria-pressed="paused" @click="togglePause">{{ paused ? copy.resume : copy.pause }}</button>
          <button @click="play('wave')">{{ copy.wave }}</button><button @click="play('bounce')">{{ copy.jump }}</button>
          <button @click="play('laugh')">{{ copy.laugh }}</button><button @click="play('stretch')">{{ copy.stretch }}</button>
        </div>
      </div>
    </dialog>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import SupportPet from './SupportPet.vue'
import PetVoiceBoard from './PetVoiceBoard.vue'
import type { PetAction } from './petMotion'
import { pickPetQuote } from './petQuotes'

const props = defineProps<{ locale: string }>()
const emit = defineEmits<{ close: [] }>()
const quoteId = ref(''), quoteRandom = ref(0)
const quote = computed(() => pickPetQuote(props.locale, quoteId.value, quoteRandom.value))
function nextQuote() { quoteId.value = quote.value.id; quoteRandom.value = Math.random() }
const dialog = ref<HTMLDialogElement>(), stage = ref<HTMLElement>(), paused = ref(false)
const action = ref<PetAction>('idle'), sequence = ref(0), look = ref({ x: 0, y: 0 })
const copy = computed(() => props.locale === 'en-US' ? {
  quote: 'Another line', title: 'Meet Pi', hint: 'Move your pointer to catch Pi’s eye · try a little action below', close: 'Close Pi preview', preview: 'Pi wearing sunglasses and a gold chain', pause: 'Pause motion', resume: 'Resume motion', wave: 'Say hello', jump: 'Little jump', laugh: 'Happy dance', stretch: 'Stretch',
} : props.locale === 'zh-TW' ? {
  quote: '再來一句', title: '和派總見個面', hint: '移動滑鼠，派總會看向你 · 試試下面的小動作', close: '關閉派總預覽', preview: '戴著墨鏡和金鏈子的派總', pause: '暫停動作', resume: '繼續活動', wave: '打個招呼', jump: '蹦躂一下', laugh: '開心一下', stretch: '伸個懶腰',
} : {
  quote: '再来一句', title: '和派总见个面', hint: '移动鼠标，派总会看向你 · 试试下面的小动作', close: '关闭派总预览', preview: '戴着墨镜和金链子的派总', pause: '暂停动作', resume: '继续活动', wave: '打个招呼', jump: '蹦跶一下', laugh: '开心一下', stretch: '伸个懒腰',
})

function resetLook() { look.value = { x: 0, y: 0 } }
function follow(event: PointerEvent) {
  if (paused.value || event.pointerType !== 'mouse' || !stage.value) return
  const bounds = stage.value.getBoundingClientRect()
  if (!bounds.width || !bounds.height) return
  const clamp = (value: number) => Math.max(-1, Math.min(1, value))
  look.value = { x: clamp((event.clientX - bounds.left) / bounds.width * 2 - 1), y: clamp((event.clientY - bounds.top) / bounds.height * 2 - 1) }
}
function togglePause() { paused.value = !paused.value; resetLook() }
function play(value: PetAction) { paused.value = false; action.value = value; sequence.value++ }
function speak() { if (!paused.value) { action.value = 'reply'; sequence.value++ } }
function close() { dialog.value?.close(); emit('close') }
function backdrop(event: MouseEvent) {
  if (event.target !== dialog.value) return
  const bounds = dialog.value!.getBoundingClientRect()
  if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) close()
}
onMounted(() => {
  paused.value = matchMedia('(prefers-reduced-motion: reduce)').matches
  dialog.value?.showModal()
  document.addEventListener('visibilitychange', resetLook)
  window.addEventListener('blur', resetLook)
})
onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', resetLook)
  window.removeEventListener('blur', resetLook)
  dialog.value?.close()
})
</script>

<style scoped>
.pet-showroom { margin: auto; width: min(520px, calc(100vw - 24px)); max-height: calc(100dvh - 24px); padding: 0; border: 1px solid #fff9; border-radius: 28px; color: #29372c; background: #f8f7ed; box-shadow: 0 28px 100px #11292050; overflow: auto; }
.pet-showroom::backdrop { background: #172a2855; backdrop-filter: blur(7px); }
.pet-showroom-inner { padding: 24px; background: radial-gradient(ellipse at 50% 35%, #fffdf9 0, #f3f4e7 65%, #e9eedf 100%); }
header { display: flex; justify-content: space-between; align-items: center; } h2 { font-size: 24px; font-weight: 750; margin: 5px 0 0; letter-spacing: -.5px; }
.pet-eyebrow { font-size: 10px; font-weight: 750; letter-spacing: 2.2px; color: #7c8969; }
button { font: inherit; cursor: pointer; color: inherit; } button:focus-visible { outline: 2px solid #587945; outline-offset: 3px; }
.pet-close { width: 36px; height: 36px; border-radius: 50%; background: #fff9; border: 1px solid #d8dfce; font-size: 25px; }
.pet-instruction { margin: 12px 0 0; color: #6c7866; font-size: 12px; }
.pet-stage { position: relative; height: min(47dvh, 395px); min-height: 200px; user-select: none; }
.pet-model { position: absolute; height: 100%; aspect-ratio: .8; left: 50%; transform: translateX(-50%); pointer-events: none; }
.pet-orbit { position: absolute; width: 68%; height: 14%; left: 16%; bottom: -2%; border: 1px solid #b9c5aa88; border-radius: 50%; box-shadow: 0 0 0 14px #b9c5aa12; }
.pet-quote { display: flex; align-items: center; gap: 12px; margin: 12px 0; padding: 12px 14px; border: 1px solid #d4ddc8; border-radius: 16px; background: #fff9; }
.pet-quote div { flex: 1; min-width: 0; }
.pet-quote small { display: block; color: #6c7866; font-size: 10px; }
.pet-quote p { margin: 5px 0 0; font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
.pet-quote button { flex-shrink: 0; padding: 8px 10px; border-radius: 10px; border: 1px solid #d4ddc8; background: #f1f5e9; font-size: 12px; }
.pet-showroom-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 7px; }
.pet-showroom-actions button { font-size: 12px; padding: 10px; border-radius: 12px; background: #fff9; border: 1px solid #d4ddc8; }
.pet-showroom-actions button[aria-pressed=true] { background: #e0e9d6; border-color: #7d956a; }
.pet-pause { grid-column: 1 / -1; }
@media (max-width: 420px) { .pet-showroom-inner { padding: 17px; } h2 { font-size: 21px; } .pet-instruction { font-size: 11px; } .pet-stage { height: min(43dvh, 340px); } }
</style>
