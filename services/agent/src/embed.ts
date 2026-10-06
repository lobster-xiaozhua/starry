// 本地免费向量化：使用 @xenova/transformers 的 all-MiniLM-L6-v2（384 维），
// 首次调用会下载模型（约 25MB），之后在内存缓存。完全离线、零密钥。
import { pipeline, env } from "@xenova/transformers";

// 优先使用镜像，避免直连 HuggingFace 受限。可在 .env 通过 TRANSFORMERS_CACHE 指定缓存目录。
env.allowRemoteModels = true;

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
    extractor = await pipeline("feature-extraction", "Xenova/all-MiniLM-L6-v2");
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
