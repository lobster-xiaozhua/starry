# 独立向量化（嵌入）服务。仅承载 @xenova/transformers 的本地文本向量化，
# 与对话/Agent 编排服务（services/agent）解耦，因此对话不再依赖 HuggingFace 模型下载。
# 使用 Debian (glibc) 基础镜像以确保 @xenova/transformers 的原生 onnxruntime 可加载。
FROM node:20-bookworm
WORKDIR /app
# 说明：@xenova/transformers 依赖 sharp ^0.32，而 sharp 0.32 的安装脚本要从
# GitHub Releases 下载预编译 libvips，受限网络下会超时并让镜像构建失败。
# package.json 里已用 overrides 把 sharp 提到 0.33+——该版本把预编译二进制作为
# npm 可选平台包（@img/sharp-*）分发，只走 registry，因此这里不需要安装系统 libvips
# 或编译工具链。改动 overrides 时请同步评估是否需要恢复下面的系统依赖。
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
# 预热本地向量化模型（约 25MB，首次运行免联网下载）；失败不阻断镜像构建。
RUN node -e "import('@xenova/transformers').then(m=>m.pipeline('embeddings','Xenova/all-MiniLM-L6-v2')).then(()=>console.log('[embed] embedding model warmed up')).catch(e=>console.warn('[embed] warmup skipped:',e.message))" || true
EXPOSE 3002
CMD ["npm", "start"]
