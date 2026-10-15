<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAdminAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import ComplianceGuardWrapper from '@/components/ComplianceGuardWrapper.vue'
import { notifyError, notifySuccess } from '@/utils/notify'

const { t } = useI18n()
const auth = useAdminAuthStore()
const canReview = computed(() => auth.hasPermission('POST:/admin/wallet/withdraws/:id/review'))
const rows = ref<any[]>([])
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const status = ref('')
const userID = ref('')
const page = ref(1)
const pages = ref(1)
const selected = ref<any>(null)
const open = ref(false)
const form = reactive({ action: '', remark: '', payout_reference: '' })
async function load(target = page.value) {
 loading.value = true; error.value = ''
 try {
  const res = await adminAPI.getWalletWithdraws({ page: target, page_size: 20, status: status.value || undefined, user_id: userID.value || undefined })
  rows.value = res.data.data || []; page.value = target; pages.value = res.data.pagination?.total_page || 1
 } catch (e: any) { error.value = e?.message || t('walletEarnings.loadFailed') }
 finally { loading.value = false }
}
function choose(row: any, action: string) { selected.value = row; form.action = action; form.remark = ''; form.payout_reference = ''; open.value = true }
async function review() {
 if (!selected.value || busy.value) return
 if ((form.action === 'paid' && !form.payout_reference.trim()) || (form.action === 'reject' && !form.remark.trim())) { notifyError(t('walletEarnings.required')); return }
 busy.value = true
 try {
  await adminAPI.reviewWalletWithdraw(selected.value.id, { action: form.action, remark: form.remark.trim(), payout_reference: form.payout_reference.trim() })
  notifySuccess(t('walletEarnings.saved')); open.value = false; await load()
 } catch (e: any) { notifyError(e?.message || t('walletEarnings.submitFailed')) }
 finally { busy.value = false }
}
onMounted(() => load())
</script>

<template>
 <ComplianceGuardWrapper>
  <div class="space-y-6">
   <div><h1 class="text-2xl font-semibold">{{ t('walletEarnings.adminTitle') }}</h1><p class="mt-2 text-sm text-muted-foreground">{{ t('walletEarnings.adminHint') }}</p></div>
   <form class="flex flex-wrap items-center gap-3" @submit.prevent="load(1)">
    <Label for="wallet-withdraw-user">{{ t('walletEarnings.userID') }}</Label><Input id="wallet-withdraw-user" v-model="userID" inputmode="numeric" class="w-32" />
    <Label for="wallet-withdraw-status">{{ t('walletEarnings.state') }}</Label><select id="wallet-withdraw-status" v-model="status" class="h-9 rounded-md border bg-background px-3 text-sm"><option value="">{{ t('walletEarnings.all') }}</option><option v-for="s in ['pending', 'approved', 'rejected', 'paid']" :key="s" :value="s">{{ t(`walletEarnings.status.${s}`) }}</option></select>
    <Button type="submit" variant="outline" :disabled="loading">{{ t('walletEarnings.refresh') }}</Button>
   </form>
   <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
   <p v-if="loading" class="text-muted-foreground">{{ t('walletEarnings.loading') }}</p>
   <p v-else-if="!rows.length" class="rounded-xl border p-8 text-center text-muted-foreground">{{ t('walletEarnings.empty') }}</p>
   <div class="space-y-4">
    <article v-for="row in rows" :key="row.id" class="rounded-xl border bg-card p-5 space-y-4">
     <div class="flex flex-wrap justify-between gap-3"><div class="font-semibold">#{{ row.id }} · {{ t('walletEarnings.userID') }} {{ row.user_id }} · ¥ {{ row.amount }}</div><span class="rounded-full border px-3 py-1 text-xs">{{ t(`walletEarnings.status.${row.status}`) }}</span></div>
     <dl class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-3">
      <div><dt class="text-muted-foreground">{{ t('walletEarnings.channel') }}</dt><dd>{{ t(`walletEarnings.channels.${row.channel}`) }}</dd></div>
      <div><dt class="text-muted-foreground">{{ t('walletEarnings.account') }}</dt><dd class="break-all">{{ row.account }}</dd></div>
      <div><dt class="text-muted-foreground">{{ t('walletEarnings.realName') }}</dt><dd class="break-all">{{ row.real_name }}</dd></div>
     </dl>
     <p class="text-xs text-muted-foreground">{{ new Date(row.created_at).toLocaleString() }}</p>
     <p v-if="Number(row.wallet?.earnings_debt) > 0" class="text-sm text-destructive">{{ t('walletEarnings.debt', { amount: row.wallet.earnings_debt }) }}</p>
     <p v-if="row.remark" class="break-words text-sm">{{ row.remark }}</p>
     <p v-if="row.payout_reference" class="break-all text-sm">{{ t('walletEarnings.reference') }}：{{ row.payout_reference }}</p>
     <div v-if="canReview && ['pending', 'approved'].includes(row.status)" class="flex flex-wrap gap-3">
      <Button v-if="row.status === 'pending'" :disabled="busy || Number(row.wallet?.earnings_debt) > 0" @click="choose(row, 'approve')">{{ t('walletEarnings.actions.approve') }}</Button>
      <Button v-if="row.status === 'approved'" :disabled="busy || Number(row.wallet?.earnings_debt) > 0" @click="choose(row, 'paid')">{{ t('walletEarnings.actions.paid') }}</Button>
      <Button variant="outline" :disabled="busy" @click="choose(row, 'reject')">{{ t('walletEarnings.actions.reject') }}</Button>
     </div>
    </article>
   </div>
   <div v-if="pages > 1" class="flex justify-center items-center gap-3"><Button variant="outline" :disabled="loading || page <= 1" @click="load(page - 1)">{{ t('walletEarnings.prev') }}</Button><span>{{ page }} / {{ pages }}</span><Button variant="outline" :disabled="loading || page >= pages" @click="load(page + 1)">{{ t('walletEarnings.next') }}</Button></div>
  </div>
  <Dialog :open="open" @update:open="value => { if (!busy) open = value }">
   <DialogContent v-if="selected" class="sm:max-w-lg" @interact-outside="event => { if (busy) event.preventDefault() }">
    <DialogHeader><DialogTitle>{{ t(`walletEarnings.actions.${form.action}`) }} · #{{ selected.id }}</DialogTitle><DialogDescription>{{ t(form.action === 'paid' ? 'walletEarnings.paidWarning' : 'walletEarnings.reviewHint') }}</DialogDescription></DialogHeader>
    <p class="rounded-lg border p-3 text-sm break-all">¥ {{ selected.amount }} · {{ t(`walletEarnings.channels.${selected.channel}`) }} · {{ selected.account }} · {{ selected.real_name }}</p>
    <div v-if="form.action === 'paid'"><Label for="payout-ref">{{ t('walletEarnings.reference') }}</Label><Input id="payout-ref" v-model="form.payout_reference" class="mt-2" maxlength="120" :disabled="busy" /></div>
    <div><Label for="withdraw-remark">{{ t('walletEarnings.remark') }}</Label><Input id="withdraw-remark" v-model="form.remark" class="mt-2" maxlength="500" :disabled="busy" /></div>
    <DialogFooter><Button variant="outline" :disabled="busy" @click="open = false">{{ t('walletEarnings.cancel') }}</Button><Button :disabled="busy" @click="review">{{ busy ? t('walletEarnings.submitting') : t('walletEarnings.confirm') }}</Button></DialogFooter>
   </DialogContent>
  </Dialog>
 </ComplianceGuardWrapper>
</template>
