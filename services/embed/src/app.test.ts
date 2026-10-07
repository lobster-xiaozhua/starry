// embed 服务契约测试：用桩替换真实模型，因此不需要下载 25MB 权重。
// 运行：npm test --workspace starry-embed
import assert from "node:assert/strict";
import { after, describe, it } from "node:test";
import type { AddressInfo } from "node:net";
import type { Server } from "node:http";

import { createApp, MAX_TEXTS, type ModelState } from "./app.js";

const TOKEN = "test-internal-token";

// 启动一个临时服务器，返回 base URL 与关闭函数。
async function start(opts: {
  state: ModelState;
  error?: string;
  embed?: (texts: string[]) => Promise<number[][]>;
  token?: string;
}): Promise<{ url: string; close: () => Promise<void> }> {
  const app = createApp({
    internalToken: opts.token ?? TOKEN,
    embedService: {
      embed: opts.embed ?? (async (texts: string[]) => texts.map(() => [0.1, 0.2])),
    },
    state: () => ({ state: opts.state, error: opts.error ?? "" }),
  });
  const server: Server = await new Promise((resolve) => {
    const s = app.listen(0, "127.0.0.1", () => resolve(s));
  });
  const { port } = server.address() as AddressInfo;
  return {
    url: `http://127.0.0.1:${port}`,
    close: () => new Promise<void>((resolve) => server.close(() => resolve())),
  };
}

const servers: Array<() => Promise<void>> = [];
after(async () => {
  for (const close of servers) await close();
});

async function withServer<T>(
  opts: Parameters<typeof start>[0],
  fn: (url: string) => Promise<T>,
): Promise<T> {
  const s = await start(opts);
  servers.push(s.close);
  return fn(s.url);
}

describe("/health", () => {
  it("始终 200，并用 embedding 字段暴露模型就绪状态", async () => {
    await withServer({ state: "pending" }, async (url) => {
      const res = await fetch(`${url}/health`);
      assert.equal(res.status, 200);
      assert.deepEqual(await res.json(), { status: "ok", embedding: "pending" });
    });
  });
});

describe("/api/agent/embed", () => {
  it("缺少内部令牌时返回 401", async () => {
    await withServer({ state: "ready" }, async (url) => {
      const res = await fetch(`${url}/api/agent/embed`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ texts: ["hi"] }),
      });
      assert.equal(res.status, 401);
    });
  });

  it("令牌正确且模型就绪时返回向量", async () => {
    await withServer({ state: "ready" }, async (url) => {
      const res = await fetch(`${url}/api/agent/embed`, {
        method: "POST",
        headers: { "content-type": "application/json", "X-Internal-Token": TOKEN },
        body: JSON.stringify({ texts: ["a", "b"] }),
      });
      assert.equal(res.status, 200);
      const body = (await res.json()) as { embeddings: number[][] };
      assert.equal(body.embeddings.length, 2);
    });
  });

  // 这是拆分 embed 服务的核心收益：模型没就绪时明确 503，而不是让调用方拿到 502 连环失败。
  it("模型未就绪返回 503 并带上原因", async () => {
    await withServer({ state: "error", error: "download blocked" }, async (url) => {
      const res = await fetch(`${url}/api/agent/embed`, {
        method: "POST",
        headers: { "content-type": "application/json", "X-Internal-Token": TOKEN },
        body: JSON.stringify({ texts: ["a"] }),
      });
      assert.equal(res.status, 503);
      const body = (await res.json()) as { message: string };
      assert.match(body.message, /download blocked/);
    });
  });

  it("texts 为空返回 400", async () => {
    await withServer({ state: "ready" }, async (url) => {
      const res = await fetch(`${url}/api/agent/embed`, {
        method: "POST",
        headers: { "content-type": "application/json", "X-Internal-Token": TOKEN },
        body: JSON.stringify({ texts: [] }),
      });
      assert.equal(res.status, 400);
    });
  });

  it("超长批次被截断到 MAX_TEXTS，避免单请求打满服务", async () => {
    let seen = 0;
    await withServer(
      {
        state: "ready",
        embed: async (texts: string[]) => {
          seen = texts.length;
          return texts.map(() => [1]);
        },
      },
      async (url) => {
        const res = await fetch(`${url}/api/agent/embed`, {
          method: "POST",
          headers: { "content-type": "application/json", "X-Internal-Token": TOKEN },
          body: JSON.stringify({ texts: Array.from({ length: 500 }, (_, i) => String(i)) }),
        });
        assert.equal(res.status, 200);
        assert.equal(seen, MAX_TEXTS);
      },
    );
  });

  it("向量化抛错时返回 500 而不是挂起连接", async () => {
    await withServer(
      {
        state: "ready",
        embed: async () => {
          throw new Error("boom");
        },
      },
      async (url) => {
        const res = await fetch(`${url}/api/agent/embed`, {
          method: "POST",
          headers: { "content-type": "application/json", "X-Internal-Token": TOKEN },
          body: JSON.stringify({ texts: ["a"] }),
        });
        assert.equal(res.status, 500);
      },
    );
  });
});
