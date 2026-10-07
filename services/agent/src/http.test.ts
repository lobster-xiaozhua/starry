// http.ts 的单元测试：重点是 SSRF 拦截、响应体上限与超时这些「出事才知道」的护栏。
import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";

import {
  fetchGuarded,
  HttpError,
  isPrivateHostLiteral,
  isPrivateIPv4,
} from "./http.js";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

// 用 Response 构造流式响应，便于验证「边读边限」。
function streamResponse(chunks: string[], init: ResponseInit = {}): Response {
  const enc = new TextEncoder();
  let i = 0;
  const stream = new ReadableStream({
    pull(controller) {
      if (i < chunks.length) {
        controller.enqueue(enc.encode(chunks[i++]));
      } else {
        controller.close();
      }
    },
  });
  return new Response(stream, { status: 200, ...init });
}

describe("isPrivateIPv4", () => {
  it("识别私有与保留网段", () => {
    const privates = [
      "10.0.0.1",
      "10.255.255.255",
      "127.0.0.1",
      "127.1.2.3",
      "169.254.169.254", // 云元数据
      "172.16.0.1",
      "172.31.255.255",
      "192.168.1.1",
      "0.0.0.0",
      "100.64.0.1",
      "224.0.0.1",
    ];
    for (const ip of privates) {
      assert.equal(isPrivateIPv4(ip), true, `${ip} 应判定为内网`);
    }
  });

  it("放行公网地址", () => {
    for (const ip of ["8.8.8.8", "1.1.1.1", "114.114.114.114", "172.32.0.1", "192.0.2.1"]) {
      assert.equal(isPrivateIPv4(ip), false, `${ip} 应判定为公网`);
    }
  });

  it("无法解析时按内网处理（宁可拒绝）", () => {
    assert.equal(isPrivateIPv4("300.1.1.1"), true);
    assert.equal(isPrivateIPv4("10.0.0"), true);
    assert.equal(isPrivateIPv4("abc"), true);
  });
});

describe("isPrivateHostLiteral", () => {
  it("覆盖 IPv6 环回/ULA/链路本地与 IPv4-mapped 形式", () => {
    for (const h of ["::1", "::", "[::1]", "fd00::1", "fc00::1", "fe80::1", "::ffff:10.0.0.1"]) {
      assert.equal(isPrivateHostLiteral(h), true, `${h} 应判定为内网`);
    }
  });

  it("放行公网 IPv6 与域名形态", () => {
    for (const h of ["2001:4860:4860::8888", "example.com"]) {
      assert.equal(isPrivateHostLiteral(h), false, `${h} 应判定为公网`);
    }
  });
});

describe("fetchGuarded", () => {
  it("拦截内网与元数据地址", async () => {
    let called = 0;
    globalThis.fetch = (async () => {
      called++;
      return new Response("leaked");
    }) as typeof fetch;

    for (const url of [
      "http://127.0.0.1:8080/api/admin/users",
      "http://169.254.169.254/latest/meta-data/",
      "http://localhost:3001/health",
      "http://[::1]/",
      "http://10.1.2.3/",
      "file:///etc/passwd",
    ]) {
      await assert.rejects(
        () => fetchGuarded(url),
        (e: unknown) => e instanceof HttpError && (e.kind === "blocked" || e.kind === "invalid"),
        `${url} 应被拦截`,
      );
    }
    assert.equal(called, 0, "被拦截的请求不应真正发出");
  });

  it("正常抓取公网 URL 并返回文本", async () => {
    globalThis.fetch = (async () => streamResponse(["hello ", "world"])) as typeof fetch;
    const res = await fetchGuarded("https://example.com/a");
    assert.equal(res.text, "hello world");
    assert.equal(res.status, 200);
  });

  it("响应体超过上限时报错而不是吃满内存", async () => {
    globalThis.fetch = (async () => streamResponse(["x".repeat(100), "y".repeat(100)])) as typeof fetch;
    await assert.rejects(
      () => fetchGuarded("https://example.com/big", { maxBytes: 50 }),
      (e: unknown) => e instanceof HttpError && e.kind === "too-large",
    );
  });

  it("重定向到内网时被拦截（逐跳校验）", async () => {
    let hop = 0;
    globalThis.fetch = (async () => {
      hop++;
      if (hop === 1) {
        return new Response(null, {
          status: 302,
          headers: { location: "http://169.254.169.254/latest/" },
        });
      }
      return new Response("leaked");
    }) as typeof fetch;

    await assert.rejects(
      () => fetchGuarded("https://example.com/redirect"),
      (e: unknown) => e instanceof HttpError && e.kind === "blocked",
    );
    assert.equal(hop, 1, "第二跳不应发出");
  });

  it("超时后抛出可识别的 timeout 错误", async () => {
    globalThis.fetch = (async (_url: any, init: any) => {
      // 与 Node 真实行为一致：abort 时抛 name 为 AbortError 的错误。
      return new Promise((_resolve, reject) => {
        init.signal.addEventListener("abort", () =>
          reject(Object.assign(new Error("The operation was aborted"), { name: "AbortError" })),
        );
      });
    }) as typeof fetch;

    await assert.rejects(
      () => fetchGuarded("https://example.com/slow", { timeoutMs: 10 }),
      (e: unknown) => e instanceof HttpError && e.kind === "timeout",
    );
  });

  it("HTTP 错误状态码带出状态值", async () => {
    globalThis.fetch = (async () => new Response("nope", { status: 404 })) as typeof fetch;
    await assert.rejects(
      () => fetchGuarded("https://example.com/404"),
      (e: unknown) => e instanceof HttpError && e.kind === "http" && e.status === 404,
    );
  });
});
