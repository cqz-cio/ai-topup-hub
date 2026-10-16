<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import AdminBrand from '@/components/AdminBrand.vue'
import { ADMIN_BRAND_NAME, STOREFRONT_URL, SUPPORT_WECHAT, SUPPORT_QQ, SUPPORT_TELEGRAM, SUPPORT_TELEGRAM_URL } from '@/utils/branding'

defineProps<{ open: boolean; version?: string }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()
const { t } = useI18n()
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="w-[calc(100%_-_2rem)] max-w-[640px] max-h-[90dvh] overflow-y-auto rounded-xl p-7">
      <DialogHeader class="text-left">
        <DialogTitle><AdminBrand :title="t('admin.systemInfo.title')" /></DialogTitle>
        <DialogDescription class="mt-4 text-sm leading-relaxed">{{ t('admin.systemInfo.description') }}</DialogDescription>
      </DialogHeader>
      <dl class="divide-y divide-border text-sm">
        <div class="flex justify-between gap-5 py-3.5"><dt class="text-muted-foreground">{{ t('admin.systemInfo.siteName') }}</dt><dd>{{ ADMIN_BRAND_NAME }}</dd></div>
        <div class="flex justify-between gap-5 py-3.5"><dt class="text-muted-foreground">{{ t('admin.systemInfo.storefront') }}</dt><dd><a :href="STOREFRONT_URL" target="_blank" rel="noopener noreferrer">www.qzweb.cloud ↗</a></dd></div>
        <div class="flex justify-between gap-5 py-3.5"><dt class="text-muted-foreground">{{ t('admin.systemInfo.wechat') }}</dt><dd>{{ SUPPORT_WECHAT }}</dd></div>
        <div class="flex justify-between gap-5 py-3.5"><dt class="text-muted-foreground">QQ</dt><dd>{{ SUPPORT_QQ }}</dd></div>
        <div class="flex justify-between gap-5 py-3.5"><dt class="text-muted-foreground">Telegram</dt><dd><a :href="SUPPORT_TELEGRAM_URL" target="_blank" rel="noopener noreferrer">{{ SUPPORT_TELEGRAM }} ↗</a></dd></div>
      </dl>
      <p class="text-xs leading-relaxed text-muted-foreground">{{ t('admin.systemInfo.orderHint') }}</p>
      <p v-if="version" class="text-xs text-muted-foreground">{{ t('admin.systemInfo.programVersion', { version }) }}</p>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
dt { flex-shrink: 0; }
dd { text-align: right; min-width: 0; overflow-wrap: anywhere; }
a { color: #178a5a; }
a:hover { text-decoration: underline; }
:global(.dark) a { color: #70d5a7; }
</style>
