export type PetAction = 'idle' | 'wave' | 'blink' | 'think' | 'reply' | 'poke' | 'laugh' | 'land' | 'look' | 'stretch' | 'shuffle' | 'bounce'
export type PetPose = { arm: number; armScale: number; lean: number; bob: number; stretch: number; blink: number; belly: number; lookX: number; lookY: number; shift: number; yaw: number; pitch: number; head: number; leftFoot: number; rightFoot: number; leftArm: number }
export type PetKey = { at: number; pose: Partial<PetPose> }
export const neutralPose = (): PetPose => ({ arm: 0, armScale: 1, lean: 0, bob: 0, stretch: 1, blink: 0, belly: 0, lookX: 0, lookY: 0, shift: 0, yaw: 0, pitch: 0, head: 0, leftFoot: 0, rightFoot: 0, leftArm: 0 })
export const ease = (value: number) => { const t = Math.max(0, Math.min(1, value)); return t * t * t * (t * (t * 6 - 15) + 10) }
export function mixPose(from: PetPose, to: PetPose, t: number): PetPose {
  const result = neutralPose()
  for (const key of Object.keys(result) as (keyof PetPose)[]) result[key] = from[key] + (to[key] - from[key]) * t
  return result
}
// Shape-preserving Hermite tangents: continuous velocity through a gesture,
// zero velocity at its endpoints, and no overshoot at reversals.
function tangent(keys: PetKey[], index: number, field: keyof PetPose): number {
  if (index === 0 || index === keys.length - 1) return 0
  const a = keys[index - 1]!, b = keys[index]!, c = keys[index + 1]!
  const base = neutralPose()[field], h0 = b.at - a.at, h1 = c.at - b.at
  const d0 = ((b.pose[field] ?? base) - (a.pose[field] ?? base)) / h0
  const d1 = ((c.pose[field] ?? base) - (b.pose[field] ?? base)) / h1
  if (d0 * d1 <= 0) return 0
  return 3 * (h0 + h1) / ((2 * h1 + h0) / d0 + (h1 + 2 * h0) / d1)
}
export function samplePose(keys: PetKey[], time: number): PetPose {
  for (let i = 1; i < keys.length; i++) {
    const a = keys[i - 1]!, b = keys[i]!
    if (time < b.at) {
      const h = b.at - a.at, t = Math.max(0, (time - a.at) / h), t2 = t * t, t3 = t2 * t
      const pose = neutralPose()
      for (const field of Object.keys(pose) as (keyof PetPose)[]) {
        const v0 = a.pose[field] ?? pose[field], v1 = b.pose[field] ?? pose[field]
        const v = (2*t3-3*t2+1)*v0 + (t3-2*t2+t)*h*tangent(keys,i-1,field)
          + (-2*t3+3*t2)*v1 + (t3-t2)*h*tangent(keys,i,field)
        pose[field] = Math.max(Math.min(v0,v1), Math.min(Math.max(v0,v1),v))
      }
      return pose
    }
  }
  return { ...neutralPose(), ...keys[keys.length - 1]!.pose }
}

const idleGestures: PetAction[] = ['look', 'stretch', 'shuffle', 'bounce']
export function nextIdleGesture(previous: PetAction, random: number): PetAction {
  const choices = idleGestures.filter(action => action !== previous)
  return choices[Math.min(choices.length - 1, Math.floor(Math.max(0, Math.min(1, random)) * choices.length))]!
}
// Exact critically damped spring; stable at different refresh rates and after pauses.
export function springStep(value: number, velocity: number, target: number, seconds: number, frequency = 13) {
  const dt = Math.max(0, Math.min(.08, seconds)), delta = value - target
  const decay = Math.exp(-frequency * dt), impulse = velocity + frequency * delta
  return { value: target + (delta + impulse * dt) * decay, velocity: (velocity - frequency * impulse * dt) * decay }
}
export function groundShadow(bob: number, shift: number) {
  const height = Math.max(0, -bob)
  return { scale: Math.max(.56, 1 - height * .023), opacity: Math.max(.10, .27 - height * .008), blur: 2.5 + height * .13, x: shift * .45 }
}

const key = (at: number, pose: Partial<PetPose> = {}): PetKey => ({ at, pose })
const chin = { arm: -90, armScale: .70, lean: -1.5, bob: .5 }
export function actionKeys(action: PetAction): PetKey[] {
  if (action === 'look') return [key(0), key(600, { yaw: -13, head: -4, lookX: -.7, shift: -1.5 }), key(1350, { yaw: -11, head: -2, lookX: -.7 }), key(2200, { yaw: 14, head: 4, lookX: .8, pitch: -2 }), key(2950, { yaw: 10, head: 2, lookX: .5 }), key(3750)]
  if (action === 'stretch') return [key(0), key(450, { stretch: .96, bob: 2, pitch: 3, arm: 4 }), key(1050, { stretch: 1.04, arm: -30, leftArm: 6, head: -5, pitch: -8, belly: .2 }), key(1850, { stretch: 1.035, arm: -27, leftArm: 5, head: -2, pitch: -6 }), key(2350, { stretch: .98, bob: 1, arm: -8 }), key(3000)]
  if (action === 'shuffle') return [key(0), key(420, { shift: -2, lean: -3, yaw: -6, rightFoot: 1, arm: -5 }), key(900, { shift: -3, lean: -2, leftFoot: .15 }), key(1500, { shift: 2, lean: 3, yaw: 7, leftFoot: 1, leftArm: -4 }), key(2050, { shift: 3, lean: 2, rightFoot: .15 }), key(2850)]
  if (action === 'bounce') return [key(0), key(300, { stretch: .94, bob: 4, arm: 5, pitch: 4 }), key(550, { stretch: 1.025, bob: -11, arm: -18, pitch: -4, leftFoot: .5, rightFoot: .5 }), key(760, { bob: -16, yaw: 5, arm: -22, head: -3 }), key(1060, { stretch: .95, bob: 3, arm: -8, pitch: 3 }), key(1320, { stretch: 1.015, bob: -4, arm: -3 }), key(1660, { stretch: .99, bob: 1 }), key(2050)]
  if (action === 'poke') return [key(0), key(130, { belly: -1, lean: -2, arm: -8 }), key(310, { belly: .65, lean: 2, bob: -2 }), key(520, { belly: -.3, lean: -1 }), key(760, { belly: .12 }), key(1050)]
  if (action === 'laugh') return [key(0), key(300, { lean: -5, yaw: -10, shift: -2, arm: -25, stretch: .96, head: 3 }), key(650, { lean: 6, yaw: 12, shift: 3, bob: -5, belly: .5, leftFoot: 1, head: -4 }), key(1030, { lean: -5, yaw: -11, shift: -3, arm: -20, stretch: .96, rightFoot: 1, leftArm: 5 }), key(1410, { lean: 5, yaw: 10, shift: 2, bob: -4, belly: .5, leftFoot: .8, head: -3 }), key(1800, { lean: -3, yaw: -6, arm: -15, rightFoot: .5 }), key(2230, { lean: 2, bob: -1, head: 2 }), key(2750)]
  if (action === 'land') return [key(0, { stretch: .94, bob: 3 }), key(180, { stretch: 1.025, bob: -3 }), key(390, { stretch: .99, bob: 1 }), key(650)]
  if (action === 'wave') return [key(0), key(230, { arm: 3, lean: .3 }), key(560, { arm: -23, lean: -2, bob: -1, yaw: -8, head: -3 }), key(880, { arm: 8, lean: .9 }), key(1210, { arm: -21, lean: -1.5, bob: -.7, yaw: -6, head: -2 }), key(1520, { arm: 5, lean: .6 }), key(1900)]
  if (action === 'blink') return [key(0), key(170), key(270, { blink: 1 }), key(325, { blink: 1 }), key(505), key(700)]
  if (action === 'think') return [key(0), key(700, chin), key(1470, { ...chin, arm: -88, lean: -2.6, bob: 1.2 }), key(2240, { ...chin, arm: -92, lean: .5, bob: .2 }), key(3010, { ...chin, arm: -89, lean: -1.7, bob: .7 }), key(3450, chin), key(4200)]
  if (action === 'reply') return [key(0), key(300, { lean: 1.7, bob: 2, stretch: .992 }), key(660, { lean: -.6, bob: -1.3, stretch: 1.003 }), key(1000)]
  return [key(0), key(8500)]
}
