<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { walletAPI } from '../../api'
import { amountToCents } from '../../utils/money'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

const props = defineProps<{ wallet: { balance?: string; withdrawable_balance?: string; frozen_balance?: string; earnings_debt?: string } | null }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const policy = ref<{ enabled: boolean; minimum: string; channels: string[]; blocked_reason?: string } | null>(null)
const rows = ref<any[]>([])
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const success = ref('')
const page = ref(1)
const pages = ref(1)
const form = reactive({ amount: '', channel: '', account: '', real_name: '' })
const available = computed(() => props.wallet?.withdrawable_balance || '0.00')
const debt = computed(() => (amountToCents(props.wallet?.earnings_debt || '0') || 0) > 0)
const nonCash = computed(() => ((amountToCents(props.wallet?.balance || '0') || 0) - (amountToCents(available.value) || 0)) / 100)
const ready = computed(() => !loading.value && !!props.wallet && !!policy.value?.enabled && !policy.value.blocked_reason && !debt.value)
let retryKey = ''
let retryPayload = ''

async function load(target = page.value) {
 loading.value = true
 try {
  const [settings, history] = await Promise.all([walletAPI.withdrawPolicy(), walletAPI.withdraws({ page: target, page_size: 10 })])
  policy.value = settings.data.data
  rows.value = history.data.data || []
  page.value = target
  pages.value = history.data.pagination?.total_page || 1
  if (!policy.value?.channels.includes(form.channel)) form.channel = policy.value?.channels[0] || ''
 } catch (e: any) { error.value = e?.message || t('walletEarnings.loadFailed'); policy.value = null }
 finally { loading.value = false }
}

async function submit() {
 error.value = ''; success.value = ''
 const cents = amountToCents(form.amount)
 if (!ready.value || !/^\d+(\.\d{1,2})?$/.test(form.amount) || cents === null || cents <= 0 || cents > (amountToCents(available.value) || 0) || cents < (amountToCents(policy.value?.minimum || '0') || 0) || !form.account.trim() || !form.real_name.trim()) {
  error.value = t('walletEarnings.invalid'); return
 }
 const body = { amount: form.amount, channel: form.channel, account: form.account.trim(), real_name: form.real_name.trim() }
 const serialized = JSON.stringify(body)
 if (!retryKey || serialized !== retryPayload) { retryKey = crypto.randomUUID(); retryPayload = serialized }
 busy.value = true
 try {
  await walletAPI.applyWithdraw({ ...body, request_key: retryKey })
  retryKey = ''; retryPayload = ''; form.amount = ''
  success.value = t('walletEarnings.submitted')
  emit('changed')
  await load(1)
 } catch (e: any) { error.value = e?.message || t('walletEarnings.submitFailed') }
 finally { busy.value = false }
}
onMounted(() => load())
</script>

<template>
 <section class="rounded-2xl border bg-card p-5 shadow-sm sm:p-7 space-y-5">
  <div class="flex flex-wrap items-start justify-between gap-3">
   <div><h3 class="text-lg font-bold">{{ t('walletEarnings.title') }}</h3><p class="mt-1 text-sm text-muted-foreground">{{ t('walletEarnings.rule') }}</p></div>
   <Button variant="outline" size="sm" :disabled="loading || busy" @click="error = ''; load(); emit('changed')">{{ t('orders.filters.refresh') }}</Button>
  </div>
  <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
   <div class="rounded-xl border p-4"><div class="text-xs text-muted-foreground">{{ t('walletEarnings.available') }}</div><p class="mt-2 font-mono text-lg font-bold">¥ {{ available }}</p></div>
   <div class="rounded-xl border p-4"><div class="text-xs text-muted-foreground">{{ t('walletEarnings.nonCash') }}</div><p class="mt-2 font-mono text-lg">¥ {{ nonCash.toFixed(2) }}</p></div>
   <div class="rounded-xl border p-4"><div class="text-xs text-muted-foreground">{{ t('walletEarnings.frozen') }}</div><p class="mt-2 font-mono text-lg">¥ {{ wallet?.frozen_balance || '0.00' }}</p></div>
  </div>
  <Alert v-if="error" variant="destructive"><AlertDescription>{{ error }}</AlertDescription></Alert>
  <Alert v-if="success"><AlertDescription>{{ success }}</AlertDescription></Alert>
  <Alert v-if="debt" variant="destructive"><AlertDescription>{{ t('walletEarnings.debt', { amount: wallet?.earnings_debt }) }}</AlertDescription></Alert>
  <Alert v-if="policy?.blocked_reason" variant="destructive"><AlertDescription>{{ policy.blocked_reason }}</AlertDescription></Alert>
  <p v-if="loading" class="text-sm text-muted-foreground">{{ t('walletEarnings.loading') }}</p>
  <p v-else-if="!policy?.enabled" class="text-sm text-muted-foreground">{{ t('walletEarnings.closed') }}</p>
  <form v-else class="space-y-4" @submit.prevent="submit">
   <p class="text-sm text-muted-foreground">{{ t('walletEarnings.manual', { minimum: policy.minimum }) }}</p>
   <div class="grid gap-4 sm:grid-cols-2">
    <div><Label for="withdraw-amount">{{ t('walletEarnings.amount') }}</Label><Input id="withdraw-amount" v-model.trim="form.amount" class="mt-2" inputmode="decimal" autocomplete="off" :disabled="!ready || busy" required /></div>
    <div><Label for="withdraw-channel">{{ t('walletEarnings.channel') }}</Label><select id="withdraw-channel" v-model="form.channel" class="mt-2 h-10 w-full rounded-md border bg-background px-3 text-sm" :disabled="!ready || busy" required><option v-for="channel in policy.channels" :key="channel" :value="channel">{{ t(`walletEarnings.channels.${channel}`) }}</option></select></div>
    <div><Label for="withdraw-account">{{ t('walletEarnings.account') }}</Label><Input id="withdraw-account" v-model.trim="form.account" class="mt-2" maxlength="200" autocomplete="off" :disabled="!ready || busy" required /></div>
    <div><Label for="withdraw-name">{{ t('walletEarnings.realName') }}</Label><Input id="withdraw-name" v-model.trim="form.real_name" class="mt-2" maxlength="80" autocomplete="name" :disabled="!ready || busy" required /></div>
   </div>
   <Button type="submit" :disabled="!ready || busy">{{ busy ? t('walletEarnings.submitting') : t('walletEarnings.submit') }}</Button>
  </form>
  <div class="border-t pt-5 space-y-3">
   <h4 class="font-semibold">{{ t('walletEarnings.history') }}</h4>
   <p v-if="!loading && rows.length === 0" class="text-sm text-muted-foreground">{{ t('walletEarnings.empty') }}</p>
   <article v-for="row in rows" :key="row.id" class="rounded-xl border p-4 text-sm">
    <div class="flex flex-wrap justify-between gap-2"><span class="font-mono font-bold">#{{ row.id }} · ¥ {{ row.amount }}</span><span>{{ t(`walletEarnings.status.${row.status}`) }}</span></div>
    <p class="mt-2 break-all text-muted-foreground">{{ t(`walletEarnings.channels.${row.channel}`) }} · {{ row.account }} · {{ row.real_name }}</p>
    <p class="mt-1 text-xs text-muted-foreground">{{ new Date(row.created_at).toLocaleString() }}</p>
    <p v-if="row.remark" class="mt-2 break-words">{{ row.remark }}</p>
    <p v-if="row.payout_reference" class="mt-2 break-all text-xs">{{ t('walletEarnings.reference') }}：{{ row.payout_reference }}</p>
   </article>
   <div v-if="pages > 1" class="flex items-center justify-center gap-3"><Button variant="outline" size="sm" :disabled="loading || page <= 1" @click="load(page - 1)">{{ t('orders.prevPage') }}</Button><span class="text-sm">{{ page }} / {{ pages }}</span><Button variant="outline" size="sm" :disabled="loading || page >= pages" @click="load(page + 1)">{{ t('orders.nextPage') }}</Button></div>
  </div>
 </section>
</template>
