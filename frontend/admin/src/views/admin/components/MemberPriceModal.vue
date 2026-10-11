<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { adminAPI } from '@/api/admin'
import type { AdminMemberLevel, AdminProduct, AdminMemberLevelPrice } from '@/api/types'
import { useAdminAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog, DialogScrollContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { formatMoney, getLocalizedText } from '@/utils/format'
import { notifySuccess } from '@/utils/notify'
import { buildMemberPricePayload, createMemberPriceRows, type MemberPriceRow } from '@/utils/memberPricing'

const props = defineProps<{ modelValue: boolean; productId: number | null; siteCurrency: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; success: []; editProduct: [id: number] }>()
const { t } = useI18n()
const auth = useAdminAuthStore()
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const product = ref<AdminProduct | null>(null)
const levels = ref<AdminMemberLevel[]>([])
const rows = ref<MemberPriceRow[]>([])
const canSave = computed(() => auth.hasPermission('POST:/admin/member-level-prices/batch'))
const skus = computed(() => product.value?.skus || [])
const scopes = computed(() => [
  { id: 0, label: t('admin.memberPrices.allSkus'), price: product.value?.price_amount || 0 },
  ...skus.value.map((sku) => ({ id: sku.id, label: getLocalizedText(sku.spec_values) || sku.sku_code || `#${sku.id}`, price: sku.price_amount })),
])
let loadId = 0

async function loadLevels() {
  const result: AdminMemberLevel[] = []
  let totalPage = 1
  for (let page = 1; page <= totalPage; page++) {
    const response = await adminAPI.getMemberLevels({ page, page_size: 100 })
    if (!Array.isArray(response.data.data)) throw new Error(t('admin.memberPrices.loadFailed'))
    result.push(...response.data.data)
    totalPage = Math.max(1, Number(response.data.pagination?.total_page || 1))
  }
  return result
}

watch(() => [props.modelValue, props.productId], async () => {
  const id = ++loadId
  if (!props.modelValue || !props.productId) return
  loading.value = true
  product.value = null
  rows.value = []
  levels.value = []
  error.value = ''
  try {
    const [productResponse, priceResponse, levelRows] = await Promise.all([
      adminAPI.getProduct(props.productId), adminAPI.getMemberLevelPrices(props.productId), loadLevels(),
    ])
    if (id !== loadId) return
    product.value = productResponse.data.data
    if (!product.value || !Array.isArray(priceResponse.data.data)) throw new Error(t('admin.memberPrices.loadFailed'))
    levels.value = levelRows
    rows.value = createMemberPriceRows(levelRows, skus.value, priceResponse.data.data as AdminMemberLevelPrice[])
  } catch (err: any) {
    if (id === loadId) error.value = err?.message || t('admin.memberPrices.loadFailed')
  } finally {
    if (id === loadId) loading.value = false
  }
}, { immediate: true })

function rowsForLevel(id: number) { return rows.value.filter((row) => row.member_level_id === id) }
function scopeLabel(id: number) { return scopes.value.find((scope) => scope.id === id)?.label || `#${id}` }
function close() { if (!saving.value) emit('update:modelValue', false) }
async function save() {
  if (!product.value || !canSave.value || saving.value || loading.value) return
  let payload
  try { payload = buildMemberPricePayload(product.value.id, rows.value) }
  catch { error.value = t('admin.memberPrices.invalidPrice'); return }
  saving.value = true
  error.value = ''
  try {
    await adminAPI.batchUpsertMemberLevelPrices(payload)
    notifySuccess(t('admin.memberPrices.saved'))
    emit('success')
    emit('update:modelValue', false)
  } catch (err: any) {
    error.value = err?.message || t('admin.memberPrices.saveFailed')
  } finally { saving.value = false }
}
</script>

<template>
  <Dialog :open="modelValue" @update:open="close">
    <DialogScrollContent class="max-w-3xl" @escape-key-down="saving && $event.preventDefault()" @pointer-down-outside="saving && $event.preventDefault()">
      <DialogHeader><DialogTitle>{{ t('admin.memberPrices.title') }}</DialogTitle></DialogHeader>
      <p v-if="loading" class="py-8 text-center text-muted-foreground">{{ t('admin.common.loading') }}</p>
      <div v-if="error" role="alert" class="rounded-md border border-destructive/30 p-3 text-sm text-destructive">{{ error }}</div>
      <form v-if="product && !loading" class="space-y-5" @submit.prevent="save">
        <div>
          <h3 class="font-semibold">{{ getLocalizedText(product.title) }}</h3>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.memberPrices.hint') }}</p>
        </div>
        <section class="rounded-lg border border-border p-4">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h4 class="font-medium">{{ t('admin.memberPrices.retail') }}</h4>
            <Button type="button" variant="outline" size="sm" :disabled="saving" @click="emit('editProduct', product.id)">{{ t('admin.memberPrices.editRetail') }}</Button>
          </div>
          <dl class="mt-3 space-y-2 text-sm">
            <div v-for="scope in (skus.length ? scopes.slice(1) : scopes.slice(0, 1))" :key="scope.id" class="flex justify-between gap-4">
              <dt class="break-words">{{ scope.label }}</dt><dd class="shrink-0 font-mono">{{ formatMoney(scope.price, siteCurrency) }}</dd>
            </div>
          </dl>
        </section>
        <p v-if="!levels.length" class="text-sm text-muted-foreground">{{ t('admin.memberPrices.noLevels') }}</p>
        <fieldset v-for="level in levels" :key="level.id" :disabled="saving || !canSave" class="rounded-lg border border-border p-4">
          <legend class="px-2 font-semibold">{{ getLocalizedText(level.name) }} <span v-if="level.is_default" class="text-xs font-normal text-muted-foreground">{{ t('admin.memberPrices.default') }}</span></legend>
          <p class="mb-3 text-xs text-muted-foreground">{{ level.is_active ? t('admin.memberPrices.rate', { rate: level.discount_rate }) : t('admin.memberPrices.inactive') }}</p>
          <div class="grid gap-3 sm:grid-cols-2">
            <div v-for="row in rowsForLevel(level.id)" :key="row.sku_id">
              <label :for="`member-price-${level.id}-${row.sku_id}`" class="mb-1 block text-sm">{{ scopeLabel(row.sku_id) }}</label>
              <Input :id="`member-price-${level.id}-${row.sku_id}`" v-model="row.price" type="text" inputmode="decimal" :placeholder="t('admin.memberPrices.inherit')" :aria-label="`${getLocalizedText(level.name)} ${scopeLabel(row.sku_id)}`" />
            </div>
          </div>
        </fieldset>
        <p class="text-xs leading-5 text-muted-foreground">{{ t('admin.memberPrices.precedence') }}</p>
        <div class="flex flex-wrap items-center justify-between gap-3 border-t border-border pt-4">
          <RouterLink to="/member-levels" class="text-sm underline" @click="close">{{ t('admin.memberPrices.manageLevels') }}</RouterLink>
          <div class="flex gap-2">
            <Button type="button" variant="outline" :disabled="saving" @click="close">{{ t('admin.common.cancel') }}</Button>
            <Button type="submit" :disabled="saving || !canSave || !levels.length">{{ saving ? t('admin.common.loading') : t('admin.common.save') }}</Button>
          </div>
        </div>
      </form>
    </DialogScrollContent>
  </Dialog>
</template>
