<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { ArrowUpRight, Ghost, MessagesSquare, Send, X } from 'lucide-vue-next'
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import { resolveSupportContacts } from '../../../../shared/supportContacts'
import { useAppStore } from '../../stores/app'
import { useProductContact } from '../../composables/useProductContact'
import { contactFirstCopy } from '../../utils/contactFirstProducts'
import { copyText } from '../../utils/clipboard'

const { locale } = useI18n()
const route = useRoute()
const app = useAppStore()
const labels = computed(() => contactFirstCopy(locale.value))
const contacts = computed(() => resolveSupportContacts(app.config?.contact, app.isResellerTenant))
const { contactOpen, closeContact, restoreFocus } = useProductContact()
const copied = ref('')
const failed = ref(false)
watch(contactOpen, () => { copied.value = ''; failed.value = false })
watch(() => route.fullPath, closeContact)
const rows = computed(() => [
  { key: 'wechat', label: labels.value.wechat, value: contacts.value.wechat, icon: MessagesSquare, url: '' },
  { key: 'qq', label: 'QQ', value: contacts.value.qq, icon: Ghost, url: '' },
  { key: 'telegram', label: 'Telegram', value: contacts.value.telegram, icon: Send, url: contacts.value.telegramUrl },
].filter(row => row.value))
async function copyContact(key: string, value: string) {
  failed.value = false
  try {
    await copyText(value)
    copied.value = key
  } catch {
    copied.value = ''
    failed.value = true
  }
}
</script>

<template>
  <DialogRoot :open="contactOpen" @update:open="value => { if (!value) closeContact() }">
    <DialogPortal>
      <DialogOverlay class="product-contact-overlay" />
      <DialogContent class="product-contact-dialog" @close-auto-focus="restoreFocus">
        <DialogClose as-child>
          <button type="button" class="contact-dismiss" :aria-label="labels.close"><X :size="24" /></button>
        </DialogClose>
        <DialogTitle class="contact-title">{{ labels.title }}</DialogTitle>
        <DialogDescription class="contact-description">{{ labels.description }}</DialogDescription>
        <div class="contact-list">
          <div v-for="row in rows" :key="row.key" class="contact-row">
            <component :is="row.icon" class="contact-icon" :size="30" :stroke-width="1.8" aria-hidden="true" />
            <div class="contact-info">
              <span class="contact-label">{{ row.label }}</span>
              <span class="contact-value">{{ row.value }}</span>
            </div>
            <a v-if="row.url" class="contact-action" :href="row.url" target="_blank" rel="noopener noreferrer" :aria-label="`${labels.open} ${row.label}`">
              {{ labels.open }}<ArrowUpRight :size="16" aria-hidden="true" />
            </a>
            <button v-else type="button" class="contact-action" :aria-label="`${labels.copy} ${row.label}`" @click="copyContact(row.key, row.value)">
              {{ copied === row.key ? labels.copied : labels.copy }}
            </button>
          </div>
        </div>
        <p v-if="!rows.length" class="contact-hint">{{ labels.unavailable }}</p>
        <p v-else class="contact-hint">{{ labels.hint }}</p>
        <p class="contact-feedback" role="status" aria-live="polite">
          {{ failed ? labels.failed : copied ? `${rows.find(row => row.key === copied)?.label} ${labels.copied}` : '' }}
        </p>
        <DialogClose as-child><button type="button" class="contact-close">{{ labels.close }}</button></DialogClose>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<style scoped>
.product-contact-overlay { position: fixed; inset: 0; z-index: 150; background: rgb(13 37 35 / 43%); backdrop-filter: blur(4px); }
.product-contact-dialog {
  position: fixed; left: 50%; top: 50%; z-index: 151;
  transform: translate(-50%, -50%); width: min(552px, calc(100% - 32px));
  max-height: calc(100dvh - 32px); overflow-y: auto; padding: 32px 36px;
  border: 1px solid var(--ui-border); border-radius: 20px;
  background: var(--ui-bg-elevated); color: var(--ui-text-primary);
  box-shadow: 0 18px 60px rgb(10 31 28 / 20%); outline: none;
}
.contact-dismiss { position: absolute; right: 22px; top: 22px; display: grid; place-items: center; width: 40px; height: 40px; border-radius: 10px; cursor: pointer; }
.contact-dismiss:hover { background: var(--ui-bg-soft); }
.contact-title { margin: 0; padding-right: 36px; font-size: 28px; line-height: 1.4; font-weight: 750; }
.contact-description { margin: 18px 0 20px; color: var(--ui-text-secondary); font-size: 16px; line-height: 1.7; }
.contact-list { border-top: 1px solid var(--ui-border); }
.contact-row { display: grid; grid-template-columns: 32px minmax(0, 1fr) auto; align-items: center; gap: 14px; min-height: 84px; padding: 14px 0; border-bottom: 1px solid var(--ui-border); }
.contact-info { display: grid; grid-template-columns: 84px minmax(0, 1fr); align-items: center; gap: 10px; font-size: 16px; line-height: 1.5; }
.contact-icon { color: var(--ui-text-primary); }
.contact-value { user-select: all; overflow-wrap: anywhere; }
.contact-action { display: inline-flex; align-items: center; justify-content: center; gap: 4px; min-width: 74px; min-height: 44px; padding: 8px 12px; border: 1px solid var(--ui-border-strong); border-radius: 16px; font-size: 15px; line-height: 1.4; white-space: nowrap; cursor: pointer; }
.contact-action:hover { background: var(--ui-bg-soft); }
.contact-hint { margin: 14px 0 0; color: var(--ui-text-muted); font-size: 14px; line-height: 1.6; }
.contact-feedback { min-height: 20px; margin: 4px 0; font-size: 13px; line-height: 1.5; color: var(--ui-text-secondary); }
.contact-close { width: 100%; min-height: 52px; margin-top: 4px; padding: 12px; border: 1px solid var(--ui-border); border-radius: 15px; background: var(--ui-bg-soft); font-size: 16px; cursor: pointer; }
.contact-close:hover { background: var(--ui-bg-muted); }
.product-contact-dialog :is(button, a):focus-visible { outline: 2px solid var(--ui-accent); outline-offset: 3px; }
@media (max-width: 560px) {
  .product-contact-dialog { padding: 26px 22px; }
  .contact-title { font-size: 24px; }
  .contact-description { font-size: 15px; }
  .contact-info { grid-template-columns: minmax(0, 1fr); gap: 2px; font-size: 15px; }
  .contact-label { font-size: 13px; color: var(--ui-text-muted); }
  .contact-row { gap: 10px; grid-template-columns: 26px minmax(0, 1fr) auto; }
  .contact-icon { width: 25px; }
  .contact-action { min-width: 60px; padding: 8px 9px; font-size: 14px; }
}
</style>
