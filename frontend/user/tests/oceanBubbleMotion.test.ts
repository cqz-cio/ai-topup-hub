import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'

const source = readFileSync(new URL('../src/utils/oceanScene/bubbleWorld.js', import.meta.url), 'utf8')
const data = JSON.parse(readFileSync(new URL('../src/utils/oceanScene/bubbleSprites.json', import.meta.url), 'utf8'))
const sandbox: Record<string, any> = {}
runInNewContext(source.replace('export class', 'class') + '\nglobalThis.BubbleWorld = BubbleWorld;', sandbox)
const BubbleWorld = sandbox.BubbleWorld

function inside(world: any) {
  for (const b of world.items) {
    assert(Number.isFinite(b.x) && Number.isFinite(b.y))
    assert(b.x >= b.r && b.x <= world.width - b.r)
    assert(b.y >= b.r && b.y <= world.height - b.r)
  }
}

test('original bubbles remain fully inside each viewport during ten minutes of random drift and resizing', () => {
  for (const scene of Object.keys(data)) {
    for (const [width,height] of [[1920,1080], [390,844], [320,568]]) {
      const world = new BubbleWorld(data[scene].sprites, width, height, 32)
      const count = world.items.length
      for (let i = 0; i < 36000; i++) { world.step(1/60); inside(world) }
      assert(world.collisions > 0)
      assert.equal(world.items.length, count)
      world.resize(320,568); inside(world)
      world.resize(1920,1080); inside(world)
    }
  }
})

test('all four walls reflect outward velocity inward without teleporting or sticking', () => {
  for (const wall of ['left','right','top','bottom']) {
    const world = new BubbleWorld(data.detail.sprites,800,600,1), b = world.items[0]
    b.x = wall === 'left' ? b.r + .01 : wall === 'right' ? 800 - b.r - .01 : 400
    b.y = wall === 'top' ? b.r + .01 : wall === 'bottom' ? 600 - b.r - .01 : 300
    b.vx = wall === 'left' ? -20 : wall === 'right' ? 20 : 0
    b.vy = wall === 'top' ? -20 : wall === 'bottom' ? 20 : 0
    b.lock = 1
    const before = { x:b.x, y:b.y }
    world.step(.05); inside(world)
    assert.equal(b.hits,1)
    assert(Math.hypot(b.x-before.x,b.y-before.y) < 2)
    assert(wall === 'left' ? b.vx > 0 : wall === 'right' ? b.vx < 0 : wall === 'top' ? b.vy > 0 : b.vy < 0)
    for (let i=0; i<30; i++) world.step(1/60)
    assert.equal(b.hits,1); inside(world)
  }
})

test('zero elapsed time freezes positions and a delayed frame cannot jump across the screen', () => {
  const world = new BubbleWorld(data.detail.sprites,1920,1080,4)
  const before = world.items.map((b: any) => [b.x,b.y])
  world.step(0)
  world.items.forEach((b: any,i: number) => assert.deepEqual([b.x,b.y],before[i]))
  world.step(120)
  world.items.forEach((b: any,i: number) => assert(Math.hypot(b.x-before[i][0],b.y-before[i][1]) < 2))
  inside(world)
})
