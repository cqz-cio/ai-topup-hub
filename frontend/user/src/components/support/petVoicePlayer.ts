export type VoiceState = { id: string | null; status: 'idle' | 'loading' | 'playing' | 'error' }

/** Owns one user-triggered clip. Stale play promises cannot revive stopped audio. */
export function createPetVoicePlayer(
  update: (state: VoiceState) => void,
  createAudio: () => HTMLAudioElement = () => new Audio(),
  timeoutMs = 12000,
) {
  let audio: HTMLAudioElement | undefined
  let state: VoiceState = { id: null, status: 'idle' }
  let generation = 0, disposed = false
  let timer: ReturnType<typeof setTimeout> | undefined
  function release() {
    clearTimeout(timer); timer = undefined
    if (!audio) return
    audio.onended = null; audio.onerror = null
    audio.pause()
    audio.removeAttribute('src'); audio.load()
    audio = undefined
  }
  function publish(next: VoiceState) { state = next; update(next) }
  function stop() {
    generation++; release()
    publish({ id: null, status: 'idle' })
  }
  async function toggle(id: string, src: string) {
    if (disposed) return
    if (state.id === id && (state.status === 'loading' || state.status === 'playing')) { stop(); return }
    stop()
    const ticket = generation
    const fail = () => {
      if (ticket !== generation || disposed) return
      generation++; release(); publish({ id, status: 'error' })
    }
    publish({ id, status: 'loading' })
    try {
      audio = createAudio()
      audio.preload = 'none'; audio.volume = 0.7; audio.src = src
      audio.onended = () => { if (ticket === generation) stop() }
      audio.onerror = fail
      timer = setTimeout(fail, timeoutMs)
      await audio.play()
      if (ticket !== generation || disposed) return
      clearTimeout(timer); timer = undefined
      publish({ id, status: 'playing' })
    } catch { fail() }
  }
  return { toggle, stop, dispose() { disposed = true; stop() } }
}
