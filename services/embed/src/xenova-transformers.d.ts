// @xenova/transformers 未随包提供类型声明（TS7016），这里给出本项目实际用到的最小声明。
// 只声明被使用的 surface（pipeline / env），避免为整个库写不准确的 any 化声明。
declare module "@xenova/transformers" {
  /** 特征提取管道：调用后返回 { data: Float32Array, dims: number[] }。 */
  type FeatureExtractionPipeline = (
    text: string,
    options?: { pooling?: "mean" | "cls" | "none"; normalize?: boolean },
  ) => Promise<{ data: Float32Array | number[]; dims?: number[] }>;

  export const env: {
    allowRemoteModels: boolean;
    allowLocalModels: boolean;
    hubUrl: string;
    localModelPath: string;
    cacheDir?: string;
  };

  export function pipeline(
    task: "feature-extraction" | "embeddings" | string,
    model?: string,
    options?: Record<string, unknown>,
  ): Promise<FeatureExtractionPipeline>;

  const transformers: {
    env: typeof env;
    pipeline: typeof pipeline;
  };
  export default transformers;
}
