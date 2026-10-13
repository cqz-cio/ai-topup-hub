<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ADMIN_BRAND_LOGO, ADMIN_BRAND_NAME } from '@/utils/branding'

defineProps<{ compact?: boolean; title?: string }>()
const { t } = useI18n()
</script>

<template>
  <span class="admin-brand" :class="{ 'admin-brand-compact': compact }" :title="ADMIN_BRAND_NAME">
    <span class="admin-brand-icon">
      <img :src="ADMIN_BRAND_LOGO" :alt="compact ? ADMIN_BRAND_NAME : ''" />
    </span>
    <span v-if="!compact" class="min-w-0">
      <span class="admin-brand-name">{{ title || ADMIN_BRAND_NAME }}</span>
      <span class="admin-brand-subtitle">{{ t('admin.layout.controlRoom') }}</span>
    </span>
  </span>
</template>

<style scoped>
.admin-brand { display: flex; align-items: center; gap: 12px; min-width: 0; }
.admin-brand-icon { display: grid; place-items: center; width: 42px; height: 42px; flex-shrink: 0; border: 1px solid #d5ebde; background: #f0f9f3; border-radius: 11px; }
.admin-brand-icon img { display: block; width: 36px; height: 36px; object-fit: contain; }
.admin-brand-name { display: block; font-size: 20px; line-height: 28px; font-weight: 650; letter-spacing: -.4px; }
.admin-brand-subtitle { display: block; margin-top: 2px; font-size: 12px; line-height: 18px; font-weight: 400; color: hsl(var(--muted-foreground)); letter-spacing: normal; }
.admin-brand-compact { justify-content: center; }
.admin-brand-compact .admin-brand-icon { width: 36px; height: 36px; border-radius: 9px; }
.admin-brand-compact img { width: 30px; height: 30px; }
@media (max-width: 767px) { .admin-brand-icon { width: 38px; height: 38px; } }
</style>
