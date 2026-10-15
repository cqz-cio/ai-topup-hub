<template>
  <div class="space-y-5">
    <ResellerSectionHeader :title="t('resellerConsole.withdraws.title')" :description="t('resellerConsole.withdraws.description')" />

    <Card class="p-5 space-y-4">
      <p class="text-sm text-muted-foreground">{{ t('walletEarnings.unified') }}</p>
      <Button as-child><RouterLink to="/me/wallet">{{ t('walletEarnings.walletLink') }}</RouterLink></Button>
    </Card>
    <h3 class="text-sm font-semibold">{{ t('walletEarnings.legacy') }}</h3>

    <ResellerPageState v-if="withdrawsLoading" loading :title="t('resellerConsole.common.loading')" />
    <ResellerPageState v-else-if="withdraws.length === 0" :title="t('personalCenter.reseller.withdrawEmpty')" :icon="Upload" />

    <template v-else>
      <!-- 桌面表 -->
      <ResellerDataTable class="hidden lg:block">
        <TableHeader>
          <TableRow class="bg-muted/50">
            <TableHead class="px-4 text-right">{{ t('personalCenter.reseller.withdrawTable.amount') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.reseller.withdrawTable.channel') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.reseller.withdrawTable.status') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.reseller.withdrawTable.createdAt') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.reseller.withdrawTable.processedAt') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="item in withdraws" :key="item.id">
            <TableCell class="px-4 py-3 text-right font-mono text-xs text-foreground">{{ formatResellerConsoleAmount(item.amount, item.currency) }}</TableCell>
            <TableCell class="px-4 py-3 text-xs text-foreground">{{ item.channel }}</TableCell>
            <TableCell class="px-4 py-3">
              <ResellerStatusBadge :label="withdrawStatusLabel(item.status)" :tone="withdrawTone(item.status)" />
              <span
                v-if="item.status === RESELLER_WITHDRAW_STATUS_REJECTED && item.reject_reason"
                class="mt-1 block max-w-[16rem] break-all text-xs text-destructive"
              >
                {{ t('personalCenter.reseller.withdrawTable.rejectReason') }}：{{ item.reject_reason }}
              </span>
            </TableCell>
            <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatResellerConsoleDate(item.created_at) }}</TableCell>
            <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatResellerConsoleDate(item.processed_at) }}</TableCell>
          </TableRow>
        </TableBody>
      </ResellerDataTable>

      <!-- 移动卡片 -->
      <div class="space-y-3 lg:hidden">
        <ResellerRecordCard v-for="item in withdraws" :key="item.id">
          <template #header>
            <span class="font-mono text-sm font-bold text-foreground">{{ formatResellerConsoleAmount(item.amount, item.currency) }}</span>
            <ResellerStatusBadge :label="withdrawStatusLabel(item.status)" :tone="withdrawTone(item.status)" />
          </template>
          <div class="col-span-2">
            <dt class="text-muted-foreground">{{ t('personalCenter.reseller.withdrawTable.channel') }}</dt>
            <dd class="mt-0.5 break-all text-foreground">{{ item.channel }}</dd>
          </div>
          <div v-if="item.status === RESELLER_WITHDRAW_STATUS_REJECTED && item.reject_reason" class="col-span-2">
            <dt class="text-muted-foreground">{{ t('personalCenter.reseller.withdrawTable.rejectReason') }}</dt>
            <dd class="mt-0.5 break-all text-destructive">{{ item.reject_reason }}</dd>
          </div>
          <div>
            <dt class="text-muted-foreground">{{ t('personalCenter.reseller.withdrawTable.createdAt') }}</dt>
            <dd class="mt-0.5 text-foreground">{{ formatResellerConsoleDate(item.created_at) }}</dd>
          </div>
          <div>
            <dt class="text-muted-foreground">{{ t('personalCenter.reseller.withdrawTable.processedAt') }}</dt>
            <dd class="mt-0.5 text-foreground">{{ formatResellerConsoleDate(item.processed_at) }}</dd>
          </div>
        </ResellerRecordCard>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Upload } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import ResellerDataTable from '../../components/reseller-console/ResellerDataTable.vue'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import ResellerRecordCard from '../../components/reseller-console/ResellerRecordCard.vue'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'
import ResellerStatusBadge, { type ResellerBadgeTone } from '../../components/reseller-console/ResellerStatusBadge.vue'
import { useResellerFinance } from '../../composables/reseller/useResellerFinance'
import { RESELLER_WITHDRAW_STATUS_PAID, RESELLER_WITHDRAW_STATUS_PENDING, RESELLER_WITHDRAW_STATUS_REJECTED } from '../../constants/reseller'
import { formatResellerConsoleAmount, formatResellerConsoleDate } from '../../utils/resellerConsole'

const { t } = useI18n()
const { withdraws, withdrawsLoading, loadWithdraws } = useResellerFinance()

const withdrawStatusLabel = (status?: string) => {
  if (status === RESELLER_WITHDRAW_STATUS_PENDING) return t('personalCenter.reseller.withdrawStatus.pending')
  if (status === RESELLER_WITHDRAW_STATUS_REJECTED) return t('personalCenter.reseller.withdrawStatus.rejected')
  if (status === RESELLER_WITHDRAW_STATUS_PAID) return t('personalCenter.reseller.withdrawStatus.paid')
  return status || '-'
}

const withdrawTone = (status?: string): ResellerBadgeTone => {
  if (status === RESELLER_WITHDRAW_STATUS_PAID) return 'success'
  if (status === RESELLER_WITHDRAW_STATUS_PENDING) return 'warning'
  return 'neutral'
}

onMounted(() => {
  void loadWithdraws({ page: 1 })
})
</script>
