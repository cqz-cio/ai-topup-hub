<template>
  <div class="commerce-page order-receipt-page min-h-screen bg-background text-foreground pt-24 pb-16">
    <div class="container mx-auto px-4">
      <CommerceHeading :title="t('shopUi.deliveryTitle')" :description="t('shopUi.deliveryDescription')">
        <template #actions><Button as-child variant="outline" size="sm"><router-link to="/guest/orders">{{ t('guestOrderDetail.backSearch') }}</router-link></Button></template>
      </CommerceHeading>

      <div v-if="viewState === 'auth'"
        class="rounded-2xl border bg-card p-6 shadow-sm mb-6">
        <h2 class="text-lg font-bold mb-2">{{ t('guestOrderDetail.authTitle') }}</h2>
        <p class="text-xs text-muted-foreground mb-4">{{ t('guestOrderDetail.authHint') }}</p>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Input v-model="auth.email" type="email" class="h-11" :placeholder="t('guestOrders.emailPlaceholder')" />
          <Input v-model="auth.order_password" type="password" class="h-11" :placeholder="t('guestOrders.passwordPlaceholder')" />
        </div>
        <Alert v-if="authError" variant="destructive" class="mt-4">
          <AlertDescription>{{ authError }}</AlertDescription>
        </Alert>
        <div class="mt-4 flex items-center gap-3">
          <Button class="font-bold" @click="handleAuthSubmit">
            {{ t('guestOrderDetail.authSubmit') }}
          </Button>
          <Button type="button" variant="link" class="h-auto p-0 text-xs font-normal text-muted-foreground hover:text-foreground" @click="clearAuth">
            {{ t('guestOrderDetail.authClear') }}
          </Button>
        </div>
      </div>

      <div v-else-if="viewState === 'loading'"
        class="h-40 bg-muted border rounded-2xl animate-pulse">
      </div>

      <div v-else-if="viewState === 'empty'"
        class="rounded-2xl border bg-card shadow-sm p-12 text-center">
        <p class="text-muted-foreground">{{ t('guestOrderDetail.notFound') }}</p>
      </div>

      <div v-else-if="viewState === 'detail' && order" class="space-y-6">
        <CheckoutSteps :current-step="deliveryState === 'unpaid' ? 'payment' : 'delivery'" :complete="deliveryState === 'ready'" v-if="deliveryState !== 'closed'" />
        <div class="rounded-2xl border bg-card shadow-sm p-6">
          <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
            <div>
              <div class="text-xs uppercase tracking-wider text-muted-foreground">{{ t('orders.orderNo') }}</div>
              <div class="text-sm font-semibold text-foreground mt-1">{{ order.order_no }}</div>
              <div class="text-xs text-muted-foreground mt-2">{{ t('orderDetail.createdAtLabel') }}：{{ formatDate(order.created_at) }}</div>
            </div>
            <div class="flex flex-col items-start md:items-end gap-2">
              <div class="text-xs uppercase tracking-wider text-muted-foreground">{{ t('orderDetail.amountTotal') }}</div>
              <div class="text-lg font-bold text-foreground">{{ formatMoney(order.total_amount,
                order.currency) }}</div>
            </div>
            <div class="flex items-center gap-3">
              <Badge :variant="statusVariant(order.status)" size="sm">
                {{ statusLabel(order.status) }}
              </Badge>
              <Button v-if="order.status === 'pending_payment'" as-child size="sm">
                <router-link :to="`/pay?guest=1&order_no=${order.order_no}`">{{ t('orders.payNow') }}</router-link>
              </Button>
            </div>
          </div>
        </div>

        <OrderDelivery :order="order" :downloading="fulfillmentDownloading" @download="handleDownloadFulfillment" />

        <div v-if="order.children && order.children.length > 0"
          class="rounded-2xl border bg-card shadow-sm p-6">
          <h2 class="text-lg font-bold mb-4">{{ t('orderDetail.childOrdersTitle') }}</h2>
          <div class="space-y-4">
            <div v-for="child in order.children" :key="child.id"
              class="border rounded-2xl p-4">
              <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
                <div>
                  <div class="text-sm text-muted-foreground">{{ t('orderDetail.childOrderNo') }}：{{ child.order_no }}</div>
                  <div class="text-xs text-muted-foreground mt-1">{{ t('orderDetail.childOrderAmount') }}：{{
                    formatMoney(child.total_amount, child.currency || order.currency) }}</div>
                </div>
                <Badge :variant="statusVariant(resolvedChildStatus(child))" size="sm">
                  {{ statusLabel(resolvedChildStatus(child)) }}
                </Badge>
              </div>
              <div class="mt-4">
                <h3 class="text-sm font-semibold text-foreground mb-3">{{ t('orderDetail.childItemsTitle')
                  }}</h3>
                <div v-if="child.items && child.items.length" class="space-y-3">
                  <div v-for="(item, cidx) in child.items" :key="cidx"
                    class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3 sm:gap-4 border-b border-gray-100 pb-3 text-sm text-muted-foreground dark:border-white/5">
                    <div class="flex min-w-0 items-start gap-3">
                      <div class="h-14 w-14 shrink-0 overflow-hidden rounded-xl border border-gray-200 bg-white transition-all duration-200 hover:-translate-y-0.5 hover:shadow-sm dark:border-white/10 dark:bg-black/30 sm:h-16 sm:w-16">
                        <img
                          v-if="orderItemImage(item)"
                          :src="orderItemImage(item)"
                          :alt="getLocalizedText(item.title)"
                          loading="lazy"
                          decoding="async"
                          class="h-full w-full object-cover"
                        />
                        <div v-else class="flex h-full w-full items-center justify-center text-gray-400 dark:text-gray-500">
                          <ImageIcon class="h-5 w-5" :stroke-width="1.5" />
                        </div>
                      </div>
                      <div class="min-w-0">
                        <div class="text-foreground font-medium">{{ getLocalizedText(item.title) }}</div>
                        <div class="text-xs text-muted-foreground">{{ t('orderDetail.quantityLabel') }}：{{ item.quantity }}</div>
                        <div v-if="orderItemSkuText(item)" class="text-xs text-muted-foreground mt-1">{{ t('orderDetail.itemSkuLabel') }}：{{ orderItemSkuText(item) }}</div>
                        <div class="text-xs text-muted-foreground mt-1">
                          {{ t('orderDetail.itemFulfillmentLabel') }}：{{ fulfillmentTypeLabelText(item.fulfillment_type) }}
                        </div>
                        <div v-if="item.tags && item.tags.length" class="mt-2 flex flex-wrap gap-2">
                          <Badge v-for="(tag, index) in item.tags" :key="index" variant="neutral" size="sm" class="rounded-full">{{ tag }}</Badge>
                        </div>
                        <div v-if="manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot).length"
                          class="mt-3 rounded-xl border border-gray-200 bg-white p-3 text-xs text-gray-600 dark:border-white/10 dark:bg-black/30 dark:text-gray-300">
                          <div class="mb-2 font-semibold text-muted-foreground">{{ t('orderDetail.manualSubmissionTitle') }}</div>
                          <div v-for="row in manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot)" :key="row.key" class="mb-1 last:mb-0">
                            <span class="text-foreground">{{ row.label }}</span>：{{ row.value }}
                          </div>
                        </div>
                      </div>
                    </div>
                    <div class="shrink-0 pl-[4.25rem] sm:pl-0 text-left sm:text-right text-sm text-muted-foreground space-y-1">
                      <div>{{ t('orderDetail.unitPriceLabel') }}：{{ formatMoney(item.original_unit_price, order.currency) }}
                      </div>
                      <div>{{ t('orderDetail.totalPriceLabel') }}：{{ formatMoney(item.original_total_price, order.currency) }}
                      </div>
                      <div v-if="hasDiscountAmount(item.coupon_discount_amount)">
                        {{ t('orderDetail.couponDiscountLabel') }}：{{ formatDiscountMoney(item.coupon_discount_amount,
                        order.currency) }}
                      </div>
                      <div v-if="hasDiscountAmount(item.promotion_discount_amount)">
                        {{ t('orderDetail.promotionDiscountLabel') }}：{{ formatDiscountMoney(item.promotion_discount_amount,
                        order.currency) }}
                      </div>
                      <div v-if="hasDiscountAmount(item.member_discount_amount)">
                        {{ t('orderDetail.memberDiscountLabel') }}：{{ formatDiscountMoney(item.member_discount_amount,
                        order.currency) }}
                      </div>
                      <div v-if="hasItemDiscount(item)" class="font-medium text-rose-600 dark:text-rose-300">
                        {{ t('orderDetail.itemDiscountTotalLabel') }}：{{ formatItemDiscountTotal(item, order.currency) }}
                      </div>
                      <div class="font-medium text-foreground">
                        {{ t('orderDetail.itemPaidAmountLabel') }}：{{ formatItemPaidAmount(item, order.currency) }}
                      </div>
                    </div>
                  </div>
                </div>
                <div v-else class="text-sm text-muted-foreground">{{ t('orderDetail.noItems') }}</div>
              </div>

            </div>
          </div>
        </div>



        <div class="rounded-2xl border bg-card shadow-sm p-6">
          <h2 class="text-lg font-bold mb-4">{{ t('orderDetail.itemsTitle') }}</h2>
          <div v-if="order.items && order.items.length > 0" class="space-y-4">
            <div v-for="(item, idx) in order.items" :key="idx"
              class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3 sm:gap-4 border-b border-gray-100 pb-3 dark:border-white/5">
              <div class="flex min-w-0 items-start gap-3">
                <div class="h-14 w-14 shrink-0 overflow-hidden rounded-xl border border-gray-200 bg-white transition-all duration-200 hover:-translate-y-0.5 hover:shadow-sm dark:border-white/10 dark:bg-black/30 sm:h-16 sm:w-16">
                  <img
                    v-if="orderItemImage(item)"
                    :src="orderItemImage(item)"
                    :alt="getLocalizedText(item.title)"
                    loading="lazy"
                    decoding="async"
                    class="h-full w-full object-cover"
                  />
                  <div v-else class="flex h-full w-full items-center justify-center text-gray-400 dark:text-gray-500">
                    <ImageIcon class="h-5 w-5" :stroke-width="1.5" />
                  </div>
                </div>
                <div class="min-w-0">
                  <div class="text-foreground font-medium">{{ getLocalizedText(item.title) }}</div>
                  <div class="text-xs text-muted-foreground">{{ t('orderDetail.quantityLabel') }}：{{ item.quantity }}</div>
                  <div v-if="orderItemSkuText(item)" class="text-xs text-muted-foreground mt-1">{{ t('orderDetail.itemSkuLabel') }}：{{ orderItemSkuText(item) }}</div>
                  <div class="text-xs text-muted-foreground mt-1">
                    {{ t('orderDetail.itemFulfillmentLabel') }}：{{ fulfillmentTypeLabelText(item.fulfillment_type) }}
                  </div>
                  <div v-if="item.tags && item.tags.length" class="mt-2 flex flex-wrap gap-2">
                    <Badge v-for="(tag, index) in item.tags" :key="index" variant="neutral" size="sm" class="rounded-full">{{ tag }}</Badge>
                  </div>
                  <div v-if="manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot).length"
                    class="mt-3 rounded-xl border border-gray-200 bg-white p-3 text-xs text-gray-600 dark:border-white/10 dark:bg-black/30 dark:text-gray-300">
                    <div class="mb-2 font-semibold text-muted-foreground">{{ t('orderDetail.manualSubmissionTitle') }}</div>
                    <div v-for="row in manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot)" :key="row.key" class="mb-1 last:mb-0">
                      <span class="text-foreground">{{ row.label }}</span>：{{ row.value }}
                    </div>
                  </div>
                </div>
              </div>
              <div class="shrink-0 pl-[4.25rem] sm:pl-0 text-left sm:text-right text-sm text-muted-foreground space-y-1">
                <div>{{ t('orderDetail.unitPriceLabel') }}：{{ formatMoney(item.original_unit_price, order.currency) }}</div>
                <div>{{ t('orderDetail.totalPriceLabel') }}：{{ formatMoney(item.original_total_price, order.currency) }}</div>
                <div v-if="hasDiscountAmount(item.coupon_discount_amount)">
                  {{ t('orderDetail.couponDiscountLabel') }}：{{ formatDiscountMoney(item.coupon_discount_amount, order.currency)
                  }}
                </div>
                <div v-if="hasDiscountAmount(item.promotion_discount_amount)">
                  {{ t('orderDetail.promotionDiscountLabel') }}：{{ formatDiscountMoney(item.promotion_discount_amount,
                  order.currency) }}
                </div>
                <div v-if="hasDiscountAmount(item.wholesale_discount_amount)">
                  {{ t('orderDetail.wholesaleDiscountLabel') }}：{{ formatDiscountMoney(item.wholesale_discount_amount,
                  order.currency) }}
                </div>
                <div v-if="hasDiscountAmount(item.member_discount_amount)">
                  {{ t('orderDetail.memberDiscountLabel') }}：{{ formatDiscountMoney(item.member_discount_amount, order.currency) }}
                </div>
                <div v-if="hasItemDiscount(item)" class="font-medium text-rose-600 dark:text-rose-300">
                  {{ t('orderDetail.itemDiscountTotalLabel') }}：{{ formatItemDiscountTotal(item, order.currency) }}
                </div>
                <div class="font-medium text-foreground">
                  {{ t('orderDetail.itemPaidAmountLabel') }}：{{ formatItemPaidAmount(item, order.currency) }}
                </div>
              </div>
            </div>
          </div>
          <div v-else class="text-sm text-muted-foreground">{{ t('orderDetail.noItems') }}</div>
        </div>

        <div class="rounded-2xl border bg-card shadow-sm p-6">
          <h2 class="text-lg font-bold mb-4">{{ t('orderDetail.amountTitle') }}</h2>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
            <div class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountOriginal') }}</div>
              <div class="text-foreground font-mono mt-1">{{ formatMoney(order.original_amount,
                order.currency) }}</div>
            </div>
            <div class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountDiscount') }}</div>
              <div
                class="font-mono mt-1"
                :class="hasDiscountAmount(order.discount_amount) ? 'text-rose-600 dark:text-rose-300' : 'text-foreground'"
              >
                {{ formatDiscountMoney(order.discount_amount, order.currency) }}
              </div>
            </div>
            <div class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountTotal') }}</div>
              <div class="text-foreground font-mono mt-1">{{ formatMoney(order.total_amount,
                order.currency) }}</div>
            </div>
            <div v-if="hasDiscountAmount(order.member_discount_amount)" class="border border-amber-200 bg-amber-50/50 dark:border-amber-800 dark:bg-amber-950/30 rounded-xl p-4">
              <div class="text-xs text-amber-700 dark:text-amber-400">{{ t('orderDetail.amountMemberDiscount') }}</div>
              <div class="text-amber-700 dark:text-amber-400 font-mono mt-1">{{ formatDiscountMoney(order.member_discount_amount,
                order.currency) }}</div>
            </div>
            <div v-if="hasDiscountAmount(order.wholesale_discount_amount)" class="border border-emerald-200 bg-emerald-50/50 dark:border-emerald-800 dark:bg-emerald-950/30 rounded-xl p-4">
              <div class="text-xs text-emerald-700 dark:text-emerald-400">{{ t('orderDetail.amountWholesaleDiscount') }}</div>
              <div class="text-emerald-700 dark:text-emerald-400 font-mono mt-1">{{ formatDiscountMoney(order.wholesale_discount_amount,
                order.currency) }}</div>
            </div>
          </div>
        </div>

        <div v-if="showRefundRecordsCard" class="rounded-2xl border bg-card shadow-sm p-6">
          <h2 class="text-lg font-bold mb-4">{{ t('orderDetail.refundRecordsTitle') }}</h2>
          <div v-if="refundRecords.length > 0" class="overflow-x-auto rounded-xl border">
            <Table>
              <TableHeader>
                <TableRow class="bg-muted/50">
                  <TableHead class="px-4">{{ t('orderDetail.refundRecordTime') }}</TableHead>
                  <TableHead class="px-4">{{ t('orderDetail.refundRecordAmount') }}</TableHead>
                  <TableHead class="px-4">{{ t('orderDetail.refundRecordReason') }}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow
                  v-for="(record, idx) in refundRecords"
                  :key="`refund-record-row-${idx}`"
                >
                  <TableCell class="px-4 text-xs text-muted-foreground whitespace-nowrap">{{ formatDate(record.created_at) }}</TableCell>
                  <TableCell class="px-4 font-mono text-sm text-foreground whitespace-nowrap">{{ formatMoney(record.amount, record.currency || order.currency) }}</TableCell>
                  <TableCell class="px-4 text-xs text-muted-foreground whitespace-pre-wrap break-words">{{ refundReasonText(record.remark) }}</TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>
          <div v-else class="text-sm text-muted-foreground">{{ t('orderDetail.refundRecordsEmpty') }}</div>
        </div>

        <div v-if="showTimeCard" class="rounded-2xl border bg-card shadow-sm p-6">
          <h2 class="text-lg font-bold mb-4">{{ t('orderDetail.timeTitle') }}</h2>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
            <div class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('orderDetail.createdAtLabel') }}</div>
              <div class="text-foreground mt-1">{{ formatDate(order.created_at) }}</div>
            </div>
            <div v-if="order.paid_at" class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('orderDetail.paidAtLabel') }}</div>
              <div class="text-foreground mt-1">{{ formatDate(order.paid_at) }}</div>
            </div>
            <div v-if="order.expires_at" class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('orderDetail.expiresAtLabel') }}</div>
              <div class="text-foreground mt-1">{{ formatDate(order.expires_at) }}</div>
            </div>
            <div v-if="order.canceled_at" class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('orderDetail.canceledAtLabel') }}</div>
              <div class="text-foreground mt-1">{{ formatDate(order.canceled_at) }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import CommerceHeading from '../components/commerce/CommerceHeading.vue'
import OrderDelivery from '../components/commerce/OrderDelivery.vue'
import CheckoutSteps from '../components/checkout/CheckoutSteps.vue'
import { orderDeliveryState } from '../utils/orderDelivery'
import { Image as ImageIcon } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useGuestOrderDetail } from '../composables/useGuestOrderDetail'

const { t } = useI18n()

const {
  order, authError, auth, viewState, handleAuthSubmit, clearAuth,
  fulfillmentDownloading, handleDownloadFulfillment,
  statusLabel, statusVariant, fulfillmentTypeLabelText,
  formatDate, getLocalizedText, formatMoney, formatDiscountMoney, hasDiscountAmount,
  refundReasonText, showRefundRecordsCard, refundRecords, showTimeCard,
  orderItemImage, orderItemSkuText, manualSubmissionRows,
  hasItemDiscount, formatItemDiscountTotal, formatItemPaidAmount, resolvedChildStatus,
} = useGuestOrderDetail()
const deliveryState = computed(() => orderDeliveryState(order.value))
</script>
