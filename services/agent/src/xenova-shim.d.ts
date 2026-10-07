// @xenova/transformers 未随包提供类型声明，这里给出极简模块声明以避免 tsc 报错。
// 运行期使用其运行时 API（pipeline / env）。该包为 CJS，使用默认导入（default）以兼容 ESM。
declare module "@xenova/transformers" {
  const env: any;
  function pipeline(task: string, model: string, options?: any): Promise<any>;
  const _default: { env: any; pipeline: typeof pipeline };
  export default _default;
}
