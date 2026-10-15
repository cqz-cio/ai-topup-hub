import { ref } from 'vue'
import { readPaymentBlock, type PaymentBlock } from '../utils/paymentBlock'

const open = ref(false)
const block = ref<PaymentBlock | null>(null)
let generation = 0
let timer: ReturnType<typeof setTimeout> | undefined
let trigger: HTMLElement | null = null

export function rememberPaymentBlockFocus() {
  trigger = document.activeElement instanceof HTMLElement && document.activeElement !== document.body
    ? document.activeElement : null
}

function restoreFocus(event: Event) {
  event.preventDefault()
  if (trigger?.isConnected) trigger.focus()
}

export function closePaymentBlock() {
  open.value = false
  generation++
  clearTimeout(timer)
}

export function showPaymentBlock(value: unknown, readStatus?: (noticeID?: number) => Promise<string>): boolean {
  const parsed = readPaymentBlock(value)
  if (!parsed) return false
  closePaymentBlock()
  block.value = parsed
  open.value = true
  const current = generation
  let attempts = 0
  const poll = async () => {
    if (current !== generation || !open.value || !readStatus) return
    try {
      const status = await readStatus(parsed.notice_id)
      if (current !== generation || !open.value) return
      if (['sent', 'partial', 'failed', 'not_sent'].includes(status)) {
        block.value = { ...parsed, notification_status: status as PaymentBlock['notification_status'] }
        return
      }
    } catch { /* Keep an explicit unconfirmed state; never invent delivery. */ }
    if (++attempts < 20 && current === generation && open.value) timer = setTimeout(poll, 3000)
  }
  if (parsed.notification_status === 'pending' && readStatus) timer = setTimeout(poll, 1500)
  return true
}

export function usePaymentBlock() { return { open, block, close: closePaymentBlock, restoreFocus } }
