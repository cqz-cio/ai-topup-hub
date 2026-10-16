<template>
  <aside ref="widget" class="support-widget" :style="widgetStyle" :data-open="open" :data-moved="!!widgetStyle" :data-dragging="dragging" :data-panel-overlap="panelOverlap" @keydown.esc.stop.prevent="escape">
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
        <button v-if="messages.length && !showTopics && !busy" class="support-more" type="button" @click="showTopics = true">{{ copy.newQuestion }}</button>
      </div>
      <form class="support-composer" @submit.prevent="ask(input)">
        <input ref="inputElement" v-model="input" type="text" maxlength="500" autocomplete="off" :placeholder="copy.placeholder" :aria-label="copy.placeholder" :disabled="busy" @keydown.enter="preventComposition" />
        <button type="submit" :disabled="busy || !input.trim()">{{ copy.send }}</button>
      </form>
      <footer class="support-footer"><span>{{ copy.footer }}</span><button type="button" :aria-expanded="contactsOpen" @click="contactsOpen = !contactsOpen">{{ copy.human }}</button></footer>
      <section v-if="contactsOpen" class="support-contacts" :aria-label="copy.contactTitle">
        <div class="support-contact-heading"><strong>{{ copy.contactTitle }}</strong><button type="button" class="support-icon" :aria-label="copy.closeContacts" @click="contactsOpen = false"><X :size="16" /></button></div>
        <div class="support-contact-row"><span>{{ copy.wechat }}<strong>{{ supportContacts.wechat }}</strong></span><button type="button" @click="copyContact(supportContacts.wechat)">{{ copy.copy }}</button></div>
        <div class="support-contact-row"><span>{{ copy.qq }}<strong>{{ supportContacts.qq }}</strong></span><button type="button" @click="copyContact(supportContacts.qq)">{{ copy.copy }}</button></div>
        <a :href="supportContacts.telegramUrl" target="_blank" rel="noopener noreferrer" class="support-telegram">{{ copy.telegram }} · {{ supportContacts.telegram }}<ExternalLink :size="14" /></a>
        <span v-if="copyStatus" class="support-copy-status" role="status">{{ copyStatus }}</span>
      </section>
    </section>
    <div v-if="open || greetingVisible" ref="speechElement" class="support-speech" :style="speechStyle" :data-side="speechSide" aria-hidden="true">{{ speech }}</div>
    <div class="support-ground" aria-hidden="true" />
    <button ref="launcher" class="support-launcher" type="button" :aria-label="open ? copy.close : copy.open" :aria-expanded="open" aria-controls="pi-support-window" @click="activate" @pointerdown="pointerDown" @pointermove="pointerMove" @pointerup="finish" @pointercancel="finish" @lostpointercapture="finish" @dragstart.prevent>
      <SupportPet :action="action" :sequence="sequence" @phase="phase = $event" />
    </button>
    <span class="support-name">{{ copy.name }}</span>
  </aside>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { ExternalLink, Minus, X } from 'lucide-vue-next'
import { useAppStore } from '../../stores/app'
import SupportPet from './SupportPet.vue'
import avatar from './patrick-avatar.webp'
import type { PetAction } from './petMotion'
import { getSupportCopy, supportContacts } from './supportDemo'
import { useSupportDrag } from './useSupportDrag'
import type { SupportLink } from './supportConversation'
import { askSupport, agentCopy, agentError } from './supportApi'

const app = useAppStore(), copy = computed(() => ({ ...getSupportCopy(app.locale), ...agentCopy(app.locale) }))
const route = useRoute()
const open = ref(false), busy = ref(false), input = ref(''), contactsOpen = ref(false), showTopics = ref(false), copyStatus = ref('')
const launcher = ref<HTMLButtonElement>(), closeButton = ref<HTMLButtonElement>(), inputElement = ref<HTMLInputElement>(), body = ref<HTMLElement>()
const widget = ref<HTMLElement>(), panel = ref<HTMLElement>(), speechElement = ref<HTMLElement>()
const { widgetStyle, panelStyle, speechStyle, speechSide, panelOverlap, dragging, pointerDown, pointerMove, finish, allowClick } = useSupportDrag(widget, launcher, panel, speechElement)
const action = ref<PetAction>('wave'), phase = ref<PetAction>('wave'), sequence = ref(0), greetingVisible = ref(true)
const messages = ref<{ id: number; role: 'user' | 'assistant'; text: string; links?: SupportLink[] }[]>([])
let greetingTimer: ReturnType<typeof setTimeout> | undefined, messageId = 0
let conversationId = crypto.randomUUID()
let request: AbortController | undefined, generation = 0
const speech = computed(() => busy.value ? copy.value.think : phase.value === 'reply' ? copy.value.reply : phase.value === 'wave' ? copy.value.greeting : copy.value.idle)
function animate(next: PetAction) { action.value = next; sequence.value++ }
function close() { open.value = false; contactsOpen.value = false; nextTick(() => launcher.value?.focus({ preventScroll: true })) }
function escape() { if (contactsOpen.value) contactsOpen.value = false; else close() }
function activate(event: MouseEvent) { if (allowClick(event)) void toggle() }
async function toggle() {
  if (open.value) { close(); return }
  open.value = true
  if (!busy.value) animate('wave')
  await nextTick()
  if (matchMedia('(pointer: fine)').matches) inputElement.value?.focus({ preventScroll: true })
  else closeButton.value?.focus({ preventScroll: true })
  scrollMessages()
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
  request?.abort(); request = new AbortController()
  messages.value.push({ id: ++messageId, role: 'user', text: value }); input.value = ''; busy.value = true; showTopics.value = false; contactsOpen.value = false
  animate('think'); scrollMessages()
  try {
    const response = await askSupport(value, locale, conversationId, typeof route.params.slug === 'string' ? route.params.slug : '', request.signal)
    if (turn !== generation) return
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
watch(() => app.locale, () => { generation++; request?.abort(); busy.value = false; conversationId = crypto.randomUUID(); messages.value = []; animate('idle') })
onBeforeUnmount(() => { generation++; request?.abort(); clearTimeout(greetingTimer) })
</script>

<style scoped>
.support-widget { --sp-paper: #fff; --sp-ink: #202720; --sp-muted: #667366; --sp-soft: #eff4ed; --sp-line: #dfe6da; --sp-accent: #294b36; --sp-on-accent: #fff; --sp-shadow: rgba(35,59,41,.13); --sp-bottom: 18px; position: fixed; z-index: 45; bottom: var(--sp-bottom); right: 24px; width: 132px; height: 167px; color: var(--sp-ink); font: 14px/1.55 Inter,"PingFang SC","Microsoft YaHei",sans-serif; }
/* Stay reachable over the sticky navigation; announcements and payment dialogs remain above. */
.support-widget[data-moved="true"], .support-widget[data-open="true"] { z-index: 55; }
:global(.dark .support-widget) { --sp-paper: #202923; --sp-ink: #f2f5f0; --sp-muted: #bdc9be; --sp-soft: #2d3a31; --sp-line: #425247; --sp-accent: #c4dbc5; --sp-on-accent: #193021; --sp-shadow: rgba(0,0,0,.32); }
.support-widget *, .support-widget *::after { box-sizing: border-box; }
.support-widget button { cursor: pointer; font: inherit; }
.support-widget button:disabled { cursor: default; opacity: .5; }
.support-widget button:focus-visible, .support-widget a:focus-visible, .support-widget input:focus-visible { outline: 2px solid var(--sp-accent); outline-offset: 3px; }
.support-launcher { position: absolute; right: 8px; bottom: 23px; width: 115.2px; height: 144px; border: 0; padding: 0; background: transparent; border-radius: 16px; filter: drop-shadow(0 4px 3px var(--sp-shadow)); }
.support-widget .support-launcher { cursor: grab; touch-action: manipulation; user-select: none; -webkit-user-select: none; }
.support-widget[data-dragging="true"] .support-launcher { cursor: grabbing; }
.support-widget[data-panel-overlap="true"] .support-launcher, .support-widget[data-panel-overlap="true"] .support-ground, .support-widget[data-panel-overlap="true"] .support-name { visibility: hidden; }
.support-ground { position: absolute; width: 73px; height: 7px; right: 29px; bottom: 25px; background: var(--sp-shadow); border-radius: 50%; filter: blur(4px); pointer-events: none; }
.support-name { position: absolute; bottom: 0; right: 8px; padding: 4px 11px; background: var(--sp-paper); border: 1px solid var(--sp-line); border-radius: 20px; font-size: 12px; white-space: nowrap; pointer-events: none; }
.support-name::before { content: ''; display: inline-block; width: 5px; height: 5px; background: var(--sp-accent); border-radius: 50%; margin-right: 5px; vertical-align: 2px; }
.support-speech { position: absolute; right: 140px; bottom: 66px; width: max-content; max-width: 176px; padding: 10px 13px; background: var(--sp-paper); border: 1px solid var(--sp-line); border-radius: 18px; box-shadow: 0 4px 15px var(--sp-shadow); font-size: 12px; text-align: center; pointer-events: none; }
.support-speech::after { content: ''; position: absolute; width: 10px; height: 10px; right: -6px; bottom: 14px; transform: rotate(-45deg); background: var(--sp-paper); border-right: 1px solid var(--sp-line); border-bottom: 1px solid var(--sp-line); }
.support-speech[data-side="right"]::after { right: auto; left: -6px; transform: rotate(135deg); }
.support-window { position: absolute; right: 0; bottom: 180px; width: 390px; max-width: calc(100vw - 32px); height: 500px; max-height: calc(100dvh - var(--sp-bottom) - 192px); display: flex; flex-direction: column; border: 1px solid var(--sp-line); border-radius: 24px; background: var(--sp-paper); box-shadow: 0 14px 42px var(--sp-shadow); }
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
  .support-widget { right: 12px; width: 110px; height: 147px; }
  .support-launcher { width: 99.2px; height: 124px; right: 5px; }
  .support-name { right: 0; }
  .support-ground { right: 18px; }
  .support-speech { right: 118px; bottom: 58px; max-width: calc(100vw - 158px); }
  .support-window { position: fixed; left: 12px; right: 12px; width: auto; max-width: none; height: 494px; bottom: calc(var(--sp-bottom) + 160px); max-height: calc(100dvh - var(--sp-bottom) - 172px); border-radius: 20px; }
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
</style>
