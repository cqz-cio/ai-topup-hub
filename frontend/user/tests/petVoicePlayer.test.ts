import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createPetVoicePlayer, type VoiceState } from '../src/components/support/petVoicePlayer.ts'
import { petVoiceClips } from '../src/components/support/petVoiceClips.ts'

class FakeAudio {
  src = ''; preload = ''; volume = 1; pauses = 0; loads = 0
  onended: (() => void) | null = null
  onerror: (() => void) | null = null
  resolve!: () => void
  reject!: (reason: Error) => void
  promise = new Promise<void>((resolve, reject) => { this.resolve = resolve; this.reject = reject })
  play() { return this.promise }
  pause() { this.pauses++ }
  removeAttribute(name: string) { if (name === 'src') this.src = '' }
  load() { this.loads++ }
}
function setup(timeout = 12000) {
  const audios: FakeAudio[] = [], states: VoiceState[] = []
  const player = createPetVoicePlayer(s => states.push(s), () => {
    const audio = new FakeAudio(); audios.push(audio); return audio as unknown as HTMLAudioElement
  }, timeout)
  return { player, audios, states, state: () => states.at(-1) }
}
test('voice assets are local MP3 files and no audio is created before a click', () => {
  const ctx = setup()
  assert.equal(ctx.audios.length, 0)
  assert.equal(petVoiceClips.length, 2)
  for (const clip of petVoiceClips) {
    const bytes = readFileSync(new URL(clip.src))
    assert.equal(bytes.subarray(0, 3).toString(), 'ID3')
    assert.ok(bytes.length > 10000 && bytes.length < 100000)
    assert.ok(clip.source.startsWith('https://www.bilibili.com/video/'))
    assert.equal(clip.text.length, 3)
  }
  ctx.player.dispose()
})
test('playing, ending, and replay release the completed audio', async () => {
  const ctx = setup(), pending = ctx.player.toggle('one', '/one.mp3')
  assert.deepEqual(ctx.state(), { id: 'one', status: 'loading' })
  assert.equal(ctx.audios[0].preload, 'none')
  ctx.audios[0].resolve(); await pending
  assert.deepEqual(ctx.state(), { id: 'one', status: 'playing' })
  ctx.audios[0].onended!()
  assert.deepEqual(ctx.state(), { id: null, status: 'idle' })
  assert.equal(ctx.audios[0].src, '')
  const replay = ctx.player.toggle('one', '/one.mp3')
  ctx.audios[1].resolve(); await replay
  ctx.player.dispose()
})
test('a second click cancels a pending play and its late resolution cannot restart it', async () => {
  const ctx = setup(), pending = ctx.player.toggle('one', '/one.mp3')
  await ctx.player.toggle('one', '/one.mp3')
  ctx.audios[0].resolve(); await pending
  assert.deepEqual(ctx.state(), { id: null, status: 'idle' })
  assert.equal(ctx.audios[0].pauses, 1)
  assert.equal(ctx.audios[0].onerror, null)
  ctx.player.dispose()
})
test('switching clips stops the first and ignores its late rejection', async () => {
  const ctx = setup(), old = ctx.player.toggle('one', '/one.mp3')
  const next = ctx.player.toggle('two', '/two.mp3')
  assert.equal(ctx.audios[0].src, '')
  ctx.audios[1].resolve(); await next
  ctx.audios[0].reject(new Error('aborted')); await old
  assert.deepEqual(ctx.state(), { id: 'two', status: 'playing' })
  ctx.player.dispose()
})
test('play rejection and media error show retryable errors', async () => {
  const ctx = setup(), pending = ctx.player.toggle('one', '/one.mp3')
  ctx.audios[0].reject(new Error('NotAllowedError')); await pending
  assert.deepEqual(ctx.state(), { id: 'one', status: 'error' })
  const retry = ctx.player.toggle('one', '/one.mp3')
  ctx.audios[1].resolve(); await retry
  ctx.audios[1].onerror!()
  assert.deepEqual(ctx.state(), { id: 'one', status: 'error' })
  assert.equal(ctx.audios[1].src, '')
  ctx.player.dispose()
})
test('a stalled clip times out and a late success cannot clear its error', async () => {
  const ctx = setup(10), pending = ctx.player.toggle('one', '/one.mp3')
  await new Promise(resolve => setTimeout(resolve, 30))
  assert.deepEqual(ctx.state(), { id: 'one', status: 'error' })
  ctx.audios[0].resolve(); await pending
  assert.deepEqual(ctx.state(), { id: 'one', status: 'error' })
  ctx.player.dispose()
})
test('dispose cancels pending audio and prevents any new playback', async () => {
  const ctx = setup(), pending = ctx.player.toggle('one', '/one.mp3')
  ctx.player.dispose()
  ctx.audios[0].resolve(); await pending
  await ctx.player.toggle('two', '/two.mp3')
  assert.equal(ctx.audios.length, 1)
  assert.deepEqual(ctx.state(), { id: null, status: 'idle' })
})
