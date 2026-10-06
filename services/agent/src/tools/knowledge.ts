import { tool } from "@langchain/core/tools";
import { z } from "zod";
import { requestContext } from "../context.js";
import { backendFetch } from "../backend-client.js";

// 知识库检索：在用户私有知识库（RAG）内做向量相似度检索，返回带来源片段。
export const knowledgeSearchTool = tool(
  async ({ query, topK }) => {
    const ctx = requestContext.getStore();
    if (!ctx) return "无请求上下文，无法检索知识库。";
    try {
      const data = await backendFetch(
        `/api/knowledge/search?q=${encodeURIComponent(query)}&k=${topK || 5}`,
        ctx.token,
      );
      const results = (data as any)?.results ?? [];
      if (results.length === 0) {
        return "知识库中未找到相关内容。可尝试录入文档（POST /api/knowledge/ingest）或改用 web_search。";
      }
      return results
        .map(
          (r: any, i: number) =>
            `[片段${i + 1}] (doc: ${r.docId})\n${r.content}`,
        )
        .join("\n\n");
    } catch (e: any) {
      return `知识库检索失败：${e?.message || e}`;
    }
  },
  {
    name: "knowledge_search",
    description:
      "在用户私有知识库（RAG）中检索相关文档片段，用于回答基于用户自有资料的问题。返回片段含来源文档 ID。",
    schema: z.object({
      query: z.string().describe("检索问题/关键词"),
      topK: z.number().optional().describe("返回片段数，默认 5"),
    }),
  },
);

// 内部辅助：供 agent 服务其它模块（如长程任务）调用同一检索入口。
export async function searchKnowledge(token: string, query: string, topK = 5): Promise<any> {
  return backendFetch(`/api/knowledge/search?q=${encodeURIComponent(query)}&k=${topK}`, token);
}
