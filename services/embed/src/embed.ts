// 本地免费向量化：使用 @xenova/transformers 的 all-MiniLM-L6-v2（384 维），
// 首次调用会下载模型（约 25MB），之后在内存缓存。完全离线、零密钥。
// 该包为 CJS，Node 端需先补浏览器 API 桩（见 polyfill），再用默认导入以兼容 ESM 互操作。
import "./transformers-polyfill.js";
import transformers from "@xenova/transformers";

const { pipeline, env } = transformers;

// 允许远程下载模型（首次调用会从 hub 拉取并缓存到本地）。
env.allowRemoteModels = true;
// 支持通过环境变量指定 HuggingFace 镜像源（如 HF_ENDPOINT=https://hf-mirror.com），
// 以绕过受限网络直连 huggingface.co 失败的问题；未设置时回退官方源。
// 注：缓存目录仍可通过 TRANSFORMERS_CACHE 指定（compose 中挂载为持久卷）。
if (process.env.HF_ENDPOINT) {
  env.hubUrl = process.env.HF_ENDPOINT;
}

let extractor: any = null;
let warming = false;

async function getExtractor() {
  if (extractor) return extractor;
  if (warming) {
    // 并发预热时等待已在进行中的实例
    while (warming && !extractor) await new Promise((r) => setTimeout(r, 100));
    if (extractor) return extractor;
  }
  warming = true;
  try {
    extractor = await pipeline("embeddings", "Xenova/all-MiniLM-L6-v2");
  } finally {
    warming = false;
  }
  return extractor;
}

// embedTexts 批量向量化，返回 384 维归一化向量。
export async function embedTexts(texts: string[]): Promise<number[][]> {
  const ex = await getExtractor();
  const out: number[][] = [];
  for (const t of texts) {
    const res = await ex(t, { pooling: "mean", normalize: true });
    out.push(Array.from(res.data as Float32Array));
  }
  return out;
}
