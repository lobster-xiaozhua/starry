import { tool } from "@langchain/core/tools";
import { z } from "zod";
import { requestContext } from "../context.js";
import {
  searchNotesRequest,
  getNoteRequest,
  createNoteRequest,
  updateNoteRequest,
  listTagsRequest,
} from "../backend-client.js";

// 每个工具通过 requestContext 获取当前用户的 JWT token，
// 调用 Go 后端 REST API 操作该用户的笔记。

function getToken(): string {
  const ctx = requestContext.getStore();
  if (!ctx) throw new Error("no request context");
  return ctx.token;
}

export const searchNotesTool = tool(
  async ({ query }) => {
    const data = await searchNotesRequest(getToken(), query);
    const notes = (data as any).notes ?? [];
    const total = (data as any).total ?? 0;
    const summary = notes
      .map((n: any) => {
        const preview =
          n.body?.length > 120 ? n.body.slice(0, 120) + "…" : (n.body ?? "");
        return `【${n.title}】(id: ${n.id}) 标签: [${(n.tags ?? []).join(", ")}]\n${preview}`;
      })
      .join("\n\n");
    return `找到 ${total} 条笔记，展示前 ${notes.length} 条：\n\n${summary}`;
  },
  {
    name: "search_notes",
    description: "搜索当前用户的笔记。支持按标题、内容、标签全文检索。当用户询问笔记内容时使用。",
    schema: z.object({
      query: z.string().describe("搜索关键词"),
    }),
  },
);

export const getNoteTool = tool(
  async ({ noteId }) => {
    const note = await getNoteRequest(getToken(), noteId);
    const n = note as any;
    return `标题: ${n.title}\n标签: [${(n.tags ?? []).join(", ")}]\n创建: ${n.createdAt}\n更新: ${n.updatedAt}\n\n正文:\n${n.body ?? "(空)"}`;
  },
  {
    name: "get_note",
    description: "获取指定笔记的完整内容。需要笔记 ID，可从 search_notes 结果中获得。",
    schema: z.object({
      noteId: z.string().describe("笔记的 UUID"),
    }),
  },
);

export const createNoteTool = tool(
  async ({ title, body, tags }) => {
    const note = await createNoteRequest(getToken(), {
      title,
      body: body ?? "",
      tags: tags ?? [],
    });
    return `已创建笔记「${title}」，ID: ${(note as any).id}`;
  },
  {
    name: "create_note",
    description: "为当前用户创建一条新笔记。",
    schema: z.object({
      title: z.string().describe("笔记标题"),
      body: z.string().optional().describe("笔记正文内容（Markdown）"),
      tags: z.array(z.string()).optional().describe("标签列表"),
    }),
  },
);

export const updateNoteTool = tool(
  async ({ noteId, title, body, tags }) => {
    const payload: Record<string, any> = {};
    if (title !== undefined) payload.title = title;
    if (body !== undefined) payload.body = body;
    if (tags !== undefined) payload.tags = tags;
    await updateNoteRequest(getToken(), noteId, payload);
    return `已更新笔记 ${noteId}`;
  },
  {
    name: "update_note",
    description: "更新指定笔记的标题、正文或标签。只需传入要修改的字段。",
    schema: z.object({
      noteId: z.string().describe("笔记的 UUID"),
      title: z.string().optional().describe("新标题"),
      body: z.string().optional().describe("新正文内容（Markdown）"),
      tags: z.array(z.string()).optional().describe("新标签列表（会替换原有标签）"),
    }),
  },
);

export const listTagsTool = tool(
  async () => {
    const data = await listTagsRequest(getToken());
    const tags = (data as any) ?? [];
    if (!Array.isArray(tags) || tags.length === 0) return "当前没有任何标签。";
    return tags.map((t: any) => `#${t.name} (${t.count})`).join("  ");
  },
  {
    name: "list_tags",
    description: "列出当前用户的所有笔记标签及每个标签下的笔记数量。",
    schema: z.object({}),
  },
);
