import {
  searchNotesTool,
  getNoteTool,
  createNoteTool,
  updateNoteTool,
  listTagsTool,
} from "./notes.js";
import { webSearchTool, webFetchTool } from "./web.js";
import { calculatorTool, currentTimeTool } from "./utils.js";
import { knowledgeSearchTool } from "./knowledge.js";
import { messageAgentTool, readMessagesTool } from "./agentbus.js";

// 笔记工具：操作当前用户的云笔记（既有能力）。
export const noteTools = [
  searchNotesTool,
  getNoteTool,
  createNoteTool,
  updateNoteTool,
  listTagsTool,
];

// 网络工具：免费搜索与网页抓取。
export const webTools = [webSearchTool, webFetchTool];

// 通用工具：计算与时间。
export const utilTools = [calculatorTool, currentTimeTool];

// 知识库与互联工具。
export const knowledgeTools = [knowledgeSearchTool];
export const busTools = [messageAgentTool, readMessagesTool];

// 工具注册表：新增工具时在此数组追加即可被 agent 使用。
export const allTools = [
  ...noteTools,
  ...webTools,
  ...utilTools,
  ...knowledgeTools,
  ...busTools,
];
