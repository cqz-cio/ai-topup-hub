<template>
  <PetShowroom v-if="showroomOpen" :locale="app.locale" @close="closeShowroom" />
  <aside ref="widget" class="support-widget" :style="{ ...widgetStyle, '--pet-scale': petScale }" :data-resting="resting" :data-open="open" :data-moved="!!widgetStyle" :data-dragging="dragging" :data-panel-overlap="panelOverlap" @keydown.esc.stop.prevent="escape">
    <section v-if="open" id="pi-support-window" ref="panel" class="support-window" :style="panelStyle" role="dialog" :aria-labelledby="'pi-support-title'" :aria-busy="busy">
      <header class="support-header">
        <img :src="avatar" alt="" class="support-avatar" />
        <div class="support-identity"><h2 id="pi-support-title">{{ copy.name }}</h2><span>{{ copy.subtitle }}</span></div>
        <button ref="closeButton" class="support-icon" type="button" :aria-label="copy.close" @click="close"><Minus :size="17" /></button>
      </header>
      <div ref="body" class="support-body">
        <div v-if="messages.length === 0" class="support-welcome"><h3>{{ copy.title }}</h3><p>{{ copy.intro }}</p></div>
        <div v-if="messages.length === 0 || showTopics" class="support-topics">
          <button v-for="(topic, index) in copy.topics" :key="index" type="button" :disabled="busy" @click="ask(copy.questions[index]!)">{{ topic }}<small>{{ copy.hints[index] }}</small></button>
        </div>
        <div class="support-messages" role="log" aria-live="polite" aria-relevant="additions text">
          <div v-for="message in messages" :key="message.id" class="support-message" :class="`support-message-${message.role}`"><p>{{ message.text }}</p><div v-if="message.links?.length" class="support-message-links"><RouterLink v-for="link in message.links" :key="link.to" :to="link.to">{{ link.label }}</RouterLink></div></div>
          <p v-if="busy" class="support-message support-message-assistant">{{ copy.thinking }}</p>
        </div>
        <button v-if="messages.length && !showTopics && !busy" class="support-more" type="button" @click="resetConversation">{{ copy.newQuestion }}</button>
      </div>
      <form class="support-composer" @submit.prevent="ask(input)">
        <input ref="inputElement" v-model="input" type="text" maxlength="500" autocomplete="off" :placeholder="copy.placeholder" :aria-label="copy.placeholder" :disabled="busy" @keydown.enter="preventComposition" />
        <button type="submit" :disabled="busy || !input.trim()">{{ copy.send }}</button>
      </form>
      <footer class="support-footer"><span>{{ copy.footer }}</span><button type="button" :aria-expanded="contactsOpen" @click="contactsOpen = !contactsOpen">{{ copy.human }}</button></footer>
      <section v-if="contactsOpen" class="support-contacts" :aria-label="copy.contactTitle">
        <div class="support-contact-heading"><strong>{{ copy.contactTitle }}</strong><button type="button" class="support-icon" :aria-label="copy.closeContacts" @click="contactsOpen = false"><X :size="16" /></button></div>
        <p class="support-copy-status">07:00–24:00 · UTC+8</p>
        <div class="support-contact-row"><span>{{ copy.wechat }}<strong>{{ supportContacts.wechat }}</strong></span><button type="button" @click="copyContact(supportContacts.wechat)">{{ copy.copy }}</button></div>
        <div class="support-contact-row"><span>{{ copy.qq }}<strong>{{ supportContacts.qq }}</strong></span><button type="button" @click="copyContact(supportContacts.qq)">{{ copy.copy }}</button></div>
        <a :href="supportContacts.telegramUrl" target="_blank" rel="noopener noreferrer" class="support-telegram">{{ copy.telegram }} · {{ supportContacts.telegram }}<ExternalLink :size="14" /></a>
        <span v-if="copyStatus" class="support-copy-status" role="status">{{ copyStatus }}</span>
      </section>
    </section>
    <div v-if="open || greetingVisible || reaction || quote" ref="speechElement" class="support-speech" :style="speechStyle" :data-side="speechSide" aria-hidden="true">{{ speech }}<small v-if="quote && !busy && !reaction" class="support-quote-label">{{ quote.label }}</small></div>
    <button ref="launcher" class="support-launcher" type="button" :aria-label="petCopy.play" :title="petCopy.hint" :aria-expanded="open" aria-controls="pi-support-window" @click="activate" @dblclick.prevent="doublePlay" @contextmenu.prevent="showMenu" @pointerdown="pointerKind = $event.pointerType; pointerDown($event)" @pointermove="movePet" @pointerleave="look = undefined" @pointerup="finish" @pointercancel="finish" @lostpointercapture="finish" @dragstart.prevent>
      <SupportPet :action="action" :sequence="sequence" :tilt="tilt" :look="look" :resting="showroomOpen || (resting && !busy)" :engaged="open || menuOpen || !!quote" :dragging="dragging" @phase="phase = $event" />
    </button>
    <span v-if="resting" class="support-sleep" aria-hidden="true">z z Z</span>
    <div class="support-pet-tools">
      <button class="support-chat-toggle" type="button" :aria-label="open ? copy.close : copy.open" :aria-expanded="open" aria-controls="pi-support-window" @click="toggle"><MessageCircle :size="13" />{{ copy.name }}<span>{{ petCopy.chat }}</span></button>
      <button ref="menuButton" class="support-pet-options" type="button" :aria-label="petCopy.settings" :aria-expanded="menuOpen" aria-controls="pi-pet-settings" @click="showMenu"><Ellipsis :size="16" /></button>
    </div>
    <section v-if="menuOpen" id="pi-pet-settings" ref="menuElement" class="support-pet-menu" :style="menuStyle" role="dialog" :aria-label="petCopy.settings">
      <header><strong>{{ petCopy.settings }}</strong><button class="support-icon" type="button" :aria-label="copy.close" @click="hideMenu"><X :size="15" /></button></header>
      <p>{{ petCopy.hint }}</p>
      <div class="support-pet-actions"><button type="button" :disabled="busy" @click="playPet('wave')">{{ petCopy.wave }}</button><button type="button" :disabled="busy" @click="playPet('laugh')">{{ petCopy.laugh }}</button><button type="button" :disabled="busy" @click="playPet('bounce')">{{ petCopy.bounce }}</button><button type="button" :disabled="busy" @click="playPet('stretch')">{{ petCopy.stretch }}</button></div>
      <button class="support-pet-setting" type="button" :disabled="busy || open" @click="sayQuote(true)"><MessageCircle :size="14" />{{ petCopy.quote }}</button>
      <button class="support-pet-setting" type="button" @click="openShowroom"><Maximize2 :size="14" />{{ petCopy.preview }}</button>
      <fieldset><legend>{{ petCopy.size }}</legend><div class="support-pet-sizes"><button v-for="(value, index) in sizes" :key="value" type="button" :aria-pressed="size === value" @click="size = value">{{ petCopy.sizes[index] }}</button></div></fieldset>
      <button class="support-pet-setting" type="button" :aria-pressed="resting" @click="resting = !resting; animate('idle')"><Moon :size="14" />{{ resting ? petCopy.wake : petCopy.rest }}</button>
      <button class="support-pet-setting" type="button" @click="resetPosition(); hideMenu()"><RotateCcw :size="14" />{{ petCopy.reset }}</button>
    </section>
  </aside>
</template>

<script setup lang="ts">
import { defineAsyncComponent, computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { ExternalLink, Minus, X, MessageCircle, Ellipsis, Moon, RotateCcw, Maximize2 } from 'lucide-vue-next'
import { useAppStore } from '../../stores/app'
import SupportPet from './SupportPet.vue'
const PetShowroom = defineAsyncComponent(() => import('./PetShowroom.vue'))
const showroomOpen = ref(false)
function openShowroom() { menuOpen.value = false; showroomOpen.value = true }
function closeShowroom() { showroomOpen.value = false; nextTick(() => menuButton.value?.focus({ preventScroll: true })) }
import avatar from './patrick-avatar.webp'
import { petHit, petLook, petStorageKey, readPetSettings, savePetSettings, type PetSize } from './petInteraction'
import type { PetAction } from './petMotion'
import { pickPetQuote, canOfferPetQuote, petQuoteDelay, type PetQuote } from './petQuotes'
import { getSupportCopy, supportContacts } from './supportDemo'
import { useSupportDrag } from './useSupportDrag'
import type { SupportLink } from './supportConversation'
import { askSupport, agentCopy, agentError, getSupportMode, type SupportMode } from './supportApi'

const supportMode = ref<SupportMode>('unknown')
const app = useAppStore(), copy = computed(() => ({ ...getSupportCopy(app.locale), ...agentCopy(app.locale, supportMode.value), name: app.locale === 'en-US' ? 'Pi' : app.locale === 'zh-TW' ? '派總' : '派总' }))
const route = useRoute()
const open = ref(false), busy = ref(false), input = ref(''), contactsOpen = ref(false), showTopics = ref(false), copyStatus = ref('')
const launcher = ref<HTMLButtonElement>(), closeButton = ref<HTMLButtonElement>(), inputElement = ref<HTMLInputElement>(), body = ref<HTMLElement>()
const widget = ref<HTMLElement>(), panel = ref<HTMLElement>(), speechElement = ref<HTMLElement>()
const { widgetStyle, panelStyle, speechStyle, speechSide, panelOverlap, dragging, tilt, resetPosition, pointerDown, pointerMove, finish, allowClick } = useSupportDrag(widget, launcher, panel, speechElement)
const action = ref<PetAction>('wave'), phase = ref<PetAction>('wave'), sequence = ref(0), greetingVisible = ref(true)
const messages = ref<{ id: number; role: 'user' | 'assistant'; text: string; links?: SupportLink[] }[]>([])
let greetingTimer: ReturnType<typeof setTimeout> | undefined, messageId = 0
let conversationId = crypto.randomUUID()
let request: AbortController | undefined, generation = 0
let modeRequest: AbortController | undefined
const speech = computed(() => busy.value ? copy.value.think : reaction.value || quote.value?.text || (phase.value === 'reply' ? copy.value.reply : phase.value === 'wave' ? copy.value.greeting : copy.value.idle))
function animate(next: PetAction) { action.value = next; sequence.value++ }
function close() { open.value = false; contactsOpen.value = false; nextTick(() => launcher.value?.focus({ preventScroll: true })) }
function escape() { if (menuOpen.value) { hideMenu(); return }; if (contactsOpen.value) contactsOpen.value = false; else close() }
const sizes = ['small', 'normal', 'large'] as const
const size = ref<PetSize>('normal'), resting = ref(false), menuOpen = ref(false), reaction = ref('')
const quote = ref<PetQuote>()
let previousQuote = '', quoteTimer: ReturnType<typeof setTimeout> | undefined, quoteHideTimer: ReturnType<typeof setTimeout> | undefined
const look = ref<{ x: number; y: number }>(), menuButton = ref<HTMLButtonElement>(), menuElement = ref<HTMLElement>()
const menuStyle = ref({ left: '12px', top: '12px' })
const petScale = computed(() => ({ small: .82, normal: 1, large: 1.2 })[size.value])
const petCopy = computed(() => app.locale === 'en-US' ? {
  quote: 'Say a line', preview: 'Meet Pi', play: 'Play with Pi; Enter to chat', hint: 'Poke my belly · double-click to play · drag to move', chat: 'Chat', settings: 'Pi’s little corner', wave: 'Say hello', laugh: 'Happy dance', bounce: 'Little jump', stretch: 'Stretch', size: 'Size', sizes: ['Small', 'Normal', 'Large'], rest: 'Quiet company', wake: 'Wake up', reset: 'Back to the corner', poke: 'Hey! That tickles.', happy: 'A little joy for your day!', hello: 'Hey there! Need a hand?', land: 'Found my spot.',
} : app.locale === 'zh-TW' ? {
  quote: '來句語錄', preview: '看看派總', play: '和派總互動；Enter 開啟客服', hint: '戳肚子 · 雙擊開心一下 · 按住拖動', chat: '聊聊', settings: '派總的小角落', wave: '打個招呼', laugh: '開心一下', bounce: '蹦躂一下', stretch: '伸個懶腰', size: '個頭大小', sizes: ['小隻', '剛好', '大隻'], rest: '安靜陪伴', wake: '叫醒派總', reset: '回到右下角', poke: '哎呀，有點癢！', happy: '今天也要開開心心～', hello: '派總在，有事招呼我！', land: '這個位置不錯。',
} : {
  quote: '来句语录', preview: '看看派总', play: '和派总互动；Enter 打开客服', hint: '戳肚子 · 双击开心一下 · 按住拖动', chat: '聊聊', settings: '派总的小角落', wave: '打个招呼', laugh: '开心一下', bounce: '蹦跶一下', stretch: '伸个懒腰', size: '个头大小', sizes: ['小只', '刚好', '大只'], rest: '安静陪伴', wake: '叫醒派总', reset: '回到右下角', poke: '哎呀，有点痒！', happy: '今天也要开开心心～', hello: '派总在，有事招呼我！', land: '这个位置不错。',
})
let clickTimer: ReturnType<typeof setTimeout> | undefined, reactionTimer: ReturnType<typeof setTimeout> | undefined
let pointerKind = ''
function clearQuote() { clearTimeout(quoteHideTimer); quote.value = undefined }
function mayChatter() {
  return canOfferPetQuote({ hidden: document.hidden, focused: document.hasFocus(), resting: resting.value,
    chatting: open.value, busy: busy.value, dragging: dragging.value, menu: menuOpen.value,
    preview: showroomOpen.value, occupied: !!reaction.value || greetingVisible.value || !!look.value })
}
function scheduleQuote() {
  clearTimeout(quoteTimer)
  if (document.hidden || !document.hasFocus() || resting.value || open.value || busy.value || dragging.value || menuOpen.value || showroomOpen.value) return
  quoteTimer = setTimeout(() => { if (mayChatter()) sayQuote(false); scheduleQuote() }, petQuoteDelay())
}
function sayQuote(manual: boolean) {
  if (busy.value || dragging.value || (!manual && !mayChatter())) return
  if (manual) { hideMenu(); resting.value = false; animate('wave') }
  clearTimeout(reactionTimer); reaction.value = ''; clearQuote()
  quote.value = pickPetQuote(app.locale, previousQuote); previousQuote = quote.value.id
  quoteHideTimer = setTimeout(clearQuote, 7000)
  scheduleQuote()
}
function resetChatter() {
  if (document.hidden || !document.hasFocus() || resting.value || open.value || busy.value || dragging.value || menuOpen.value || showroomOpen.value) clearQuote()
  scheduleQuote()
}
watch([open, busy, resting, dragging, menuOpen, showroomOpen], resetChatter)
watch(() => app.locale, () => { clearQuote(); scheduleQuote() })
onMounted(() => {
  document.addEventListener('visibilitychange', resetChatter)
  window.addEventListener('blur', resetChatter); window.addEventListener('focus', resetChatter)
  scheduleQuote()
})
onBeforeUnmount(() => {
  clearTimeout(quoteTimer); clearTimeout(quoteHideTimer)
  document.removeEventListener('visibilitychange', resetChatter)
  window.removeEventListener('blur', resetChatter); window.removeEventListener('focus', resetChatter)
})
function react(text: string) { clearQuote(); clearTimeout(reactionTimer); reaction.value = text; reactionTimer = setTimeout(() => { reaction.value = '' }, 2800) }
function playPet(next: 'poke' | 'laugh' | 'wave' | 'bounce' | 'stretch') {
  if (busy.value || dragging.value || (next === phase.value && next !== 'poke')) return
  if (menuOpen.value) hideMenu()
  resting.value = false; animate(next)
  react(next === 'poke' ? petCopy.value.poke : ['laugh', 'bounce', 'stretch'].includes(next) ? petCopy.value.happy : petCopy.value.hello)
}
function activate(event: MouseEvent) {
  if (!allowClick(event)) return
  if (event.detail === 0 || pointerKind === 'touch') { void toggle(); return }
  clearTimeout(clickTimer)
  if (event.detail > 1) return
  const rect = launcher.value!.getBoundingClientRect(), hit = petHit((event.clientX - rect.x) / rect.width, (event.clientY - rect.y) / rect.height)
  if (hit !== 'outside') clickTimer = setTimeout(() => playPet(hit === 'belly' ? 'poke' : 'wave'), 260)
}
function doublePlay(event: MouseEvent) { clearTimeout(clickTimer); if (allowClick(event) && pointerKind !== 'touch') playPet('laugh') }
function movePet(event: PointerEvent) {
  pointerKind = event.pointerType; pointerMove(event)
  if (event.pointerType !== 'mouse' || dragging.value) { look.value = undefined; return }
  const rect = launcher.value!.getBoundingClientRect()
  look.value = petLook((event.clientX - rect.x) / rect.width, (event.clientY - rect.y) / rect.height)
}
function hideMenu() { menuOpen.value = false; nextTick(() => menuButton.value?.focus({ preventScroll: true })) }
async function showMenu() {
  clearTimeout(clickTimer)
  if (menuOpen.value) { hideMenu(); return }
  menuOpen.value = true
  await nextTick()
  const trigger = menuButton.value!.getBoundingClientRect(), box = menuElement.value!.getBoundingClientRect(), pet = widget.value!.getBoundingClientRect()
  const view = window.visualViewport, left = view?.offsetLeft ?? 0, top = view?.offsetTop ?? 0
  const beside = pet.left - box.width - 12 >= left + 12 ? pet.left - box.width - 12 : trigger.right - box.width
  menuStyle.value = { left: `${Math.max(left + 12, Math.min(beside, left + (view?.width ?? innerWidth) - box.width - 12))}px`, top: `${Math.max(top + 12, Math.min(trigger.top - box.height - 10, top + (view?.height ?? innerHeight) - box.height - 12))}px` }
  menuElement.value?.querySelector('button')?.focus({ preventScroll: true })
}
function outsideMenu(event: PointerEvent) { if (menuOpen.value && !menuElement.value?.contains(event.target as Node) && !menuButton.value?.contains(event.target as Node)) menuOpen.value = false }
function viewportChanged() { menuOpen.value = false }
function suspendInteraction() { clearTimeout(clickTimer); look.value = undefined }
watch(dragging, (value, old) => { clearTimeout(clickTimer); if (value) { menuOpen.value = false; animate(busy.value ? 'think' : 'idle') }; if (!value && old && !busy.value) { animate('land'); react(petCopy.value.land) } })
watch([size, resting], () => savePetSettings({ size: size.value, resting: resting.value }))
onMounted(() => {
  try { const saved = readPetSettings(localStorage.getItem(petStorageKey)); size.value = saved.size; resting.value = saved.resting } catch { /* Optional preferences. */ }
  document.addEventListener('pointerdown', outsideMenu)
  window.addEventListener('resize', viewportChanged); window.addEventListener('blur', suspendInteraction)
  document.addEventListener('visibilitychange', suspendInteraction)
  window.visualViewport?.addEventListener('resize', viewportChanged)
})
onBeforeUnmount(() => {
  clearTimeout(clickTimer); clearTimeout(reactionTimer)
  document.removeEventListener('pointerdown', outsideMenu)
  window.removeEventListener('resize', viewportChanged); window.removeEventListener('blur', suspendInteraction)
  document.removeEventListener('visibilitychange', suspendInteraction)
  window.visualViewport?.removeEventListener('resize', viewportChanged)
})
async function toggle() {
  menuOpen.value = false; clearTimeout(clickTimer)
  if (open.value) { close(); return }
  open.value = true
  void refreshMode()
  if (!busy.value) animate('wave')
  await nextTick()
  if (matchMedia('(pointer: fine)').matches) inputElement.value?.focus({ preventScroll: true })
  else closeButton.value?.focus({ preventScroll: true })
  scrollMessages()
}
async function refreshMode() {
  modeRequest?.abort()
  const controller = modeRequest = new AbortController()
  const mode = await getSupportMode(controller.signal)
  if (modeRequest === controller && !controller.signal.aborted) supportMode.value = mode
}
function scrollMessages(newReply = false) {
  void nextTick(() => {
    const container = body.value
    if (!container) return
    const replies = container.querySelectorAll<HTMLElement>('.support-message-assistant')
    const latest = replies[replies.length - 1]
    // Long instructions start at their first step instead of showing only the tail.
    const top = newReply && latest && latest.offsetHeight > container.clientHeight - 24
      ? container.scrollTop + latest.getBoundingClientRect().top - container.getBoundingClientRect().top - 12
      : container.scrollHeight
    container.scrollTo({ top: Math.max(0, top), behavior: 'auto' })
  })
}
function preventComposition(event: KeyboardEvent) { if (event.isComposing || event.keyCode === 229) event.preventDefault() }
async function ask(question: string) {
  const value = question.trim().slice(0, 500)
  if (!value || busy.value) return
  const locale = app.locale, turn = ++generation
  void refreshMode()
  request?.abort(); request = new AbortController()
  messages.value.push({ id: ++messageId, role: 'user', text: value }); input.value = ''; busy.value = true; showTopics.value = false; contactsOpen.value = false
  animate('think'); scrollMessages()
  try {
    const response = await askSupport(value, locale, conversationId, typeof route.params.slug === 'string' ? route.params.slug : '', request.signal)
    if (turn !== generation) return
    if (response.mode === 'model') { modeRequest?.abort(); supportMode.value = 'model' }
    messages.value.push({ id: ++messageId, role: 'assistant', text: response.text, links: response.links })
    contactsOpen.value = response.handoff === true
  } catch (error) {
    if (turn !== generation) return
    messages.value.push({ id: ++messageId, role: 'assistant', text: agentError(error, locale) })
  } finally {
    if (turn === generation) {
      busy.value = false; animate('reply'); scrollMessages(true)
      if (messages.value.length > 40) messages.value.splice(0, messages.value.length - 40)
      if (open.value && matchMedia('(pointer: fine)').matches) nextTick(() => inputElement.value?.focus({ preventScroll: true }))
    }
  }
}

async function copyContact(value: string) {
  try { await navigator.clipboard.writeText(value); copyStatus.value = copy.value.copied }
  catch { copyStatus.value = copy.value.copyFailed }
}
onMounted(() => { greetingTimer = setTimeout(() => { greetingVisible.value = false }, 8000) })
function resetConversation() { generation++; request?.abort(); busy.value = false; conversationId = crypto.randomUUID(); messages.value = []; input.value = ''; showTopics.value = false; contactsOpen.value = false; animate('idle') }
watch(() => app.locale, resetConversation)
watch(() => route.params.slug, resetConversation)
onBeforeUnmount(() => { generation++; request?.abort(); modeRequest?.abort(); clearTimeout(greetingTimer) })
</script>

<style scoped>
.support-widget { --sp-paper: #fff; --sp-ink: #202720; --sp-muted: #667366; --sp-soft: #eff4ed; --sp-line: #dfe6da; --sp-accent: #294b36; --sp-on-accent: #fff; --sp-shadow: rgba(35,59,41,.13); --sp-bottom: 18px; min-width: 132px; position: fixed; z-index: 45; bottom: var(--sp-bottom); right: 24px; width: calc(132px * var(--pet-scale, 1)); height: calc(144px * var(--pet-scale, 1) + 42px); color: var(--sp-ink); font: 14px/1.55 Inter,"PingFang SC","Microsoft YaHei",sans-serif; }
/* Stay reachable over the sticky navigation; announcements and payment dialogs remain above. */
.support-widget[data-moved="true"], .support-widget[data-open="true"] { z-index: 55; }
:global(.dark .support-widget) { --sp-paper: #202923; --sp-ink: #f2f5f0; --sp-muted: #bdc9be; --sp-soft: #2d3a31; --sp-line: #425247; --sp-accent: #c4dbc5; --sp-on-accent: #193021; --sp-shadow: rgba(0,0,0,.32); }
.support-widget *, .support-widget *::after { box-sizing: border-box; }
.support-widget button { cursor: pointer; font: inherit; }
.support-widget button:disabled { cursor: default; opacity: .5; }
.support-widget button:focus-visible, .support-widget a:focus-visible, .support-widget input:focus-visible { outline: 2px solid var(--sp-accent); outline-offset: 3px; }
.support-launcher { position: absolute; right: 8px; bottom: 40px; width: calc(115.2px * var(--pet-scale, 1)); height: calc(144px * var(--pet-scale, 1)); border: 0; padding: 0; background: transparent; border-radius: 16px; filter: drop-shadow(0 4px 3px var(--sp-shadow)); }
.support-widget .support-launcher { cursor: grab; touch-action: manipulation; user-select: none; -webkit-user-select: none; }
.support-widget[data-dragging="true"] .support-launcher { cursor: grabbing; }
.support-widget[data-panel-overlap="true"] .support-launcher, .support-widget[data-panel-overlap="true"] .support-ground, .support-widget[data-panel-overlap="true"] .support-name { visibility: hidden; }
.support-ground { position: absolute; width: 73px; height: 7px; right: 29px; bottom: 40px; background: var(--sp-shadow); border-radius: 50%; filter: blur(4px); pointer-events: none; }
.support-name { position: absolute; bottom: 0; right: 8px; padding: 4px 11px; background: var(--sp-paper); border: 1px solid var(--sp-line); border-radius: 20px; font-size: 12px; white-space: nowrap; pointer-events: none; }
.support-name::before { content: ''; display: inline-block; width: 5px; height: 5px; background: var(--sp-accent); border-radius: 50%; margin-right: 5px; vertical-align: 2px; }
.support-speech { position: absolute; right: calc(132px * var(--pet-scale, 1) + 8px); bottom: 76px; width: max-content; max-width: 176px; padding: 10px 13px; background: var(--sp-paper); border: 1px solid var(--sp-line); border-radius: 18px; box-shadow: 0 4px 15px var(--sp-shadow); font-size: 12px; text-align: center; pointer-events: none; }
.support-quote-label { display: block; margin-top: 5px; color: var(--sp-muted); font-size: 10px; line-height: 1.4; }
.support-speech::after { content: ''; position: absolute; width: 10px; height: 10px; right: -6px; bottom: 14px; transform: rotate(-45deg); background: var(--sp-paper); border-right: 1px solid var(--sp-line); border-bottom: 1px solid var(--sp-line); }
.support-speech[data-side="right"]::after { right: auto; left: -6px; transform: rotate(135deg); }
.support-window { position: absolute; right: 0; bottom: calc(144px * var(--pet-scale, 1) + 55px); width: 390px; max-width: calc(100vw - 32px); height: 500px; max-height: calc(100dvh - var(--sp-bottom) - 144px * var(--pet-scale, 1) - 67px); display: flex; flex-direction: column; border: 1px solid var(--sp-line); border-radius: 24px; background: var(--sp-paper); box-shadow: 0 14px 42px var(--sp-shadow); }
.support-header { flex-shrink: 0; display: flex; align-items: center; gap: 10px; padding: 14px 16px; border-bottom: 1px solid var(--sp-line); }
.support-avatar { width: 34px; height: 40px; object-fit: contain; }
.support-identity { flex: 1; min-width: 0; }
.support-identity h2 { font: 600 16px/1.4 Inter,"PingFang SC","Microsoft YaHei",sans-serif; margin: 0; }
.support-identity span { display: block; font-size: 11px; color: var(--sp-muted); }
.support-icon { padding: 10px; border: 0; border-radius: 10px; color: var(--sp-muted); background: transparent; display: inline-flex; align-items: center; justify-content: center; }
.support-icon:hover { background: var(--sp-soft); }
.support-body { flex: 1; min-height: 0; padding: 23px 20px 14px; overflow-y: auto; overscroll-behavior: contain; scrollbar-width: thin; }
.support-welcome h3 { margin: 0 0 9px; font: 600 23px/1.3 Inter,"PingFang SC","Microsoft YaHei",sans-serif; }
.support-welcome p { font-size: 13px; color: var(--sp-muted); margin: 0 0 20px; }
.support-topics { display: grid; grid-template-columns: 1fr 1fr; gap: 9px; margin-bottom: 12px; }
.support-topics button { padding: 13px 11px; min-height: 64px; border: 1px solid var(--sp-line); border-radius: 13px; text-align: left; background: var(--sp-paper); color: var(--sp-ink); font-size: 13px; }
.support-topics button:hover:not(:disabled) { background: var(--sp-soft); border-color: var(--sp-accent); }
.support-topics small { display: block; font-size: 11px; color: var(--sp-muted); }
.support-message { margin: 0 0 14px; padding: 11px 13px; font-size: 13px; white-space: pre-line; overflow-wrap: anywhere; }
.support-message p { margin: 0; }
.support-message-links { display: flex; flex-wrap: wrap; gap: 8px 14px; margin-top: 12px; white-space: normal; }
.support-message-links a { color: var(--sp-accent); text-decoration: underline; text-underline-offset: 3px; }
.support-message-user { margin-left: 36px; background: var(--sp-accent); color: var(--sp-on-accent); border-radius: 14px 14px 4px 14px; }
.support-message-assistant { margin-right: 12px; padding: 14px; background: var(--sp-soft); border-radius: 4px 14px 14px; }
.support-more { color: var(--sp-accent); border: 0; background: transparent; padding: 6px 0; font-size: 12px !important; }
.support-composer { flex-shrink: 0; margin: 0 16px; min-height: 49px; display: flex; align-items: center; gap: 8px; padding: 7px 8px 7px 13px; border: 1px solid var(--sp-line); border-radius: 15px; }
.support-composer input { flex: 1; width: 0; min-width: 0; padding: 3px 0; background: transparent; border: 0; color: var(--sp-ink); font: inherit; font-size: 13px; }
.support-composer input::placeholder { color: var(--sp-muted); }
.support-composer:focus-within { outline: 2px solid var(--sp-accent); outline-offset: 2px; }
.support-composer input:focus-visible { outline: none; }
.support-composer button { flex-shrink: 0; padding: 8px 10px; background: var(--sp-accent); color: var(--sp-on-accent); border: 0; border-radius: 10px; font-size: 12px; }
.support-footer { flex-shrink: 0; display: flex; justify-content: space-between; align-items: center; gap: 10px; margin: 10px 16px 14px; font-size: 11px; color: var(--sp-muted); }
.support-footer span { max-width: 230px; }
.support-footer button { border: 0; padding: 5px 0; background: transparent; color: var(--sp-ink); white-space: nowrap; font-size: 11px; }
.support-contacts { position: absolute; bottom: 95px; left: 16px; right: 16px; max-height: calc(100% - 105px); overflow-y: auto; padding: 12px 16px; border: 1px solid var(--sp-line); border-radius: 15px; background: var(--sp-paper); box-shadow: 0 5px 20px var(--sp-shadow); }
.support-contact-heading, .support-contact-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.support-contact-row { margin: 9px 0; font-size: 11px; color: var(--sp-muted); }
.support-contact-row strong { display: block; font-size: 14px; font-weight: 500; color: var(--sp-ink); user-select: all; }
.support-contact-row button { border: 1px solid var(--sp-line); border-radius: 8px; background: var(--sp-soft); color: var(--sp-ink); padding: 7px 9px; font-size: 12px; }
.support-telegram { display: flex; gap: 6px; align-items: center; color: var(--sp-accent); font-size: 12px; padding: 8px 0; }
.support-copy-status { font-size: 11px; color: var(--sp-muted); }
@media (max-width: 1279px) { .support-widget { --sp-bottom: calc(3.5rem + env(safe-area-inset-bottom, 0px) + 12px); } }
@media (max-width: 480px) {
  .support-widget { right: 12px; width: calc(110px * var(--pet-scale, 1)); height: calc(124px * var(--pet-scale, 1) + 42px); }
  .support-launcher { width: calc(99.2px * var(--pet-scale, 1)); height: calc(124px * var(--pet-scale, 1)); right: 5px; }
  .support-name { right: 0; }
  .support-ground { right: 18px; }
  .support-speech { right: calc(max(132px, 110px * var(--pet-scale, 1)) + 8px); bottom: 68px; max-width: calc(100vw - 158px); }
  .support-window { position: fixed; left: 12px; right: 12px; width: auto; max-width: none; height: 494px; bottom: calc(var(--sp-bottom) + 124px * var(--pet-scale, 1) + 55px); max-height: calc(100dvh - var(--sp-bottom) - 124px * var(--pet-scale, 1) - 67px); border-radius: 20px; }
  .support-header { padding: 12px; }
  .support-body { padding: 18px 15px 12px; }
  .support-footer span { max-width: 175px; }
  .support-composer input { font-size: 16px; }
}
@media (max-width: 360px) { .support-topics small { display: none; } .support-topics button { min-height: 57px; } .support-welcome h3 { font-size: 21px; } }
@media (max-height: 570px) {
  .support-widget[data-open="true"] .support-window { position: fixed; top: 8px; bottom: calc(var(--sp-bottom) + 8px); right: 12px; width: min(390px, calc(100vw - 24px)); height: auto; max-height: none; z-index: 1; }
  .support-widget[data-open="true"] .support-launcher, .support-widget[data-open="true"] .support-name, .support-widget[data-open="true"] .support-ground, .support-widget[data-open="true"] .support-speech { visibility: hidden; }
}
@media (pointer: coarse) { .support-icon, .support-composer button, .support-footer button, .support-contact-row button, .support-more, .support-telegram { min-height: 44px; } }
@media (max-height: 570px) and (max-width: 480px) { .support-widget[data-open="true"] .support-window { left: 12px; width: auto; } }
.support-pet-tools { position: absolute; bottom: 0; right: 0; display: flex; align-items: center; gap: 3px; padding: 3px; border-radius: 22px; background: var(--sp-paper); border: 1px solid var(--sp-line); box-shadow: 0 3px 12px var(--sp-shadow); white-space: nowrap; }
.support-chat-toggle, .support-pet-options { display: flex; align-items: center; justify-content: center; gap: 5px; color: var(--sp-ink); border: 0; background: transparent; border-radius: 18px; min-height: 30px; padding: 5px 7px; font-size: 11px !important; }
.support-chat-toggle span { color: var(--sp-muted); }
.support-chat-toggle:hover, .support-pet-options:hover { background: var(--sp-soft); }
.support-pet-menu { position: fixed; width: 244px; max-width: calc(100vw - 24px); max-height: calc(100dvh - 24px); overflow: auto; padding: 12px; border: 1px solid var(--sp-line); background: var(--sp-paper); border-radius: 20px; box-shadow: 0 12px 36px var(--sp-shadow); z-index: 2; }
.support-pet-menu header { display: flex; align-items: center; justify-content: space-between; }
.support-pet-menu strong { font-size: 14px; }.support-pet-menu p { font-size: 11px; color: var(--sp-muted); margin: 3px 0 12px; }
.support-pet-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
.support-pet-sizes { display: flex; gap: 6px; }
.support-pet-actions button, .support-pet-sizes button { flex: 1; border: 1px solid var(--sp-line); border-radius: 10px; background: var(--sp-soft); color: var(--sp-ink); font-size: 12px; padding: 8px 4px; }
.support-pet-menu fieldset { border: 0; padding: 0; margin: 14px 0 10px; }.support-pet-menu legend { font-size: 11px; color: var(--sp-muted); margin-bottom: 6px; }
.support-pet-sizes button[aria-pressed="true"] { background: var(--sp-accent); color: var(--sp-on-accent); border-color: var(--sp-accent); }
.support-pet-setting { display: flex; align-items: center; gap: 8px; width: 100%; border: 0; padding: 9px 5px; border-radius: 9px; color: var(--sp-ink); background: transparent; text-align: left; font-size: 12px !important; }.support-pet-setting:hover { background: var(--sp-soft); }
.support-sleep { position: absolute; right: 5px; top: 18px; font-size: 12px; letter-spacing: 3px; color: var(--sp-muted); pointer-events: none; }
.support-widget[data-panel-overlap="true"] .support-pet-tools, .support-widget[data-panel-overlap="true"] .support-sleep { visibility: hidden; }
@media (max-height: 570px) { .support-widget[data-open="true"] .support-pet-tools, .support-widget[data-open="true"] .support-sleep { visibility: hidden; } }
@media (pointer: coarse) { .support-pet-tools button, .support-pet-menu button { min-height: 44px; }.support-launcher { bottom: 53px; }.support-widget { height: calc(144px * var(--pet-scale, 1) + 55px); } }
</style>
