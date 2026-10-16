export type PetAction = 'idle' | 'wave' | 'blink' | 'think' | 'reply'
export type PetPose = { arm: number; armScale: number; lean: number; bob: number; stretch: number; blink: number }
export type PetKey = { at: number; pose: Partial<PetPose> }
export const neutralPose = (): PetPose => ({ arm: 0, armScale: 1, lean: 0, bob: 0, stretch: 1, blink: 0 })
export const ease = (value: number) => { const t = Math.max(0, Math.min(1, value)); return t * t * t * (t * (t * 6 - 15) + 10) }
export function mixPose(from: PetPose, to: PetPose, t: number): PetPose {
  const result = neutralPose()
  for (const key of Object.keys(result) as (keyof PetPose)[]) result[key] = from[key] + (to[key] - from[key]) * t
  return result
}
export function samplePose(keys: PetKey[], time: number): PetPose {
  for (let i = 1; i < keys.length; i++) {
    const a = keys[i - 1]!, b = keys[i]!
    if (time < b.at) return mixPose({ ...neutralPose(), ...a.pose }, { ...neutralPose(), ...b.pose }, ease((time - a.at) / (b.at - a.at)))
  }
  return { ...neutralPose(), ...keys[keys.length - 1]!.pose }
}
const key = (at: number, pose: Partial<PetPose> = {}): PetKey => ({ at, pose })
const chin = { arm: -90, armScale: .70, lean: -1.5, bob: .5 }
export function actionKeys(action: PetAction): PetKey[] {
  if (action === 'wave') return [key(0), key(230, { arm: 3, lean: .3 }), key(560, { arm: -16, lean: -1.3, bob: -1 }), key(880, { arm: 8, lean: .9 }), key(1210, { arm: -15, lean: -1.1, bob: -.7 }), key(1520, { arm: 5, lean: .6 }), key(1900)]
  if (action === 'blink') return [key(0), key(170), key(270, { blink: 1 }), key(325, { blink: 1 }), key(505), key(700)]
  if (action === 'think') return [key(0), key(700, chin), key(1470, { ...chin, arm: -88, lean: -2.6, bob: 1.2 }), key(2240, { ...chin, arm: -92, lean: .5, bob: .2 }), key(3010, { ...chin, arm: -89, lean: -1.7, bob: .7 }), key(3450, chin), key(4200)]
  if (action === 'reply') return [key(0), key(300, { lean: 1.7, bob: 2, stretch: .992 }), key(660, { lean: -.6, bob: -1.3, stretch: 1.003 }), key(1000)]
  return [key(0), key(8500)]
}
