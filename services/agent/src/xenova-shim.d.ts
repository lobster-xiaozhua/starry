// @xenova/transformers 未随包提供类型声明，这里给出极简模块声明以避免 tsc 报错。
// 运行期使用其运行时 API（pipeline / env）。
declare module "@xenova/transformers" {
  export const env: any;
  export function pipeline(task: string, model: string, options?: any): Promise<any>;
}
