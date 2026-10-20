import { ref } from 'vue'

const contactOpen = ref(false)
let trigger: HTMLElement | null = null

export function useProductContact() {
  const openContact = (event?: Event) => {
    trigger = event?.currentTarget instanceof HTMLElement ? event.currentTarget : null
    contactOpen.value = true
  }
  const closeContact = () => { contactOpen.value = false }
  const restoreFocus = (event: Event) => {
    event.preventDefault()
    if (trigger?.isConnected) trigger.focus({ preventScroll: true })
    trigger = null
  }
  return { contactOpen, openContact, closeContact, restoreFocus }
}
