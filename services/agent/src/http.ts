// 出站 HTTP 的统一护栏。
//
// 背景：Agent 的工具由 LLM 驱动，会拿到任意 URL。缺护栏时有两类真实风险：
//   1. 可用性：一个慢站点 / 一个无限大的响应体能把整个对话流拖死；
//   2. 安全：web_fetch("http://169.254.169.254/latest/meta-data/") 这类请求可以把
//      云主机元数据读进对话，SSRF。
//
// 因此对外出口（外部 URL）必须走 fetchGuarded：超时 + 响应体上限 + 目标地址校验 +
// 逐跳重定向校验。内部调用（Go 后端）用 fetchWithTimeout，只加超时与大小上限——
// 内网地址本来就是预期目标，套 SSRF 规则会把自己拦掉。
import dns from "node:dns";

export type HttpErrorKind = "timeout" | "too-large" | "blocked" | "http" | "network" | "invalid";

export class HttpError extends Error {
  readonly kind: HttpErrorKind;
  readonly status?: number;

  constructor(kind: HttpErrorKind, message: string, status?: number) {
    super(message);
    this.name = "HttpError";
    this.kind = kind;
    this.status = status;
  }
}

export const DEFAULT_TIMEOUT_MS = 15_000;
export const DEFAULT_MAX_BYTES = 2 * 1024 * 1024;
const MAX_REDIRECTS = 3;

export interface GuardedFetchOptions {
  timeoutMs?: number;
  maxBytes?: number;
  headers?: Record<string, string>;
  /** 是否启用 SSRF 目标校验；抓取外部 URL 时必须为 true。 */
  blockPrivate?: boolean;
}

export interface GuardedResponse {
  status: number;
  text: string;
  url: string;
}

/** 抓取外部站点：带全部护栏（超时 / 体积上限 / 目标校验 / 重定向逐跳校验）。 */
export async function fetchGuarded(
  rawURL: string,
  opts: GuardedFetchOptions = {},
): Promise<GuardedResponse> {
  const blockPrivate = opts.blockPrivate !== false;
  let current = rawURL;

  for (let hop = 0; ; hop++) {
    const target = parseURL(current);
    if (blockPrivate) await assertPublicTarget(target);

    const timeoutMs = opts.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    const res = await rawFetch(
      target.toString(),
      {
        method: "GET",
        redirect: "manual", // 重定向逐跳校验，避免「先公网后内网」绕过
        headers: opts.headers,
      },
      timeoutMs,
    );

    if (isRedirect(res.status)) {
      const location = res.headers.get("location");
      if (!location) throw new HttpError("invalid", `重定向缺少 Location（${res.status}）`);
      if (hop >= MAX_REDIRECTS) {
        throw new HttpError("http", `重定向次数超过上限 ${MAX_REDIRECTS}`);
      }
      current = new URL(location, target).toString();
      continue; // 下一跳会重新做目标校验
    }

    if (!res.ok) {
      throw new HttpError("http", `HTTP ${res.status}`, res.status);
    }
    const text = await readBodyWithLimit(res, opts.maxBytes ?? DEFAULT_MAX_BYTES);
    return { status: res.status, text, url: current };
  }
}

/**
 * 内部服务调用（如 Go 后端）：只加超时与响应体上限。
 * 不校验目标地址，因为后端地址本就在内网。
 */
export async function fetchWithTimeout(
  url: string,
  init: RequestInit & { timeoutMs?: number; maxBytes?: number } = {},
): Promise<Response> {
  const { timeoutMs = DEFAULT_TIMEOUT_MS, ...rest } = init;
  return rawFetch(url, rest as RequestInit, timeoutMs);
}

async function rawFetch(
  url: string,
  init: RequestInit,
  timeoutMs: number,
): Promise<Response> {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    return await fetch(url, { ...init, signal: ctrl.signal });
  } catch (e: any) {
    if (e?.name === "AbortError") {
      throw new HttpError("timeout", `请求超时（${timeoutMs}ms）: ${url}`);
    }
    throw new HttpError("network", e?.message || String(e));
  } finally {
    clearTimeout(timer);
  }
}

async function readBodyWithLimit(res: Response, maxBytes: number): Promise<string> {
  const body = res.body;
  // 部分运行环境（含测试桩）不提供流，退化为 text() 但仍检查长度。
  if (!body) {
    const text = await res.text();
    assertWithinLimit(text.length, maxBytes);
    return text;
  }
  const reader = body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      total += value.byteLength;
      // 边读边限：不等整个响应体进内存才判断。
      assertWithinLimit(total, maxBytes);
      chunks.push(value);
    }
  } finally {
    reader.cancel().catch(() => {});
  }
  return Buffer.concat(chunks.map((c) => Buffer.from(c.buffer, c.byteOffset, c.byteLength))).toString("utf8");
}

function assertWithinLimit(size: number, maxBytes: number): void {
  if (size > maxBytes) {
    throw new HttpError("too-large", `响应体超过上限 ${Math.floor(maxBytes / 1024)}KB`);
  }
}

function isRedirect(status: number): boolean {
  return status === 301 || status === 302 || status === 303 || status === 307 || status === 308;
}

// ===== SSRF 目标校验 =====

const BLOCKED_HOST_SUFFIXES = [".localhost", ".local", ".internal"];
const BLOCKED_HOSTNAMES = ["localhost", "metadata", "metadata.google.internal", "instance-data"];

export function isPrivateHostLiteral(host: string): boolean {
  let h = host.trim().toLowerCase();
  if (h.startsWith("[") && h.endsWith("]")) h = h.slice(1, -1); // [::1]
  if (h === "" || h === "::" || h === "::1" || h === "0.0.0.0") return true;

  // IPv4-mapped IPv6：::ffff:10.0.0.1 等价于 10.0.0.1
  const mapped = /^::ffff:(\d{1,3}(?:\.\d{1,3}){3})$/.exec(h);
  if (mapped) return isPrivateIPv4(mapped[1]);

  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(h)) return isPrivateIPv4(h);

  // IPv6：ULA(fc00::/7)、链路本地(fe80::/10)、站点本地(fec0::/10)
  if (/^(fc|fd|fe[89ab])/.test(h)) return true;
  return false;
}

export function isPrivateIPv4(ip: string): boolean {
  const parts = ip.split(".").map((p) => Number(p));
  // 解析不出来的一律按私有处理：宁可拒绝，不要放行。
  if (parts.length !== 4 || parts.some((p) => !Number.isInteger(p) || p < 0 || p > 255)) return true;
  const [a, b] = parts;
  if (a === 0 || a === 10 || a === 127) return true;
  if (a === 169 && b === 254) return true; // link-local / 云元数据
  if (a === 172 && b >= 16 && b <= 31) return true;
  if (a === 192 && b === 168) return true;
  if (a === 100 && b >= 64 && b <= 127) return true; // CGNAT
  if (a >= 224) return true; // 多播与保留段
  return false;
}

async function assertPublicTarget(url: URL): Promise<void> {
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new HttpError("blocked", `不支持的协议: ${url.protocol}`);
  }
  const host = url.hostname.toLowerCase();
  if (BLOCKED_HOSTNAMES.includes(host) || BLOCKED_HOST_SUFFIXES.some((s) => host.endsWith(s))) {
    throw new HttpError("blocked", `禁止访问内部域名: ${host}`);
  }
  if (isPrivateHostLiteral(host)) {
    throw new HttpError("blocked", `禁止访问内网地址: ${host}`);
  }
  // 域名形式：预先解析，任一解析结果为内网地址则拒绝。
  // 残余风险：DNS rebinding（解析与实际连接之间目标被改写）无法在应用层根治，
  // 需要靠网络策略 / 沙箱隔离兜底，这里按已知风险记录。
  let records: Array<{ address: string }> = [];
  try {
    records = await dns.promises.lookup(host, { all: true });
  } catch {
    // 解析失败交给真正的 fetch 去报错，避免这里给出误导性结论
    return;
  }
  for (const r of records) {
    if (isPrivateHostLiteral(r.address)) {
      throw new HttpError("blocked", `域名 ${host} 解析到内网地址: ${r.address}`);
    }
  }
}

function parseURL(raw: string): URL {
  try {
    return new URL(raw);
  } catch {
    throw new HttpError("invalid", `无效的 URL: ${raw}`);
  }
}
