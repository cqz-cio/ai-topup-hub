import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { createRedemptionAPI, RedemptionError, type RedemptionRecord } from '../api/redemption'
import { needsAccountCheck, normalizeRedemptionCode, shouldPoll, validSession } from '../utils/redemption'

export function useRedemption() {
  const api = createRedemptionAPI(import.meta.env.VITE_API_BASE_URL || '')
  const codeInput = ref('')
  const activeCode = ref('')
  const sessionInput = ref('')
  const authorized = ref(false)
  const confirmed = ref(false)
  const record = ref<RedemptionRecord | null>(null)
  const proof = ref<RedemptionRecord | null>(null)
  const step = ref(1)
  const busy = ref('')
  const error = ref('')
  const notice = ref('')
  const now = ref(Date.now())
  const lastRefresh = ref(0)
  const uncertain = ref(false)
  let timer: ReturnType<typeof setTimeout> | undefined
  let clock: ReturnType<typeof setInterval> | undefined
  let controller: AbortController | undefined
  let epoch = 0
  let disposed = false

  const proofExpires = computed(() => Date.parse(proof.value?.checked_until || ''))
  const proofValid = computed(() => !!proof.value?.confirmation_token && !!proof.value?.account?.account_id
    && proof.value?.account?.current_plan === 'free' && proofExpires.value > now.value)
  const canConfirm = computed(() => confirmed.value && proofValid.value && !busy.value)
  const canRefresh = computed(() => !busy.value && now.value - lastRefresh.value >= 5000)
  const maskedCode = computed(() => activeCode.value ? `${activeCode.value.slice(0, 5)}-•••••-•••••-•••••-${activeCode.value.slice(-5)}` : '')

  function stopPolling() { if (timer) clearTimeout(timer); timer = undefined }
  function clearProof() { proof.value = null; confirmed.value = false }
  function message(e: unknown) { return e instanceof RedemptionError ? e.message : '服务暂时不可用，请稍后重试。' }
  function requestSignal() { controller = new AbortController(); return controller.signal }
  function current(version: number) { return !disposed && epoch === version }
  function schedule(delay = 10000) {
    stopPolling()
    if (disposed || document.hidden || step.value !== 4 || !activeCode.value) return
    if (!uncertain.value && !shouldPoll(record.value?.state || '')) return
    timer = setTimeout(() => { void refresh(true) }, delay)
  }

  async function lookup() {
    if (busy.value) return
    const code = normalizeRedemptionCode(codeInput.value)
    if (!code) { error.value = '请输入完整卡密：5 组字母和数字，每组 5 位。'; return }
    stopPolling(); clearProof(); error.value = ''; notice.value = ''; busy.value = 'lookup'
    const version = epoch
    try {
      const value = await api.lookup(code, requestSignal())
      if (!current(version)) return
      activeCode.value = code; codeInput.value = code; record.value = value; uncertain.value = false
      sessionInput.value = ''; authorized.value = false
      step.value = needsAccountCheck(value.state) ? 2 : 4
      lastRefresh.value = Date.now()
    } catch (e) { if (current(version)) error.value = message(e) }
    finally { if (current(version)) { busy.value = ''; schedule() } }
  }

  async function checkAccount() {
    if (busy.value || !activeCode.value) return
    clearProof(); error.value = ''; notice.value = ''
    if (!authorized.value) { error.value = '请先确认你有权使用此账号，并授权核验和充值。'; return }
    if (!validSession(sessionInput.value)) { error.value = '请输入包含 accessToken 和 sessionToken 的完整 Session JSON。'; return }
    const version = epoch
    busy.value = 'account'
    try {
      const value = await api.account(activeCode.value, sessionInput.value, requestSignal())
      if (!current(version)) return
      record.value = value
      if (value.reason === 'existing_subscription_not_supported') {
        error.value = '该账号已有订阅，本卡密仅支持新开通。请核对账号或联系商城客服。'
        return
      }
      if (value.state !== 'awaiting_confirmation' || !value.account?.account_id || value.account.current_plan !== 'free'
        || !value.confirmation_token || !(Date.parse(value.checked_until || '') > Date.now())) {
        error.value = '账号信息尚未核实完整，请稍后重新核验。'
        return
      }
      proof.value = value; now.value = Date.now(); step.value = 3
    } catch (e) {
      if (current(version)) {
        error.value = message(e)
        if (e instanceof RedemptionError && e.code === 'recharge_already_started') { step.value = 4; uncertain.value = true }
      }
    } finally { if (current(version)) { sessionInput.value = ''; busy.value = ''; schedule() } }
  }

  function editAccount() {
    if (busy.value) return
    stopPolling(); clearProof(); sessionInput.value = ''; authorized.value = false
    error.value = ''; notice.value = ''; step.value = 2; uncertain.value = false
  }

  async function confirmRecharge() {
    if (!canConfirm.value || !activeCode.value || !proof.value?.confirmation_token) return
    const version = epoch
    const token = proof.value.confirmation_token
    busy.value = 'confirm'; error.value = ''; notice.value = ''
    try {
      const value = await api.confirm(activeCode.value, token, requestSignal())
      if (!current(version)) return
      record.value = value; uncertain.value = false; step.value = 4; clearProof()
    } catch (e) {
      if (current(version)) {
        // A timeout is not evidence that confirmation failed. Query the same
        // entitlement instead of replaying the write or offering another payment.
        notice.value = message(e); uncertain.value = true; step.value = 4; clearProof()
      }
    } finally {
      if (current(version)) {
        busy.value = ''
        if (uncertain.value) void refresh(true)
        else schedule()
      }
    }
  }

  async function refresh(automatic = false) {
    if (busy.value || !activeCode.value || (!automatic && !canRefresh.value)) return
    const version = epoch
    stopPolling(); busy.value = 'refresh'; lastRefresh.value = Date.now()
    try {
      const value = await api.lookup(activeCode.value, requestSignal())
      if (!current(version)) return
      record.value = value; uncertain.value = false; notice.value = ''; error.value = ''
    } catch (e) {
      if (current(version)) {
        notice.value = message(e)
        if (e instanceof RedemptionError && e.code === 'recharge_not_found') {
          record.value = { state: 'unavailable', plan: record.value?.plan || '' }; uncertain.value = false
        }
      }
    } finally { if (current(version)) { busy.value = ''; schedule(notice.value ? 30000 : 10000) } }
  }

  function reset() {
    epoch++; controller?.abort(); stopPolling(); clearProof()
    codeInput.value = ''; activeCode.value = ''; sessionInput.value = ''; authorized.value = false
    record.value = null; step.value = 1; error.value = ''; notice.value = ''; busy.value = ''; uncertain.value = false
  }
  function visibility() { if (document.hidden) stopPolling(); else schedule(1000) }
  onMounted(() => {
    clock = setInterval(() => { now.value = Date.now() }, 1000)
    document.addEventListener('visibilitychange', visibility)
    window.addEventListener('pagehide', reset)
  })
  onBeforeUnmount(() => {
    disposed = true; reset(); if (clock) clearInterval(clock)
    document.removeEventListener('visibilitychange', visibility)
    window.removeEventListener('pagehide', reset)
  })
  return { codeInput, activeCode, sessionInput, authorized, confirmed, record, proof, step, busy, error, notice,
    uncertain, proofValid, canConfirm, canRefresh, maskedCode, lookup, checkAccount, editAccount, confirmRecharge, refresh, reset }
}
