import test from 'node:test'
import assert from 'node:assert/strict'
import { actorCamera } from '../src/utils/jellyfishing/composition.js'
import { StationaryActors } from '../src/utils/jellyfishing/actors.js'

test('desktop and landscape retain the original background-aligned actor transform', () => {
  for (const [width, height] of [[768, 1024], [1280, 720], [1920, 1080], [667, 375]]) {
    const scale = Math.max(width / 1672, height / 941)
    assert.deepEqual(actorCamera(width, height), { scale, ox: (width - 1672 * scale) * (width < 768 ? .75 : .5), oy: (height - 941 * scale) / 2 })
  }
})

test('portrait composition keeps both full characters and their gestures within the viewport', () => {
  for (const [width, height] of [[320, 568], [320, 844], [390, 844], [430, 932]]) {
    const camera = actorCamera(width, height)
    const actors = new StationaryActors(42)
    actors.cue()
    for (let frame = 0; frame < 120 * 8; frame++) {
      actors.step(1 / 120, actors.items.map(a => ({x: a.net[0], y: a.net[1]})))
      for (const actor of actors.items) {
        const [x, y, w, h] = actor.rect
        for (const point of [[x,y],[x+w,y],[x,y+h],[x+w,y+h],actor.net,actor.anchor]) {
          const pose = actors.warp(actor, point)
          const px = camera.ox + pose[0] * camera.scale
          const py = camera.oy + pose[1] * camera.scale
          assert.ok(px >= 0 && px <= width, `horizontal clipping at ${width}: ${px}`)
          assert.ok(py >= 0 && py <= height, `vertical clipping at ${height}: ${py}`)
        }
      }
    }
  }
})
