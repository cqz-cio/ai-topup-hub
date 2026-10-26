import assert from 'node:assert/strict'
import test from 'node:test'
import { petHit, petLook, readPetSettings, savePetSettings } from '../src/components/support/petInteraction.ts'
import { actionKeys, neutralPose, samplePose, nextIdleGesture, springStep, groundShadow, type PetAction } from '../src/components/support/petMotion.ts'

test('belly pokes stay away from face, necklace and shorts', () => {
  assert.equal(petHit(.55, .64), 'belly')
  for (const point of [[.5, .3], [.5, .5], [.5, .77], [.16, .62]]) assert.equal(petHit(point[0]!, point[1]!), 'body')
  for (const point of [[0, 0], [1, .5], [NaN, .5]]) assert.equal(petHit(point[0]!, point[1]!), 'outside')
})
test('look direction is bounded even near a screen edge', () => {
  assert.deepEqual(petLook(.5, .3), { x: 0, y: 0 })
  assert.deepEqual(petLook(-5, 8), { x: -1, y: 1 })
})
test('invalid and obsolete local preferences cannot move the pet off screen', () => {
  for (const raw of ['{', 'null', '[]', '{}', '{"position":{"x":4,"y":0}}', '{"position":{"x":"1","y":0}}']) assert.equal(readPetSettings(raw).position, undefined)
  assert.equal(readPetSettings('{"size":"enormous","resting":"false"}').size, 'normal')
  assert.equal(readPetSettings('{"resting":"false"}').resting, false)
  assert.deepEqual(readPetSettings('{"size":"large","resting":true,"position":{"x":0.2,"y":1}}'), { size: 'large', resting: true, position: { x: .2, y: 1 } })
})
test('partial preference updates preserve position; reset removes only position', () => {
  let raw: string | null = null
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: { getItem: () => raw, setItem: (_key: string, value: string) => { raw = value } } })
  savePetSettings({ position: { x: .3, y: .4 } }); savePetSettings({ size: 'large', resting: true })
  assert.deepEqual(readPetSettings(raw).position, { x: .3, y: .4 })
  savePetSettings({ position: undefined }); assert.equal(readPetSettings(raw).position, undefined)
  assert.equal(readPetSettings(raw).resting, true)
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, get() { throw new Error('blocked') } })
  assert.doesNotThrow(() => savePetSettings({ size: 'normal' }))
  Reflect.deleteProperty(globalThis, 'localStorage')
})
test('every finite animation returns to an undistorted connected pose', () => {
  for (const action of ['wave', 'blink', 'think', 'reply', 'poke', 'laugh', 'land', 'look', 'stretch', 'shuffle', 'bounce'] as PetAction[]) {
    const keys = actionKeys(action), end = keys.at(-1)!.at
    assert.deepEqual(samplePose(keys, end + 100), neutralPose())
    for (let t = 0; t <= end; t += 17) {
      const pose = samplePose(keys, t)
      assert.ok(Object.values(pose).every(Number.isFinite))
      assert.ok(pose.stretch >= .94 && pose.stretch <= 1.04)
      assert.ok(pose.blink >= 0 && pose.blink <= 1)
      assert.ok(Math.abs(pose.belly) <= 1)
    }
  }
})
test('poke has a local indentation followed by a rebound', () => {
  const keys = actionKeys('poke')
  assert.ok(samplePose(keys, 130).belly < 0)
  assert.ok(samplePose(keys, 310).belly > 0)
  assert.equal(samplePose(keys, 1050).belly, 0)
})

test('idle variety never repeats the same gesture and random boundaries are valid', () => {
  const actions = new Set(['look', 'stretch', 'shuffle', 'bounce'])
  for (const previous of actions) for (const random of [0, .2, .5, .9999, 1]) {
    const next = nextIdleGesture(previous as PetAction, random)
    assert.ok(actions.has(next)); assert.notEqual(next, previous)
  }
})
test('pose curves have continuous velocity and stay within the segment bounds', () => {
  const keys = [{ at: 0, pose: { yaw: 0 } }, { at: 400, pose: { yaw: 4 } }, { at: 1000, pose: { yaw: 10 } }, { at: 1600, pose: { yaw: 0 } }]
  const dt = .01
  const left = (samplePose(keys,400).yaw - samplePose(keys,400-dt).yaw)/dt
  const right = (samplePose(keys,400+dt).yaw - samplePose(keys,400).yaw)/dt
  assert.ok(Math.abs(left-right) < .00001)
  assert.ok(left > 0, 'do not stop at an intermediate point on the way up')
  for (let t = 0; t <= 1600; t++) assert.ok(samplePose(keys,t).yaw >= 0 && samplePose(keys,t).yaw <= 10)
  assert.deepEqual(samplePose(keys,-10),neutralPose())
})
test('pointer spring converges consistently at 30, 60 and 120 fps', () => {
  const outcomes = [30,60,120].map(fps => {
    let spring = { value: -1, velocity: 0 }
    for (let i = 0; i < fps; i++) spring = springStep(spring.value,spring.velocity,1,1/fps)
    assert.ok(spring.value <= 1 && spring.value > .99)
    return spring.value
  })
  assert.ok(Math.max(...outcomes)-Math.min(...outcomes) < 1e-9)
})
test('drag reversal and a delayed frame do not cause spring explosions', () => {
  let spring = { value: 12, velocity: 0 }
  for (let i=0; i<120; i++) spring = springStep(spring.value,spring.velocity,-12,i===5 ? 10 : 1/60,16)
  assert.ok(Math.abs(spring.value+12)<.01)
  assert.ok(Number.isFinite(spring.velocity))
})
test('a jump has anticipation, airtime and landing; its shadow follows height', () => {
  const keys=actionKeys('bounce')
  assert.ok(samplePose(keys,300).bob>0)
  assert.ok(samplePose(keys,760).bob<-10)
  assert.ok(samplePose(keys,1060).bob>0)
  const ground=groundShadow(0,0), air=groundShadow(-16,3)
  assert.ok(air.scale<ground.scale && air.opacity<ground.opacity && air.blur>ground.blur)
  assert.ok(air.x>0)
  assert.deepEqual(groundShadow(3,0),ground)
})
