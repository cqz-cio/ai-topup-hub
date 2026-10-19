export type DeliveryAccessState<T> = Record<string, { loading: boolean; failed: boolean; data?: T }>

// Sensitive content lives only in this short-lived view state. A hide, navigation
// or unmount invalidates outstanding responses as well as clearing loaded data.
export function createDeliveryAccess<T>(load: (key: string) => Promise<T>, publish: (state: DeliveryAccessState<T>) => void) {
  let state: DeliveryAccessState<T> = {}
  let generation = 0
  let disposed = false
  const requests = new Map<string, object>()
  const update = () => publish({ ...state })
  const hide = (key: string) => {
    requests.delete(key)
    delete state[key]
    update()
  }
  const clear = () => {
    generation++
    requests.clear()
    state = {}
    update()
  }
  const reveal = async (key: string) => {
    if (disposed || state[key]?.loading || state[key]?.data) return
    const request = {}
    const version = generation
    requests.set(key, request)
    state[key] = { loading: true, failed: false }
    update()
    const current = () => !disposed && generation === version && requests.get(key) === request
    try {
      const data = await load(key)
      if (current()) state[key] = { loading: false, failed: false, data }
    } catch {
      if (current()) state[key] = { loading: false, failed: true }
    } finally {
      if (current()) { requests.delete(key); update() }
    }
  }
  return { reveal, hide, clear, dispose: () => { disposed = true; clear() } }
}
