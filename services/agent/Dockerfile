# Node LangGraph agent 服务。tsx 为 devDependency，保留 dev 安装以直接运行 TS。
# 使用 Debian (glibc) 基础镜像以确保 @xenova/transformers 的原生 onnxruntime 可加载。
FROM node:20-bookworm
WORKDIR /app
COPY package.json ./
RUN npm install --no-audit --no-fund
COPY . .
# 预热本地向量化模型（约 25MB，首次运行免联网下载）
RUN node -e "import('@xenova/transformers').then(m=>m.pipeline('feature-extraction','Xenova/all-MiniLM-L6-v2')).then(()=>console.log('[agent] embedding model warmed up')).catch(e=>console.warn('[agent] warmup skipped:',e.message))" || true
EXPOSE 3001
CMD ["npm", "start"]
