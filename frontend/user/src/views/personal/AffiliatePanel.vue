<template>
  <div class="space-y-6">
    <div class="rounded-2xl border bg-card p-7 shadow-sm">
      <PanelHeading :title="t('personalCenter.affiliate.title')" :description="t('personalCenter.affiliate.subtitle')" :icon="Megaphone">
        <template #actions>
          <Badge variant="accent" size="sm">{{ t('personalCenter.tabs.affiliate') }}</Badge>
        </template>
      </PanelHeading>

      <Alert v-if="panelAlert" class="mb-5" :variant="pageAlertVariant(panelAlert.level)" :class="pageAlertToneClass(panelAlert.level)">
        <AlertDescription>{{ panelAlert.message }}</AlertDescription>
      </Alert>

      <div v-if="loading" class="space-y-3">
        <div v-for="idx in 3" :key="idx" class="h-16 animate-pulse rounded-xl border bg-muted"></div>
      </div>
      <div v-else-if="!dashboard" class="rounded-xl border border-dashed p-5">
        <p class="text-sm text-muted-foreground">{{ t('personalCenter.affiliate.errors.loadFailed') }}</p>
        <Button type="button" variant="outline" class="mt-3" @click="initialize">{{ t('orders.filters.refresh') }}</Button>
      </div>
      <Alert v-else-if="!dashboard.enabled" class="mb-4"><AlertDescription>{{ t('featureAvailability.affiliateClosed') }}</AlertDescription></Alert>
      <Alert v-else-if="dashboard.opened && dashboard.profile_status !== 'active'" class="mb-4"><AlertDescription>{{ t('featureAvailability.affiliateSuspended') }}</AlertDescription></Alert>

      <template v-if="!loading && dashboard?.opened">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
          <div class="rounded-xl border p-4 md:col-span-2">
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.affiliate.affiliateCode') }}</div>
            <div class="mt-2 flex flex-wrap items-center gap-2">
              <span class="rounded-lg border border-border bg-muted/30 px-2 py-1 font-mono text-sm text-foreground">{{ dashboard?.affiliate_code || '-' }}</span>
              <Button type="button" variant="outline" size="sm" @click="copyPromotionUrl">
                {{ t('personalCenter.affiliate.copyPromotionUrl') }}
              </Button>
            </div>
            <div class="mt-3 text-xs text-muted-foreground break-all">{{ promotionUrl }}</div>
          </div>
          <div class="rounded-xl border p-4">
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.affiliate.conversionRate') }}</div>
            <div class="mt-2 text-lg font-bold text-foreground">{{ conversionRateText }}</div>
            <div class="mt-2 text-xs text-muted-foreground">
              {{ t('personalCenter.affiliate.conversionDetail', { clicks: dashboard?.click_count || 0, orders: dashboard?.valid_order_count || 0 }) }}
            </div>
          </div>
          <div class="rounded-xl border p-4">
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.affiliate.pendingCommission') }}</div>
            <div class="mt-2 text-lg font-bold text-foreground">{{ dashboard?.pending_commission || '0.00' }}</div>
          </div>
          <div class="rounded-xl border p-4">
            <div class="text-xs text-muted-foreground">{{ t('walletEarnings.settled') }}</div>
            <div class="mt-2 text-lg font-bold text-foreground">{{ dashboard?.available_commission || '0.00' }}</div>
          </div>
          <div class="rounded-xl border p-4">
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.affiliate.withdrawnCommission') }}</div>
            <div class="mt-2 text-lg font-bold text-foreground">{{ dashboard?.withdrawn_commission || '0.00' }}</div>
          </div>
        </div>
      </template>

      <div v-else-if="!loading && dashboard?.can_open" class="rounded-xl border border-dashed p-5">
        <p class="text-sm text-muted-foreground">{{ t('personalCenter.affiliate.notOpened') }}</p>
        <Button type="button" :disabled="opening" class="mt-4 font-bold" @click="openAffiliate">
          {{ opening ? t('personalCenter.affiliate.opening') : t('personalCenter.affiliate.openButton') }}
        </Button>
      </div>
    </div>

    <div class="rounded-2xl border bg-card p-5 sm:p-7 space-y-3">
      <p class="text-sm text-muted-foreground">{{ t('walletEarnings.affiliateRule') }}</p>
      <p class="text-sm">{{ t('walletEarnings.unified') }}</p>
      <Button as-child><RouterLink to="/me/wallet">{{ t('walletEarnings.walletLink') }}</RouterLink></Button>
    </div>

    <div v-if="dashboard?.opened" class="rounded-2xl border bg-card p-7 shadow-sm">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-lg font-bold text-foreground">{{ t('personalCenter.affiliate.commissionTitle') }}</h3>
        <Button type="button" variant="outline" size="sm" @click="loadCommissions(commissionsPagination.page)">
          {{ t('orders.filters.refresh') }}
        </Button>
      </div>

      <div v-if="commissionsLoading" class="space-y-3">
        <div v-for="idx in 3" :key="idx" class="h-14 animate-pulse rounded-xl border bg-muted"></div>
      </div>
      <div v-else-if="commissions.length === 0" class="rounded-xl border border-dashed px-4 py-6 text-sm text-muted-foreground">
        {{ t('personalCenter.affiliate.commissionEmpty') }}
      </div>
      <div v-else class="overflow-x-auto rounded-xl border">
        <Table>
          <TableHeader>
            <TableRow class="bg-muted/50">
              <TableHead class="px-4">{{ t('personalCenter.affiliate.table.orderNo') }}</TableHead>
              <TableHead class="px-4">{{ t('personalCenter.affiliate.table.amount') }}</TableHead>
              <TableHead class="px-4">{{ t('personalCenter.affiliate.table.status') }}</TableHead>
              <TableHead class="px-4">{{ t('personalCenter.affiliate.table.createdAt') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="item in commissions" :key="item.id">
              <TableCell class="px-4 font-mono text-xs text-foreground">-</TableCell>
              <TableCell class="px-4 font-mono text-xs text-foreground">{{ item.commission_amount }}</TableCell>
              <TableCell class="px-4">
                <Badge :variant="commissionStatusVariant(item.status)" size="sm">
                  {{ commissionStatusLabel(item.status) }}
                </Badge>
              </TableCell>
              <TableCell class="px-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <PaginationNav
        :current-page="commissionsPagination.page"
        :total-pages="commissionsPagination.total_page"
        :loading="commissionsLoading"
        :scroll-top="false"
        @change-page="loadCommissions"
      />
    </div>

    <div v-if="dashboard?.opened" class="rounded-2xl border bg-card p-7 shadow-sm">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-lg font-bold text-foreground">{{ t('walletEarnings.legacy') }}</h3>
        <Button type="button" variant="outline" size="sm" @click="loadWithdraws(withdrawsPagination.page)">
          {{ t('orders.filters.refresh') }}
        </Button>
      </div>

      <div v-if="withdrawsLoading" class="space-y-3">
        <div v-for="idx in 3" :key="idx" class="h-14 animate-pulse rounded-xl border bg-muted"></div>
      </div>
      <div v-else-if="withdraws.length === 0" class="rounded-xl border border-dashed px-4 py-6 text-sm text-muted-foreground">
        {{ t('personalCenter.affiliate.withdrawEmpty') }}
      </div>
      <div v-else class="overflow-x-auto rounded-xl border">
        <Table>
          <TableHeader>
            <TableRow class="bg-muted/50">
              <TableHead class="px-4">{{ t('personalCenter.affiliate.withdrawTable.amount') }}</TableHead>
              <TableHead class="px-4">{{ t('personalCenter.affiliate.withdrawTable.channel') }}</TableHead>
              <TableHead class="px-4">{{ t('personalCenter.affiliate.withdrawTable.status') }}</TableHead>
              <TableHead class="px-4">{{ t('personalCenter.affiliate.withdrawTable.createdAt') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="item in withdraws" :key="item.id">
              <TableCell class="px-4 font-mono text-xs text-foreground">{{ item.amount }}</TableCell>
              <TableCell class="px-4 text-xs text-muted-foreground">{{ item.channel }}</TableCell>
              <TableCell class="px-4">
                <Badge :variant="withdrawStatusVariant(item.status)" size="sm">
                  {{ withdrawStatusLabel(item.status) }}
                </Badge>
              </TableCell>
              <TableCell class="px-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <PaginationNav
        :current-page="withdrawsPagination.page"
        :total-pages="withdrawsPagination.total_page"
        :loading="withdrawsLoading"
        :scroll-top="false"
        @change-page="loadWithdraws"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Megaphone } from 'lucide-vue-next'
import PanelHeading from '../../components/shared/PanelHeading.vue'
import { affiliateAPI, type AffiliateCommissionData, type AffiliateDashboardData, type AffiliateWithdrawData } from '../../api'
import {
  AFFILIATE_COMMISSION_STATUS_AVAILABLE,
  AFFILIATE_COMMISSION_STATUS_PENDING_CONFIRM,
  AFFILIATE_COMMISSION_STATUS_REJECTED,
  AFFILIATE_COMMISSION_STATUS_WITHDRAWN,
  AFFILIATE_WITHDRAW_STATUS_PAID,
  AFFILIATE_WITHDRAW_STATUS_PENDING_REVIEW,
  AFFILIATE_WITHDRAW_STATUS_REJECTED,
} from '../../constants/affiliate'
import { useAppStore } from '../../stores/app'
import { pageAlertVariant, pageAlertToneClass, type PageAlert } from '../../utils/alerts'
import type { BadgeTone } from '../../utils/status'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import PaginationNav from '../../components/PaginationNav.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const opening = ref(false)
const commissionsLoading = ref(false)
const withdrawsLoading = ref(false)
const dashboard = ref<AffiliateDashboardData | null>(null)
const panelAlert = ref<PageAlert | null>(null)

const commissions = ref<AffiliateCommissionData[]>([])
const withdraws = ref<AffiliateWithdrawData[]>([])

const commissionsPagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})

const withdrawsPagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})

const promotionUrl = computed(() => {
  if (!dashboard.value?.affiliate_code) return '-'
  const path = dashboard.value.promotion_path || `/?aff=${dashboard.value.affiliate_code}`
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return `${origin}${path}`
})

const conversionRateText = computed(() => {
  const value = Number(dashboard.value?.conversion_rate || 0)
  if (!Number.isFinite(value)) return '0.00%'
  return `${value.toFixed(2)}%`
})

const formatDate = (raw?: string) => {
  if (!raw) return '-'
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
}

const loadDashboard = async () => {
  try {
    const response = await affiliateAPI.dashboard()
    dashboard.value = response.data.data || null
  } catch (err: any) {
    dashboard.value = null
    panelAlert.value = {
      level: 'error',
      message: err?.message || t('personalCenter.affiliate.errors.loadFailed'),
    }
  }
}

const loadCommissions = async (page = 1) => {
  commissionsLoading.value = true
  try {
    const response = await affiliateAPI.commissions({
      page,
      page_size: commissionsPagination.page_size,
    })
    commissions.value = response.data.data || []
    Object.assign(commissionsPagination, response.data.pagination || commissionsPagination)
  } catch {
    commissions.value = []
  } finally {
    commissionsLoading.value = false
  }
}

const loadWithdraws = async (page = 1) => {
  withdrawsLoading.value = true
  try {
    const response = await affiliateAPI.withdraws({
      page,
      page_size: withdrawsPagination.page_size,
    })
    withdraws.value = response.data.data || []
    Object.assign(withdrawsPagination, response.data.pagination || withdrawsPagination)
  } catch {
    withdraws.value = []
  } finally {
    withdrawsLoading.value = false
  }
}

const reloadOpenedData = async () => {
  if (!dashboard.value?.opened) return
  await Promise.all([loadCommissions(1), loadWithdraws(1)])
}

const initialize = async () => {
  loading.value = true
  panelAlert.value = null
  await appStore.loadConfig()
  await loadDashboard()
  await reloadOpenedData()
  loading.value = false
}

const openAffiliate = async () => {
  if (!dashboard.value?.can_open || opening.value) return
  opening.value = true
  panelAlert.value = null
  try {
    await affiliateAPI.open()
    await loadDashboard()
    await reloadOpenedData()
    panelAlert.value = {
      level: 'success',
      message: t('personalCenter.affiliate.openSuccess'),
    }
  } catch (err: any) {
    panelAlert.value = {
      level: 'error',
      message: err?.message || t('personalCenter.affiliate.errors.openFailed'),
    }
  } finally {
    opening.value = false
  }
}

const copyPromotionUrl = async () => {
  if (!dashboard.value?.affiliate_code || !promotionUrl.value || promotionUrl.value === '-') return
  try {
    await navigator.clipboard.writeText(promotionUrl.value)
    panelAlert.value = {
      level: 'success',
      message: t('personalCenter.affiliate.copySuccess'),
    }
  } catch {
    panelAlert.value = {
      level: 'error',
      message: t('personalCenter.affiliate.errors.copyFailed'),
    }
  }
}

const commissionStatusLabel = (status?: string) => {
  if (status === AFFILIATE_COMMISSION_STATUS_PENDING_CONFIRM) return t('personalCenter.affiliate.commissionStatus.pendingConfirm')
  if (status === AFFILIATE_COMMISSION_STATUS_AVAILABLE) return t('walletEarnings.settled')
  if (status === AFFILIATE_COMMISSION_STATUS_REJECTED) return t('personalCenter.affiliate.commissionStatus.rejected')
  if (status === AFFILIATE_COMMISSION_STATUS_WITHDRAWN) return t('personalCenter.affiliate.commissionStatus.withdrawn')
  return status || '-'
}

const commissionStatusVariant = (status?: string): BadgeTone => {
  if (status === AFFILIATE_COMMISSION_STATUS_PENDING_CONFIRM) return 'warning'
  if (status === AFFILIATE_COMMISSION_STATUS_AVAILABLE) return 'success'
  if (status === AFFILIATE_COMMISSION_STATUS_WITHDRAWN) return 'info'
  return 'neutral'
}

const withdrawStatusLabel = (status?: string) => {
  if (status === AFFILIATE_WITHDRAW_STATUS_PENDING_REVIEW) return t('personalCenter.affiliate.withdrawStatus.pendingReview')
  if (status === AFFILIATE_WITHDRAW_STATUS_REJECTED) return t('personalCenter.affiliate.withdrawStatus.rejected')
  if (status === AFFILIATE_WITHDRAW_STATUS_PAID) return t('personalCenter.affiliate.withdrawStatus.paid')
  return status || '-'
}

const withdrawStatusVariant = (status?: string): BadgeTone => {
  if (status === AFFILIATE_WITHDRAW_STATUS_PENDING_REVIEW) return 'warning'
  if (status === AFFILIATE_WITHDRAW_STATUS_PAID) return 'success'
  return 'neutral'
}

onMounted(() => {
  initialize()
})
</script>
