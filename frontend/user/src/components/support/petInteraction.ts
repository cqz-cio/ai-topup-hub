// Independent storefront interactions; no third-party pet code or media.
export type PetSize = 'small' | 'normal' | 'large'
export type PetPosition = { x: number; y: number }
export const petStorageKey = 'pi-companion-v1'
export function readPetSettings(raw: string | null): { size: PetSize; resting: boolean; position?: PetPosition } {
  const fallback = { size: 'normal' as PetSize, resting: false }
  try {
    const value = JSON.parse(raw || '{}')
    if (!value || typeof value !== 'object') return fallback
    const p = value.position
    return { size: ['small', 'normal', 'large'].includes(value.size) ? value.size : 'normal', resting: value.resting === true,
      position: p && Number.isFinite(p.x) && Number.isFinite(p.y) && p.x >= 0 && p.x <= 1 && p.y >= 0 && p.y <= 1 ? { x: p.x, y: p.y } : undefined }
  } catch { return fallback }
}
export function savePetSettings(patch: Partial<ReturnType<typeof readPetSettings>>) {
  try { localStorage.setItem(petStorageKey, JSON.stringify({ ...readPetSettings(localStorage.getItem(petStorageKey)), ...patch })) } catch { /* Storage may be disabled. */ }
}
export function petHit(x: number, y: number): 'belly' | 'body' | 'outside' {
  if (!Number.isFinite(x) || !Number.isFinite(y) || x < .12 || x > .93 || y < .06 || y > .96) return 'outside'
  // Ellipse inside the bare belly, below the necklace and above the shorts.
  return ((x - .53) / .20) ** 2 + ((y - .64) / .075) ** 2 <= 1 ? 'belly' : 'body'
}
export function petLook(x: number, y: number) { return { x: Math.max(-1, Math.min(1, (x - .5) * 2)), y: Math.max(-1, Math.min(1, (y - .3) * 2)) } }
