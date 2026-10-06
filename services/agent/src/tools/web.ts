import { tool } from "@langchain/core/tools";
import { z } from "zod";

// 免费网络搜索：直连 DuckDuckGo HTML 端点并解析结果，无需任何 API Key。
// 若 DDG 不可达（网络受限/被限流），返回明确提示而非崩溃，由 agent 自行兜底。
export const webSearchTool = tool(
  async ({ query, maxResults }) => {
    const url = `https://html.duckduckgo.com/html/?q=${encodeURIComponent(query)}`;
    try {
      const res = await fetch(url, {
        headers: { "User-Agent": "Mozilla/5.0 (compatible; StarryBot/1.0)" },
      });
      const html = await res.text();
      const results = parseDuckDuckGo(html);
      if (results.length === 0) {
        return "未从搜索引擎获取到结果（可能网络受限或被限流）。可尝试换一种表述，或使用知识库/笔记工具。";
      }
      const top = results.slice(0, maxResults || 5);
      return top
        .map(
          (r, i) =>
            `[${i + 1}] ${r.title}\n链接: ${r.link}\n摘要: ${r.snippet}`,
        )
        .join("\n\n");
    } catch (e: any) {
      return `网络搜索失败：${e?.message || e}。请检查运行环境网络，或改用知识库/笔记工具。`;
    }
  },
  {
    name: "web_search",
    description:
      "免费网络搜索（无需密钥）。当用户需要实时/公开网络信息、新闻、事实核实时使用。返回标题、链接与摘要。",
    schema: z.object({
      query: z.string().describe("搜索关键词"),
      maxResults: z.number().optional().describe("返回结果数量，默认 5"),
    }),
  },
);

// 抓取网页正文：去掉 HTML 标签，保留可读文本并截断，供 agent 精读。
export const webFetchTool = tool(
  async ({ url, maxChars }) => {
    try {
      const res = await fetch(url, {
        headers: { "User-Agent": "Mozilla/5.0 (compatible; StarryBot/1.0)" },
        redirect: "follow",
      });
      const html = await res.text();
      const text = stripHtml(html);
      const limit = maxChars && maxChars > 0 ? maxChars : 4000;
      return text.length > limit ? text.slice(0, limit) + "\n…(已截断)" : text;
    } catch (e: any) {
      return `抓取失败：${e?.message || e}`;
    }
  },
  {
    name: "web_fetch",
    description: "抓取指定网页并提取纯文本内容，用于精读页面。",
    schema: z.object({
      url: z.string().describe("目标网页 URL"),
      maxChars: z.number().optional().describe("最大返回字符数，默认 4000"),
    }),
  },
);

function parseDuckDuckGo(html: string): Array<{ title: string; link: string; snippet: string }> {
  const out: Array<{ title: string; link: string; snippet: string }> = [];
  // 每个结果块形如：<a class="result__a" href="LINK">TITLE</a> ... <a class="result__snippet">SNIPPET</a>
  const blockRe = /<a[^>]*class="result__a"[^>]*href="([^"]+)"[^>]*>([\s\S]*?)<\/a>/g;
  let m: RegExpExecArray | null;
  while ((m = blockRe.exec(html)) !== null) {
    const link = decodeHtml(m[1]);
    const title = decodeHtml(stripTags(m[2]));
    // 紧随其后的 snippet
    const after = html.slice(m.index + m[0].length, m.index + m[0].length + 600);
    const snip = after.match(/class="result__snippet"[^>]*>([\s\S]*?)<\/a>/);
    const snippet = snip ? decodeHtml(stripTags(snip[1])) : "";
    if (title) out.push({ title, link, snippet });
  }
  return out;
}

function stripTags(s: string): string {
  return s.replace(/<[^>]+>/g, "").replace(/\s+/g, " ").trim();
}

function stripHtml(html: string): string {
  return html
    .replace(/<script[\s\S]*?<\/script>/gi, " ")
    .replace(/<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/\s+/g, " ")
    .trim();
}

function decodeHtml(s: string): string {
  return s
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&#x27;/g, "'")
    .replace(/&nbsp;/g, " ");
}
