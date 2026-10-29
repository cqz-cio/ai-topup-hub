<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { paiplusAPI, type PAIPLUSCard, type PAIPLUSDetail, type PAIPLUSList } from '@/api/paiplus'
import { useAdminAuthStore } from '@/stores/auth'
import { adminUrl } from '@/utils/adminBase'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogScrollContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import ListPagination from '@/components/ListPagination.vue'

const auth = useAdminAuthStore()
const permission = (action: string) => auth.hasPermission(`POST:/admin/card-secrets/paiplus/${action}`)
const query = ref(''), status = ref(''), page = ref(1), pageSize = ref(20)
const data = ref<PAIPLUSList | null>(null), loading = ref(false), error = ref(''), notice = ref('')
const open = ref(false), detail = ref<PAIPLUSDetail | null>(null), selected = ref<PAIPLUSCard | null>(null)
const detailLoading = ref(false), working = ref(false), detailError = ref(''), detailNotice = ref('')
const reason = ref(''), confirmed = ref(false)
let active = true, listVersion = 0, detailVersion = 0, timer: ReturnType<typeof setInterval> | undefined
const labels: Record<string, string> = { unused: '未使用', used: '已使用', processing: '充值处理中', revoking: '销毁核验中', revoked: '已销毁', invalid: '已失效' }
const statusName = (value: string) => labels[value] || '待核验'
const date = (value: number | null | undefined) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '尚未核验'
const totalPages = computed(() => Math.max(1, Math.ceil((data.value?.total || 0) / pageSize.value)))
const current = (version: number, token: string) => active && open.value && version === detailVersion && token === auth.token

async function load(silent = false) {
  const version = ++listVersion, token = auth.token
  if (!silent) loading.value = true
  error.value = ''
  try {
    const result = await paiplusAPI.list({ query: query.value.trim(), status: status.value, page: page.value, page_size: pageSize.value })
    if (active && version === listVersion && token === auth.token) data.value = result
  } catch (e: any) {
    if (active && version === listVersion && token === auth.token) error.value = e?.message || '读取卡密失败，请重试。'
  } finally { if (active && version === listVersion) loading.value = false }
}
function close() {
  ++detailVersion; open.value = false; detail.value = null; selected.value = null
  reason.value = ''; confirmed.value = false; detailError.value = ''; detailNotice.value = ''; working.value = false; detailLoading.value = false
}
async function show(card: PAIPLUSCard) {
  close(); open.value = true; selected.value = card; detailLoading.value = true
  const version = ++detailVersion, token = auth.token
  try {
    const value = await paiplusAPI.reveal(card.id)
    if (current(version, token)) detail.value = value
  } catch (e: any) { if (current(version, token)) detailError.value = e?.message || '读取详情失败。' }
  finally { if (current(version, token)) detailLoading.value = false }
}
async function check() {
  if (!detail.value || working.value) return
  const cardID = detail.value.id, version = detailVersion, token = auth.token
  working.value = true; detailError.value = ''; detailNotice.value = ''
  try {
    const value = await paiplusAPI.check(cardID)
    if (!current(version, token)) return
    // Status queries return masked codes. Keep the explicitly revealed pair.
    detail.value = { ...detail.value!, ...value, code: detail.value!.code, source_code: detail.value!.source_code }
    detailNotice.value = '已核对上游当前状态。'; await load(true)
  } catch (e: any) { if (current(version, token)) detailError.value = e?.message || '核验失败。' }
  finally { if (current(version, token)) working.value = false }
}
async function destroy() {
  if (!detail.value || working.value || !confirmed.value || !reason.value.trim()) return
  const cardID = detail.value.id, version = detailVersion, token = auth.token
  const pending = detail.value.operations.find(item => item.state === 'pending')
  working.value = true; detailError.value = ''; detailNotice.value = ''
  try {
    const result = await paiplusAPI.revoke(cardID, reason.value.trim(), pending?.id || crypto.randomUUID())
    if (!current(version, token)) return
    if (!result.destroyed || result.card.status !== 'revoked') throw new Error('销毁尚未确认，请刷新状态，不要据此退款。')
    detail.value = { ...detail.value!, ...result.card, code: detail.value!.code, source_code: detail.value!.source_code }
    detailNotice.value = '销毁成功：本站卡密已永久停用，客户不能再通过本站发起充值。可按退款流程处理；此操作不会自动退款或销毁上游卡密。'
    confirmed.value = false
  } catch (e: any) { if (current(version, token)) detailError.value = e?.message || '销毁未确认成功，请刷新卡密状态。' }
  finally {
    if (current(version, token)) {
      try { const value = await paiplusAPI.reveal(cardID); if (current(version, token)) detail.value = value } catch { /* Keep the explicit action result visible. */ }
      if (current(version, token)) { working.value = false; await load(true) }
    }
  }
}
async function copy(value: string) {
  try { await navigator.clipboard.writeText(value); if (open.value) detailNotice.value = '已复制。' }
  catch { if (open.value) detailError.value = '复制失败，请手动选择卡密复制。' }
}
function search() { page.value = 1; close(); void load() }
watch(() => auth.token, () => { ++listVersion; data.value = null; close() })
onMounted(() => {
  void load()
  timer = setInterval(() => { if (!document.hidden && !loading.value && !working.value) void load(true) }, 30000)
})
onUnmounted(() => { active = false; ++listVersion; close(); clearInterval(timer) })
</script>

<template>
  <section class="space-y-4" aria-label="PAIPLUS 交付卡密">
    <div class="rounded-xl border border-border bg-card p-4">
      <h2 class="font-semibold">已交付卡密与上游对应关系</h2>
      <p class="mt-1 text-sm text-muted-foreground">付款发货后自动记录到这里。“未使用”表示客户尚未兑换，不代表尚未发货。查看双码、核验和销毁均记录操作人。</p>
      <div class="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-6">
        <button v-for="key in ['unused', 'used', 'processing', 'revoking', 'revoked', 'invalid']" :key="key" type="button" class="rounded-lg border px-3 py-2 text-left" :class="status === key ? 'border-primary ring-1 ring-primary' : 'border-border'" :aria-pressed="status === key" @click="status = status === key ? '' : key; search()">
          <span class="block text-xs text-muted-foreground">{{ statusName(key) }}</span><strong class="mt-1 block text-xl">{{ data?.counts[key] ?? '—' }}</strong>
        </button>
      </div>
    </div>
    <form class="flex flex-col gap-2 sm:flex-row" @submit.prevent="search">
      <Input v-model="query" aria-label="搜索卡密或订单" placeholder="搜索订单编号、商品名称或完整卡密" autocomplete="off" class="flex-1" />
      <select v-model="status" aria-label="卡密状态" class="h-10 rounded-md border border-input bg-background px-3 text-sm" @change="search">
        <option value="">全部状态</option><option v-for="(label, key) in labels" :key="key" :value="key" v-show="key !== 'unknown'">{{ label }}</option>
      </select>
      <Button type="submit" :disabled="loading">查询</Button>
      <Button type="button" variant="outline" :disabled="loading" @click="load()">刷新列表</Button>
    </form>
    <p v-if="error" role="alert" class="rounded-lg border border-destructive/30 p-3 text-sm text-destructive">{{ error }}</p>
    <p v-if="notice" role="status" class="text-sm">{{ notice }}</p>
    <div class="overflow-x-auto rounded-xl border border-border bg-card">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-border"><tr><th class="p-3 font-medium">订单 / 商品</th><th class="p-3 font-medium">本站卡密 / 上游卡密</th><th class="p-3 font-medium">兑换状态</th><th class="p-3 font-medium">最近核验</th><th class="p-3 text-right font-medium">操作</th></tr></thead>
        <tbody>
          <tr v-if="!data && loading"><td colspan="5" class="p-8 text-center text-muted-foreground">正在读取卡密…</td></tr>
          <tr v-else-if="!data?.cards.length"><td colspan="5" class="p-8 text-center text-muted-foreground">暂无符合条件的 PAIPLUS 卡密。新订单完成自动发货后会在这里出现。</td></tr>
          <tr v-for="card in data?.cards" :key="card.id" class="border-b border-border last:border-0">
            <td class="min-w-48 p-3"><a v-if="card.order_id" :href="adminUrl(`/orders?order_id=${card.order_id}`)" class="font-medium underline underline-offset-4">{{ card.order_no || `订单 #${card.order_id}` }}</a><span v-else>手动导入</span><div class="mt-1 text-xs text-muted-foreground">{{ card.label || '历史记录：查看订单核对商品' }}</div></td>
            <td class="p-3 font-mono text-xs"><div class="whitespace-nowrap">{{ card.code }}</div><div class="mt-2 text-muted-foreground">上游：{{ card.source_code }}</div></td>
            <td class="whitespace-nowrap p-3"><span class="rounded-full border px-2 py-1 text-xs">{{ statusName(card.status) }}</span><p v-if="card.local_status === 'disabled'" class="mt-2 text-xs text-muted-foreground">已停用</p></td>
            <td class="whitespace-nowrap p-3 text-xs text-muted-foreground">{{ date(card.upstream_checked_at) }}</td>
            <td class="p-3 text-right"><Button v-if="permission('reveal')" variant="outline" size="sm" @click="show(card)">查看双码与管理</Button><span v-else class="text-xs text-muted-foreground">无双码查看权限</span></td>
          </tr>
        </tbody>
      </table>
    </div>
    <ListPagination :page="page" :total-page="totalPages" :total="data?.total || 0" :page-size="pageSize" :page-size-options="[20, 50, 100]" @change-page="page = $event; load()" @change-page-size="pageSize = $event; search()" />
    <p class="text-xs text-muted-foreground">状态在兑换结果返回后更新，并定期向上游核对。退款前请执行“检查并销毁”，以当次核验结果为准。</p>

    <Dialog :open="open" @update:open="value => { if (!value) close() }">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-3xl p-4 sm:p-6">
        <DialogHeader><DialogTitle>卡密对应关系与退款前销毁</DialogTitle><DialogDescription>两张卡密仅在此管理详情中显示。客户交付仍只包含 PAIPLUS 卡密。</DialogDescription></DialogHeader>
        <p v-if="detailLoading" class="py-8 text-center">正在读取双码详情…</p>
        <div v-if="detail" class="space-y-5">
          <div class="flex flex-wrap items-center justify-between gap-2"><div><strong>{{ detail.order_no || (detail.order_id ? `订单 #${detail.order_id}` : '手动导入卡密') }}</strong><p class="mt-1 text-sm text-muted-foreground">{{ detail.label }}</p></div><span class="rounded-full border px-3 py-1 text-sm">{{ statusName(detail.status) }}</span></div>
          <div class="space-y-3 rounded-xl border border-border p-4">
            <div v-for="item in [{ name: '本站 PAIPLUS 卡密', value: detail.code }, { name: '对应上游卡密', value: detail.source_code }]" :key="item.name"><div class="mb-1 flex items-center justify-between gap-2"><label class="text-xs text-muted-foreground">{{ item.name }}</label><Button type="button" size="sm" variant="ghost" @click="copy(item.value)">复制</Button></div><code class="block select-all break-all rounded-md border border-border p-3 text-sm">{{ item.value }}</code></div>
            <p class="break-all text-xs text-muted-foreground">绑定编号：{{ detail.reference }}</p>
          </div>
          <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border p-4"><div class="text-sm"><p>上游状态：{{ detail.upstream ? (detail.upstream.used ? '已使用' : detail.upstream.status === 'unused' && detail.upstream.can_resubmit ? '未使用，可兑换' : '需核对，不能直接退款') : '尚未核验' }}</p><p class="mt-1 text-xs text-muted-foreground">最近核验：{{ date(detail.upstream_checked_at) }}</p></div><Button v-if="permission('check')" type="button" variant="outline" :disabled="working" @click="check">核验上游状态</Button></div>
          <div v-if="detail.status === 'revoked'" class="rounded-xl border border-border p-4 text-sm"><strong>本站卡密已永久销毁</strong><p class="mt-1">销毁时间：{{ date(detail.revoked_at) }}。原对应关系保留用于核账和向上游申请退款，不能重新启用。</p></div>
          <form v-else-if="permission('revoke')" class="space-y-3 rounded-xl border border-destructive/30 p-4" @submit.prevent="destroy">
            <h3 class="font-semibold">退款前检查并销毁</h3><p class="text-sm text-muted-foreground">先锁定本站卡密并实时检查上游；只有确认未使用且可兑换，才永久断开兑换能力。已使用、充值中或无法确认状态时销毁失败。</p>
            <label for="paiplus-revoke-reason" class="block text-sm">销毁原因</label><Textarea id="paiplus-revoke-reason" v-model="reason" maxlength="500" placeholder="例如：客户申请未使用卡密退款" :disabled="working" />
            <label class="flex items-start gap-2 text-sm"><Checkbox :model-value="confirmed" :disabled="working" @update:model-value="confirmed = $event === true" /><span>确认永久停用本站卡密，并保留上游卡密及对应关系用于退款核账。</span></label>
            <Button type="submit" variant="destructive" :disabled="working || !confirmed || !reason.trim()">{{ working ? '正在核验，请勿安排退款…' : detail.status === 'revoking' ? '继续核验销毁结果' : '检查并销毁本站卡密' }}</Button>
          </form>
          <div v-if="detail.operations.length" class="space-y-2"><h3 class="text-sm font-semibold">销毁记录</h3><div v-for="operation in detail.operations" :key="operation.id" class="rounded-lg border border-border p-3 text-xs"><p>{{ date(operation.created_at) }} · {{ operation.actor }} · {{ operation.state === 'succeeded' ? '销毁成功' : operation.state === 'failed' ? '销毁失败' : '核验中' }}</p><p class="mt-1 break-words">{{ operation.reason }}</p><p v-if="operation.message" class="mt-1 text-destructive">{{ operation.message }}</p></div></div>
        </div>
        <p v-if="detailError" role="alert" class="rounded-lg border border-destructive/30 p-3 text-sm text-destructive">{{ detailError }}</p>
        <p v-if="detailNotice" role="status" class="rounded-lg border border-border p-3 text-sm">{{ detailNotice }}</p>
        <Button type="button" variant="outline" @click="close">关闭并隐藏卡密</Button>
      </DialogScrollContent>
    </Dialog>
  </section>
</template>
