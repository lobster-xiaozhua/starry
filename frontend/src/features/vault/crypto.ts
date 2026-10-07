// 客户端零知识加密工具：主密码永不上传，密钥仅存于内存。
// 使用 Web Crypto：PBKDF2 派生 AES-GCM 256 密钥。

const enc = new TextEncoder()
const dec = new TextDecoder()
const VERIFIER_PLAIN = 'starry-vault-v1'

function bufToB64(buf: ArrayBuffer | Uint8Array): string {
  const bytes = buf instanceof Uint8Array ? buf : new Uint8Array(buf)
  let bin = ''
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i])
  return btoa(bin)
}

function b64ToBytes(b64: string): Uint8Array {
  const bin = atob(b64)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out
}

// 生成随机 salt（base64）
export function randomSalt(): string {
  return bufToB64(crypto.getRandomValues(new Uint8Array(16)).buffer)
}

// 由主密码 + salt 派生 AES-GCM 密钥
export async function deriveKey(password: string, saltB64: string): Promise<CryptoKey> {
  const base = await crypto.subtle.importKey('raw', enc.encode(password), 'PBKDF2', false, [
    'deriveKey',
  ])
  return crypto.subtle.deriveKey(
    {
      name: 'PBKDF2',
      salt: b64ToBytes(saltB64) as unknown as BufferSource,
      iterations: 100_000,
      hash: 'SHA-256',
    },
    base,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt'],
  )
}

// 加密任意 JSON 对象，输出 base64（12 字节 IV 前缀 + 密文）
export async function encryptJSON(key: CryptoKey, obj: unknown): Promise<string> {
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const ct = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv: iv as unknown as BufferSource },
    key,
    enc.encode(JSON.stringify(obj)) as unknown as BufferSource,
  )
  const combined = new Uint8Array(iv.length + ct.byteLength)
  combined.set(iv, 0)
  combined.set(new Uint8Array(ct), iv.length)
  return bufToB64(combined.buffer)
}

// 解密 base64 密文为 JSON 对象
export async function decryptJSON<T>(key: CryptoKey, b64: string): Promise<T> {
  const combined = b64ToBytes(b64)
  const iv = combined.slice(0, 12)
  const ct = combined.slice(12)
  const pt = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: iv as unknown as BufferSource },
    key,
    ct as unknown as BufferSource,
  )
  return JSON.parse(dec.decode(pt)) as T
}

// 设定主密码时：生成 verifier（加密已知常量）
export async function makeVerifier(key: CryptoKey): Promise<string> {
  return encryptJSON(key, { v: VERIFIER_PLAIN })
}

// 解锁时：校验主密码（解密 verifier 并比对常量）
export async function verifyKey(key: CryptoKey, verifierB64: string): Promise<boolean> {
  try {
    const obj = await decryptJSON<{ v: string }>(key, verifierB64)
    return obj.v === VERIFIER_PLAIN
  } catch {
    return false
  }
}
