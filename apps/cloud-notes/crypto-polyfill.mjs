import { webcrypto } from 'crypto'
if (typeof globalThis.crypto === 'undefined') {
  Object.defineProperty(globalThis, 'crypto', { value: webcrypto, writable: true })
}
export {}
