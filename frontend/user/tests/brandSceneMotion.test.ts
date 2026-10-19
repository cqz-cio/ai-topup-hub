import test from 'node:test'
import assert from 'node:assert/strict'
import { JellyWorld } from '../src/utils/jellyfishing/simulation.js'
import { StationaryActors, specs } from '../src/utils/jellyfishing/actors.js'
import { MarineEnvironment, plants, plantOffset } from '../src/utils/jellyfishing/environment.js'
import { mountJellyfishing } from '../src/utils/jellyfishing/renderer.js'

test('random swimming stays smooth and in bounds on desktop and narrow screens', () => {
  for (const [width, height, count] of [[1920, 1080, 9], [390, 844, 6], [320, 568, 6]]) {
    const world = new JellyWorld({ width, height, count, seed: 42 })
    const initial = world.snapshot()
    for (let frame = 0; frame < 120 * 180; frame++) {
      const before = world.items.map(j => ({ x: j.x, y: j.y, angle: j.angle }))
      world.step(1 / 120)
      world.items.forEach((j, i) => {
        const bounds = world.limits(j)
        assert.ok(j.x >= bounds.left && j.x <= bounds.right)
        assert.ok(j.y >= bounds.top && j.y <= bounds.bottom)
        assert.ok(Math.hypot(j.x - before[i].x, j.y - before[i].y) < 1.3)
        assert.ok(Math.abs(j.angle - before[i].angle) <= .85 / 120 + 1e-12)
      })
    }
    assert.equal(world.boundaryCorrections, 0, 'turn before the edge instead of bouncing')
    assert.ok(world.routes > count * 3, 'pick new destinations continuously')
    assert.ok(world.maxAcceleration <= 180 + 1e-10)
    assert.notDeepEqual(world.snapshot().items, initial.items)
  }
})

test('route changes preserve velocity and position; resizing preserves identities', () => {
  const world = new JellyWorld({ seed: 123 })
  for (let i = 0; i < 240; i++) world.step(1 / 120)
  const positions = () => world.items.map(j => [j.id, j.x, j.y, j.vx, j.vy, j.phase])
  const before = positions()
  world.reroute()
  assert.deepEqual(positions(), before)
  world.resize(320, 568)
  assert.deepEqual(world.items.map(j => j.id), before.map(j => j[0]))
  world.items.forEach(j => {
    const b = world.limits(j)
    assert.ok(j.x >= b.left && j.x <= b.right && j.y >= b.top && j.y <= b.bottom)
  })
  assert.notDeepEqual(new JellyWorld({ seed: 1 }).snapshot().items, new JellyWorld({ seed: 2 }).snapshot().items)
})

test('characters keep planted feet, small continuous net gestures and staggered timing', () => {
  const actors = new StationaryActors(42)
  const near = specs.map(a => ({ x: a.net[0], y: a.net[1] }))
  let previous = actors.items.map(a => actors.warp(a, a.net))
  let events = 0, lastEvent = -10, maxReach = 0
  for (let frame = 0; frame < 120 * 120; frame++) {
    actors.step(1 / 120, near)
    const count = actors.items.reduce((sum, a) => sum + a.events, 0)
    if (count > events) {
      assert.equal(count, events + 1, 'never gesture together')
      assert.ok(actors.time - lastEvent >= 3.3 - 1e-10)
      lastEvent = actors.time
      events = count
    }
    actors.items.forEach((a, i) => {
      assert.deepEqual(actors.warp(a, a.anchor), a.anchor)
      const point = actors.warp(a, a.net)
      assert.ok(Math.hypot(point[0] - previous[i][0], point[1] - previous[i][1]) < .8)
      maxReach = Math.max(maxReach, Math.hypot(point[0] - a.net[0], point[1] - a.net[1]))
      previous[i] = point
    })
  }
  assert.ok(maxReach > 25 && maxReach < 40)
  assert.ok(actors.items.every(a => a.events > 5))
})

test('ambient plants stay rooted and recycled bubbles enter invisibly', () => {
  const environment = new MarineEnvironment(42)
  for (let frame = 0; frame < 120 * 180; frame++) {
    environment.step(1 / 120)
    for (const plant of plants) {
      assert.ok(plantOffset(plant, plant.x, plant.root, environment.time) === 0)
      assert.ok(Math.abs(plantOffset(plant, plant.x, plant.top + 7, environment.time)) <= plant.amplitude * 1.24)
    }
    for (const bubble of environment.items) {
      const pose = environment.bubblePose(bubble)
      assert.ok(Number.isFinite(pose.x) && Number.isFinite(pose.y))
      assert.ok(pose.opacity >= 0 && pose.opacity <= .68)
      if (bubble.age === 0) assert.equal(pose.opacity, 0)
    }
  }
  assert.ok(environment.respawns > 50)
})

test('WebGL unavailable keeps the CSS wallpaper without starting a renderer', () => {
  let fallback = 0
  let ready = 0
  const scene = mountJellyfishing({ getContext: () => null } as unknown as HTMLCanvasElement, {
    onFallback: () => { fallback++ },
    onReady: () => { ready++ },
  })
  scene.dispose()
  scene.dispose()
  assert.equal(fallback, 1)
  assert.equal(ready, 0)
})
