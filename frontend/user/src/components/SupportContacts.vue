<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Copy, ArrowUpRight } from 'lucide-vue-next'
import { resolveSupportContacts } from '../../../shared/supportContacts'
import { useAppStore } from '../stores/app'
import { copyText } from '../utils/clipboard'
import { toast } from '../composables/useToast'

withDefaults(defineProps<{ includeWechat?: boolean }>(), { includeWechat: false })
const appStore = useAppStore()
const { locale, t } = useI18n()
const contacts = computed(() => resolveSupportContacts(appStore.config?.contact, appStore.isResellerTenant))
const labels = computed(() => locale.value === 'en-US'
  ? { copy: 'Copy {name}', copied: '{name} copied', failed: 'Copy failed. Please copy manually.' }
  : locale.value === 'zh-TW'
    ? { copy: '複製{name}', copied: '已複製{name}', failed: '複製失敗，請手動複製。' }
    : { copy: '复制{name}', copied: '已复制{name}', failed: '复制失败，请手动复制。' })
const copy = async (name: string, value: string) => {
  try { await copyText(value); toast.success(labels.value.copied.replace('{name}', name)) }
  catch { toast.error(labels.value.failed) }
}
</script>

<template>
  <div v-if="contacts.qq || contacts.telegram || (includeWechat && contacts.wechat)" class="support-contacts" data-support-contacts>
    <button v-if="includeWechat && contacts.wechat" type="button" :aria-label="labels.copy.replace('{name}', t('shopUi.wechat'))" @click="copy(t('shopUi.wechat'), contacts.wechat)">{{ t('shopUi.wechat') }}：{{ contacts.wechat }}<Copy :size="13" aria-hidden="true" /></button>
    <button v-if="contacts.qq" type="button" :aria-label="labels.copy.replace('{name}', 'QQ')" @click="copy('QQ', contacts.qq)">QQ：{{ contacts.qq }}<Copy :size="13" aria-hidden="true" /></button>
    <a v-if="contacts.telegram" :href="contacts.telegramUrl" target="_blank" rel="noopener noreferrer">Telegram：{{ contacts.telegram }}<ArrowUpRight :size="13" aria-hidden="true" /></a>
  </div>
</template>

<style scoped>
.support-contacts { display: flex; flex-wrap: wrap; gap: 8px 16px; margin-top: 10px; min-width: 0; font-size: 13px; line-height: 1.7; }
.support-contacts button, .support-contacts a { display: inline-flex; align-items: center; gap: 5px; max-width: 100%; color: inherit; overflow-wrap: anywhere; text-align: left; cursor: pointer; }
.support-contacts svg { flex-shrink: 0; }
.support-contacts button:hover, .support-contacts a:hover { text-decoration: underline; }
.support-contacts button:focus-visible, .support-contacts a:focus-visible { outline: 2px solid currentColor; outline-offset: 3px; border-radius: 2px; }
</style>
