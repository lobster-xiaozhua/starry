const { webcrypto } = require('crypto')
if (typeof globalThis.crypto === 'undefined') {
  Object.defineProperty(globalThis, 'crypto', { value: webcrypto, writable: true })
}
