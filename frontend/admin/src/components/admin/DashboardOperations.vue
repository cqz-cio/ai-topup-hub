<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { useAdminAuthStore } from '@/stores/auth'
import { PURCHASE_NOTICE_URL, SUPPORT_WECHAT } from '@/utils/branding'

const { t } = useI18n()
const authStore = useAdminAuthStore()
</script>

<template>
  <section class="operations" :aria-label="t('admin.operations.title')">
    <div class="operation">
      <h2>{{ t('admin.operations.noticeTitle') }}</h2>
      <p>{{ t('admin.operations.noticeDescription') }}</p>
      <a :href="PURCHASE_NOTICE_URL" target="_blank" rel="noopener noreferrer">
        {{ t('admin.operations.noticeAction') }} <span aria-hidden="true">↗</span>
      </a>
    </div>
    <div class="operation">
      <h2>{{ t('admin.operations.pricingTitle') }}</h2>
      <p>{{ t('admin.operations.pricingDescription') }}</p>
      <RouterLink v-if="authStore.hasPermission('GET:/admin/member-levels')" to="/member-levels">
        {{ t('admin.operations.pricingAction') }} <span aria-hidden="true">→</span>
      </RouterLink>
    </div>
    <div class="operation">
      <h2>{{ t('admin.operations.supportTitle') }}</h2>
      <p>{{ t('admin.operations.supportDescription', { wechat: SUPPORT_WECHAT }) }}</p>
      <RouterLink v-if="authStore.hasPermission('GET:/admin/orders')" to="/orders">
        {{ t('admin.operations.supportAction') }} <span aria-hidden="true">→</span>
      </RouterLink>
    </div>
  </section>
</template>

<style scoped>
.operations { display: grid; grid-template-columns: 1fr 1fr 1fr; border: 1px solid hsl(var(--border)); border-radius: 12px; background: hsl(var(--card)); box-shadow: 0 1px 2px #00000006; }
.operation { padding: 17px 22px; min-width: 0; }
.operation + .operation { border-left: 1px solid hsl(var(--border)); }
.operation h2 { margin: 0 0 5px; font-size: 14px; line-height: 20px; font-weight: 600; }
.operation p { margin: 0; font-size: 12px; color: hsl(var(--muted-foreground)); line-height: 20px; overflow-wrap: anywhere; }
.operation a { display: inline-flex; gap: 4px; margin-top: 8px; font-size: 12px; font-weight: 500; color: #178a5a; }
.operation a:hover { text-decoration: underline; }
:global(.dark) .operation a { color: #70d5a7; }
@media (max-width: 767px) {
  .operations { grid-template-columns: 1fr; }
  .operation { padding: 14px 18px; }
  .operation + .operation { border-left: 0; border-top: 1px solid hsl(var(--border)); }
}
</style>
