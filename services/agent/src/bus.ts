// Agent 消息总线：基于 Redis 的「信箱 + 发布订阅」，
// 让主 agent 与子 agent 之间可以异步投递与收取消息，实现 agent 互联。
import Redis from "ioredis";

const REDIS_ADDR = process.env.REDIS_ADDR || "redis:6379";
const REDIS_PASSWORD = process.env.REDIS_PASSWORD || "";
const REDIS_DB = parseInt(process.env.REDIS_DB || "0", 10);

let pubClient: Redis | null = null;
let subClient: Redis | null = null;

function client(): Redis {
  if (!pubClient) {
    pubClient = new Redis(`redis://${REDIS_PASSWORD ? ":" + REDIS_PASSWORD + "@" : ""}${REDIS_ADDR}/${REDIS_DB}`);
    subClient = pubClient.duplicate();
  }
  return pubClient;
}

function channel(agent: string): string {
  return `agent:bus:${agent}`;
}
function mailboxKey(agent: string): string {
  return `agent:mailbox:${agent}`;
}

export interface AgentMessage {
  from: string;
  to: string;
  message: string;
  ts: number;
}

// 向目标 agent 投递一条消息：写入其持久信箱（可事后读取），并实时发布到频道。
export async function sendToAgent(to: string, from: string, message: string): Promise<AgentMessage> {
  const payload: AgentMessage = { from, to, message, ts: Date.now() };
  const c = client();
  await c.rpush(mailboxKey(to), JSON.stringify(payload));
  await c.publish(channel(to), JSON.stringify(payload));
  return payload;
}

// 读取并清空某 agent 的信箱（收取他人投递的消息）。
export async function readMailbox(agent: string): Promise<AgentMessage[]> {
  const c = client();
  const raw = await c.lrange(mailboxKey(agent), 0, -1);
  if (raw.length > 0) {
    await c.del(mailboxKey(agent));
  }
  return raw.map((r) => JSON.parse(r) as AgentMessage);
}

// 订阅某 agent 频道的实时消息（用于长程任务/调试场景）。
export function subscribeAgent(agent: string, cb: (m: AgentMessage) => void): () => void {
  const c = client();
  const handler = (ch: string, raw: string) => {
    if (ch === channel(agent)) {
      try {
        cb(JSON.parse(raw) as AgentMessage);
      } catch {
        /* ignore malformed */
      }
    }
  };
  c.on("message", handler);
  c.subscribe(channel(agent));
  return () => {
    c.off("message", handler);
    c.unsubscribe(channel(agent));
  };
}
