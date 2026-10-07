// @xenova/transformers@1.4.x 在 Node 端导入时，onnxruntime-web（无 onnxruntime-node 时的回退）
// 会设置全局 self，导致 transformers 走「浏览器图像分支」，在顶层无条件引用 OffscreenCanvas /
// ImageData / self.createImageBitmap 等浏览器专有 API 而崩溃。我们仅做文本向量化，从不真正
// 解码图像，因此这里提供最小桩，让顶层赋值通过即可（桩方法不会被实际调用）。
// 本模块必须排在 transformers 导入之前执行。
const g = globalThis as any;

if (typeof g.OffscreenCanvas === "undefined") {
  g.OffscreenCanvas = class OffscreenCanvas {
    width = 0;
    height = 0;
    getContext(): null {
      return null;
    }
  };
}

if (typeof g.ImageData === "undefined") {
  g.ImageData = class ImageData {
    data: Uint8ClampedArray;
    width: number;
    height: number;
    constructor(data?: any, width?: number, height?: number) {
      this.data = data instanceof Uint8ClampedArray ? data : new Uint8ClampedArray(0);
      this.width = width ?? 0;
      this.height = height ?? 0;
    }
  };
}

if (typeof g.self === "undefined") {
  g.self = g;
}
const s = g.self || g;
if (typeof s.createImageBitmap === "undefined") {
  s.createImageBitmap = async (): Promise<null> => null;
}
