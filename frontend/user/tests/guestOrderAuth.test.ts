import test from 'node:test'
import assert from 'node:assert/strict'

const guestAuth = {
  email: 'guest@example.com',
  order_password: 'order-secret',
}

const emptyGuestAuth = {
  email: '',
  order_password: '',
}

const createMemoryStorage = (initial: Record<string, string> = {}) => {
  const values = new Map(Object.entries(initial))
  return {
    get length() {
      return values.size
    },
    clear() {
      values.clear()
    },
    getItem(key: string) {
      return values.get(key) ?? null
    },
    key(index: number) {
      return Array.from(values.keys())[index] ?? null
    },
    removeItem(key: string) {
      values.delete(key)
    },
    setItem(key: string, value: string) {
      values.set(key, value)
    },
  }
}

const installWindow = (value: object) => {
  const original = Object.getOwnPropertyDescriptor(globalThis, 'window')
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value,
  })
  return () => {
    if (original) {
      Object.defineProperty(globalThis, 'window', original)
      return
    }
    Reflect.deleteProperty(globalThis, 'window')
  }
}

const importGuestOrderAuth = (name: string) => {
  return import(`../src/utils/guestOrderAuth.ts?test=${name}-${Date.now()}-${Math.random()}`)
}

test('guest auth uses sessionStorage when it is available', async (t) => {
  const sessionStorage = createMemoryStorage()
  const localStorage = createMemoryStorage()
  const restoreWindow = installWindow({ sessionStorage, localStorage })
  t.after(restoreWindow)
  const auth = await importGuestOrderAuth('available')

  auth.saveGuestOrderAuth(guestAuth)

  assert.deepEqual(auth.loadGuestOrderAuth(), guestAuth)
  assert.equal(sessionStorage.getItem('guest_order_auth'), JSON.stringify(guestAuth))

  auth.clearGuestOrderAuth()
  assert.deepEqual(auth.loadGuestOrderAuth(), emptyGuestAuth)
})

test('guest auth stays available in the current SPA when sessionStorage writes fail', async (t) => {
  const sessionStorage = {
    ...createMemoryStorage(),
    setItem() {
      throw new DOMException('blocked', 'SecurityError')
    },
  }
  const localStorage = createMemoryStorage()
  const restoreWindow = installWindow({ sessionStorage, localStorage })
  t.after(restoreWindow)
  const auth = await importGuestOrderAuth('write-failure')

  auth.saveGuestOrderAuth(guestAuth)

  assert.deepEqual(auth.loadGuestOrderAuth(), guestAuth)
})

test('legacy credentials survive the current SPA when migration storage writes fail', async (t) => {
  const sessionStorage = {
    ...createMemoryStorage(),
    setItem() {
      throw new DOMException('blocked', 'SecurityError')
    },
  }
  const localStorage = createMemoryStorage({
    guest_order_auth: JSON.stringify(guestAuth),
  })
  const restoreWindow = installWindow({ sessionStorage, localStorage })
  t.after(restoreWindow)
  const auth = await importGuestOrderAuth('migration-write-failure')

  assert.deepEqual(auth.loadGuestOrderAuth(), guestAuth)
  assert.equal(localStorage.getItem('guest_order_auth'), null)
  assert.deepEqual(auth.loadGuestOrderAuth(), guestAuth)
})

test('guest auth handles Storage property getters that throw', async (t) => {
  const unavailableWindow = {}
  Object.defineProperties(unavailableWindow, {
    sessionStorage: {
      get() {
        throw new DOMException('blocked', 'SecurityError')
      },
    },
    localStorage: {
      get() {
        throw new DOMException('blocked', 'SecurityError')
      },
    },
  })
  const restoreWindow = installWindow(unavailableWindow)
  t.after(restoreWindow)
  const auth = await importGuestOrderAuth('getter-failure')

  assert.doesNotThrow(() => auth.saveGuestOrderAuth(guestAuth))
  assert.deepEqual(auth.loadGuestOrderAuth(), guestAuth)
  assert.doesNotThrow(() => auth.clearGuestOrderAuth())
  assert.deepEqual(auth.loadGuestOrderAuth(), emptyGuestAuth)
})

test('loaded guest auth is copied so callers cannot mutate the in-memory credential', async (t) => {
  const sessionStorage = createMemoryStorage()
  const localStorage = createMemoryStorage()
  const restoreWindow = installWindow({ sessionStorage, localStorage })
  t.after(restoreWindow)
  const auth = await importGuestOrderAuth('copy')

  auth.saveGuestOrderAuth(guestAuth)
  const loaded = auth.loadGuestOrderAuth()
  loaded.email = 'mutated@example.com'

  assert.deepEqual(auth.loadGuestOrderAuth(), guestAuth)
})
