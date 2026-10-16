import { computed, onBeforeUnmount, onMounted, ref, watch, type CSSProperties, type Ref } from 'vue'

type Point = { x: number; y: number }
type Box = { x: number; y: number; width: number; height: number }
const gap = 12
const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(value, Math.max(min, max)))
const overlaps = (a: Box, b: Box) => a.x < b.x + b.width && a.x + a.width > b.x && a.y < b.y + b.height && a.y + a.height > b.y

// Only the launcher starts a drag. The pet's animated pose remains independent.
export function useSupportDrag(widget: Ref<HTMLElement | undefined>, launcher: Ref<HTMLButtonElement | undefined>, panel: Ref<HTMLElement | undefined>, speech: Ref<HTMLElement | undefined>) {
  const position = ref<Point>()
  const dragging = ref(false)
  const panelStyle = ref<CSSProperties>()
  const speechStyle = ref<CSSProperties>()
  const speechSide = ref('left')
  const panelOverlap = ref(false)
  let gesture: { id: number; pointer: Point; origin: Point } | undefined
  let suppressClick = false
  let layoutFrame = 0
  let observer: ResizeObserver | undefined

  const widgetStyle = computed<CSSProperties | undefined>(() => position.value ? {
    left: `${position.value.x}px`, top: `${position.value.y}px`, right: 'auto', bottom: 'auto',
  } : undefined)

  function viewport() {
    const visual = window.visualViewport
    return {
      x: visual?.offsetLeft ?? 0, y: visual?.offsetTop ?? 0,
      width: Math.min(document.documentElement.clientWidth, visual?.width ?? innerWidth),
      height: Math.min(innerHeight, visual?.height ?? innerHeight),
    }
  }

  function bounded(point: Point): Point {
    const view = viewport(), rect = widget.value!.getBoundingClientRect()
    return {
      x: clamp(point.x, view.x + gap, view.x + view.width - rect.width - gap),
      y: clamp(point.y, view.y + gap, view.y + view.height - rect.height - gap),
    }
  }

  function fixed(box: Box): CSSProperties {
    return { position: 'fixed', left: `${box.x}px`, top: `${box.y}px`, right: 'auto', bottom: 'auto' }
  }

  function place() {
    layoutFrame = 0
    if (!position.value || !widget.value) return
    const view = viewport(), pet = widget.value.getBoundingClientRect()
    const left = view.x + gap, top = view.y + gap
    const right = view.x + view.width - gap, bottom = view.y + view.height - gap
    let windowBox: Box | undefined
    panelOverlap.value = false
    if (panel.value) {
      const mobile = matchMedia('(max-width: 480px)').matches
      const width = mobile ? view.width - 2 * gap : Math.min(390, view.width - 2 * gap)
      const height = Math.min(mobile ? 494 : 500, view.height - 2 * gap)
      let x = clamp(pet.right - width, left, right - width), y = pet.y - height - gap
      if (y < top) {
        if (pet.bottom + gap + height <= bottom) y = pet.bottom + gap
        else {
          // When neither vertical side fits, place beside the pet if possible.
          const roomLeft = pet.left - gap - left, roomRight = right - pet.right - gap
          if (roomLeft >= width || roomRight >= width) x = roomLeft >= roomRight ? pet.left - gap - width : pet.right + gap
          y = clamp(pet.y + pet.height / 2 - height / 2, top, bottom - height)
        }
      }
      windowBox = { x, y, width, height }
      panelStyle.value = { ...fixed(windowBox), width: `${width}px`, height: `${height}px`, maxWidth: 'none', maxHeight: 'none' }
      // On very narrow screens, keep the dialogue usable and its close button clear.
      panelOverlap.value = overlaps(windowBox, pet)
    } else panelStyle.value = undefined
    if (speech.value) {
      const { width, height } = speech.value.getBoundingClientRect()
      let x = pet.left - 8 - width
      speechSide.value = 'left'
      if (x < left) { x = pet.right + 8; speechSide.value = 'right' }
      const box = { x: clamp(x, left, right - width), y: clamp(pet.bottom - (pet.width <= 110 ? 58 : 66) - height, top, bottom - height), width, height }
      speechStyle.value = { ...fixed(box), visibility: windowBox && overlaps(windowBox, box) ? 'hidden' : undefined }
    } else speechStyle.value = undefined
  }

  function schedulePlace() {
    if (!layoutFrame) layoutFrame = requestAnimationFrame(place)
  }

  function keepVisible() {
    finish()
    if (position.value && widget.value) position.value = bounded(position.value)
    schedulePlace()
  }

  function pointerDown(event: PointerEvent) {
    if (!event.isPrimary || event.button !== 0 || gesture || !widget.value) return
    suppressClick = false
    if (event.pointerType === 'touch') return
    const rect = widget.value.getBoundingClientRect()
    gesture = { id: event.pointerId, pointer: { x: event.clientX, y: event.clientY }, origin: { x: rect.x, y: rect.y } }
    launcher.value?.setPointerCapture(event.pointerId)
  }

  function pointerMove(event: PointerEvent) {
    if (!gesture || event.pointerId !== gesture.id) return
    const dx = event.clientX - gesture.pointer.x, dy = event.clientY - gesture.pointer.y
    // Small hand movement remains an ordinary click.
    if (!dragging.value && Math.hypot(dx, dy) < 6) return
    dragging.value = true
    suppressClick = true
    position.value = bounded({ x: gesture.origin.x + dx, y: gesture.origin.y + dy })
  }

  function finish(event?: PointerEvent) {
    if (!gesture || (event && event.pointerId !== gesture.id)) return
    const id = gesture.id
    gesture = undefined
    dragging.value = false
    // Pointerup releases capture natively; cancellation and blur need explicit cleanup.
    if (event?.type !== 'pointerup' && launcher.value?.hasPointerCapture(id)) launcher.value.releasePointerCapture(id)
  }

  function allowClick(event: MouseEvent) {
    // Keyboard activation has detail 0 and must work even after a cancelled drag.
    if (suppressClick && event.detail !== 0) { event.preventDefault(); event.stopPropagation(); return false }
    return true
  }

  function cancel() { finish() }

  watch(position, schedulePlace, { flush: 'post' })
  watch([panel, speech], ([nextPanel, nextSpeech], [oldPanel, oldSpeech]) => {
    for (const element of [oldPanel, oldSpeech]) if (element) observer?.unobserve(element)
    for (const element of [nextPanel, nextSpeech]) if (element) observer?.observe(element)
    // Position new elements before the first paint instead of flashing at the default corner.
    place()
  }, { flush: 'post' })
  onMounted(() => {
    observer = new ResizeObserver(schedulePlace)
    for (const element of [widget.value, panel.value, speech.value]) if (element) observer.observe(element)
    window.addEventListener('resize', keepVisible)
    window.addEventListener('blur', cancel)
    window.visualViewport?.addEventListener('resize', keepVisible)
    window.visualViewport?.addEventListener('scroll', keepVisible)
  })
  onBeforeUnmount(() => {
    finish()
    observer?.disconnect()
    cancelAnimationFrame(layoutFrame)
    window.removeEventListener('resize', keepVisible)
    window.removeEventListener('blur', cancel)
    window.visualViewport?.removeEventListener('resize', keepVisible)
    window.visualViewport?.removeEventListener('scroll', keepVisible)
  })

  return { widgetStyle, panelStyle, speechStyle, speechSide, panelOverlap, dragging, pointerDown, pointerMove, finish, allowClick }
}
