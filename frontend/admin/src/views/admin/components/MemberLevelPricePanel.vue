<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAdminAuthStore } from '@/stores/auth'
import { adminAPI } from '@/api/admin'
import type { AdminProduct } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatMoney, getLocalizedText } from '@/utils/format'
import { notifySuccess } from '@/utils/notify'
import type { UserPriceDraft, UserPriceInput } from '@/utils/userPricing'
import { buildLevelProductPricePayload } from '@/utils/levelProductPricing'

const props = defineProps<{ levelId: number; siteCurrency: string }>()
const { t } = useI18n()
const emit = defineEmits<{ busy: [value: boolean] }>()
const auth = useAdminAuthStore()
const canSave = computed(() => auth.hasPermission('POST:/admin/member-level-prices/batch'))
const products = ref<AdminProduct[]>([])
const saved = ref<UserPriceInput[]>([])
const drafts = ref<UserPriceDraft[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const search = ref('')
let loadVersion = 0

const rowsForProduct = (productId: number) => drafts.value.filter((row) => row.product_id === productId)
const scopeLabel = (product: AdminProduct, skuId: number) => {
  if (!skuId) return t('admin.levelProductPrices.allSkus')
  const sku = product.skus?.find((row) => row.id === skuId)
  return Object.values(sku?.spec_values || {}).join(' / ') || sku?.sku_code || `#${skuId}`
}
const retailPrice = (product: AdminProduct, skuId: number) => {
  const amount = skuId ? product.skus?.find((row) => row.id === skuId)?.price_amount : product.price_amount
  return formatMoney(amount ?? product.price_amount, props.siteCurrency)
}
const visibleProducts = computed(() => {
  const text = search.value.trim().toLowerCase()
  if (!text) return products.value
  return products.value.filter((product) => `${Object.values(product.title).join(' ')} ${product.slug} ${product.id}`.toLowerCase().includes(text))
})
const hasChanges = computed(() => drafts.value.some((row) => {
  const original = saved.value.find((price) => price.product_id === row.product_id && price.sku_id === row.sku_id)
  return row.price.trim() !== (original ? String(original.price_amount) : '')
}))

const load = async () => {
  const version = ++loadVersion
  const levelId = props.levelId
  loading.value = true
  error.value = ''
  products.value = []
  drafts.value = []
  saved.value = []
  try {
    const [priceResponse, firstPage] = await Promise.all([
      adminAPI.getPricesForMemberLevel(levelId),
      adminAPI.getProducts({ page: 1, page_size: 100, is_active: true }),
    ])
    const allProducts: AdminProduct[] = firstPage.data.data || []
    const totalPages = firstPage.data.pagination?.total_page || 1
    for (let page = 2; page <= totalPages; page++) {
      const response = await adminAPI.getProducts({ page, page_size: 100, is_active: true })
      allProducts.push(...(response.data.data || []))
    }
    if (version !== loadVersion || levelId !== props.levelId) return
    if (!Array.isArray(priceResponse.data.data) || !Array.isArray(firstPage.data.data)) throw new Error(t('admin.levelProductPrices.loadFailed'))
    saved.value = priceResponse.data.data
    products.value = allProducts
    drafts.value = allProducts.flatMap((product) => {
      const skus = (product.skus || []).filter((sku) => sku.is_active)
      const scopes = [0, ...skus.filter((sku) => skus.length > 1 || saved.value.some((price) => price.product_id === product.id && price.sku_id === sku.id)).map((sku) => sku.id)]
      return scopes.map((skuId) => {
        const price = saved.value.find((row) => row.product_id === product.id && row.sku_id === skuId)
        return { product_id: product.id, sku_id: skuId, price: price ? String(price.price_amount) : '' }
      })
    })
  } catch (err: any) {
    if (version === loadVersion) error.value = err?.message || t('admin.levelProductPrices.loadFailed')
  } finally {
    if (version === loadVersion) loading.value = false
  }
}

const save = async () => {
  if (!canSave.value || loading.value || saving.value || !hasChanges.value) return
  let payload: ReturnType<typeof buildLevelProductPricePayload>
  error.value = ''
  try { payload = buildLevelProductPricePayload(props.levelId, saved.value, drafts.value) }
  catch { error.value = t('admin.levelProductPrices.invalid'); return }
  saving.value = true
  try {
    await adminAPI.batchUpsertMemberLevelPrices(payload)
    saved.value = payload.prices
    for (const row of drafts.value) {
      row.price = String(payload.prices.find((price) => price.product_id === row.product_id && price.sku_id === row.sku_id)?.price_amount ?? '')
    }
    notifySuccess(t('admin.levelProductPrices.saved'))
  } catch (err: any) {
    error.value = err?.message || t('admin.levelProductPrices.saveFailed')
  } finally { saving.value = false }
}

watch(() => props.levelId, load, { immediate: true })
watch(saving, (value) => emit('busy', value))
onBeforeUnmount(() => { loadVersion++ })
</script>

<template>
  <form class="space-y-4 rounded-xl border border-border bg-card p-4 sm:p-6" @submit.prevent="save">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h2 class="text-lg font-semibold">{{ t('admin.levelProductPrices.title') }}</h2>
        <p class="mt-1 text-sm leading-6 text-muted-foreground">{{ t('admin.levelProductPrices.hint') }}</p>
        <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ t('admin.levelProductPrices.scopeHint') }}</p>
      </div>
      <div class="flex shrink-0 gap-2">
        <Button type="button" variant="outline" :disabled="loading || saving" @click="load">{{ t('admin.levelProductPrices.reload') }}</Button>
        <Button type="submit" :disabled="loading || saving || !canSave || !hasChanges">{{ saving ? t('admin.levelProductPrices.saving') : t('admin.levelProductPrices.save') }}</Button>
      </div>
    </div>
    <div v-if="error" role="alert" class="rounded-lg border border-destructive/30 p-3 text-sm text-destructive">{{ error }}</div>
    <Input v-model="search" class="sm:max-w-md" :placeholder="t('admin.levelProductPrices.search')" :aria-label="t('admin.levelProductPrices.search')" />
    <p v-if="loading" class="py-8 text-center text-muted-foreground">{{ t('admin.common.loading') }}</p>
    <div v-else class="overflow-x-auto rounded-lg border border-border">
      <Table class="min-w-0 sm:min-w-[560px]">
        <TableHeader class="hidden sm:table-header-group">
          <TableRow>
            <TableHead>{{ t('admin.levelProductPrices.product') }}</TableHead>
            <TableHead class="w-32">{{ t('admin.levelProductPrices.retail') }}</TableHead>
            <TableHead class="w-56">{{ t('admin.levelProductPrices.personal', { currency: siteCurrency }) }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="block sm:table-row-group">
          <TableRow v-if="!visibleProducts.length"><TableCell colspan="3" class="py-8 text-center text-muted-foreground">{{ t('admin.levelProductPrices.empty') }}</TableCell></TableRow>
          <template v-for="product in visibleProducts" :key="product.id">
            <TableRow v-for="row in rowsForProduct(product.id)" :key="`${row.product_id}:${row.sku_id}`" class="grid gap-2 p-3 sm:table-row sm:p-0">
              <TableCell class="block max-w-md p-0 sm:table-cell sm:p-2">
                <div class="whitespace-normal break-words font-medium">{{ getLocalizedText(product.title) }}</div>
                <div class="mt-1 text-xs text-muted-foreground">#{{ product.id }} · {{ scopeLabel(product, row.sku_id) }}</div>
              </TableCell>
              <TableCell class="flex items-center justify-between gap-2 p-0 sm:table-cell sm:p-2"><span class="text-xs text-muted-foreground sm:hidden">{{ t('admin.levelProductPrices.retail') }}</span><span class="font-mono">{{ retailPrice(product, row.sku_id) }}</span></TableCell>
              <TableCell class="block p-0 sm:table-cell sm:p-2">
                <span class="mb-1 block text-xs text-muted-foreground sm:hidden">{{ t('admin.levelProductPrices.personal', { currency: siteCurrency }) }}</span>
                <div class="flex items-center gap-1">
                  <Input v-model="row.price" type="text" inputmode="decimal" autocomplete="off" :disabled="saving || !canSave" :placeholder="row.sku_id ? t('admin.levelProductPrices.inheritProduct') : t('admin.levelProductPrices.inherit')" :aria-label="`${getLocalizedText(product.title)} ${scopeLabel(product, row.sku_id)} ${t('admin.levelProductPrices.priceLabel')}`" />
                  <Button v-if="row.price" type="button" variant="ghost" size="sm" :disabled="saving || !canSave" @click="row.price = ''">{{ t('admin.levelProductPrices.clear') }}</Button>
                </div>
              </TableCell>
            </TableRow>
          </template>
        </TableBody>
      </Table>
    </div>
  </form>
</template>
