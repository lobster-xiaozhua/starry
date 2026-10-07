# 独立向量化（嵌入）服务。仅承载 @xenova/transformers 的本地文本向量化，
# 与对话/Agent 编排服务（services/agent）解耦，因此对话不再依赖 HuggingFace 模型下载。
# 使用 Debian (glibc) 基础镜像以确保 @xenova/transformers 的原生 onnxruntime 可加载。
FROM node:20-bookworm
WORKDIR /app
# 安装系统 libvips：@xenova/transformers 传递依赖 sharp 用于图片处理，
# 受限网络下 sharp 无法从 GitHub Releases 下载预编译 libvips 二进制会导致构建失败；
# 装系统库后 sharp 自动复用、跳过下载（文本向量化无需图片能力）。
RUN apt-get update && apt-get install -y --no-install-recommends libvips ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY package.json ./
RUN npm install --no-audit --no-fund
COPY . .
# 预热本地向量化模型（约 25MB，首次运行免联网下载）；失败不阻断镜像构建。
RUN node -e "import('@xenova/transformers').then(m=>m.pipeline('embeddings','Xenova/all-MiniLM-L6-v2')).then(()=>console.log('[embed] embedding model warmed up')).catch(e=>console.warn('[embed] warmup skipped:',e.message))" || true
EXPOSE 3002
CMD ["npm", "start"]
