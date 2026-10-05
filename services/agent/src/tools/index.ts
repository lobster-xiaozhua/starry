import {
  searchNotesTool,
  getNoteTool,
  createNoteTool,
  updateNoteTool,
  listTagsTool,
} from "./notes.js";

// 工具注册表：新增工具时在此数组追加即可被 agent 使用。
export const allTools = [
  searchNotesTool,
  getNoteTool,
  createNoteTool,
  updateNoteTool,
  listTagsTool,
];
