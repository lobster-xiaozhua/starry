# Stage 1: build TypeScript
FROM node:20-bookworm AS build
WORKDIR /build
COPY package.json package-lock.json* ./
# npm 源：默认国内镜像（受限网络下 registry.npmjs.org 常 DNS 解析失败；实测腾讯云镜像延迟最低）；
# 可按部署环境覆盖：--build-arg NPM_REGISTRY=https://registry.npmjs.org
ARG NPM_REGISTRY=https://mirrors.cloud.tencent.com/npm/
RUN --mount=type=cache,target=/root/.npm \
    npm config set registry "$NPM_REGISTRY" \
    && npm config set fetch-retries 5 \
    && npm config set fetch-retry-maxtimeout 120000 \
    && npm ci --no-audit --no-fund
COPY . .
ENV NODE_OPTIONS=--max-old-space-size=1536
RUN npx tsc --outDir dist

# Stage 2: runtime (pre-compiled, no tsx/typescript in runtime)
# Debian (glibc) base for @xenova/transformers native onnxruntime.
FROM node:20-bookworm AS runtime
WORKDIR /app
COPY --from=build /build/package.json ./
COPY --from=build /build/node_modules ./node_modules
COPY --from=build /build/dist ./dist
RUN mkdir -p /data/models && chown -R node /data
# Pre-warm embedding model (~25MB); failure does not block image build.
RUN node -e "import('@xenova/transformers').then(m=>m.pipeline('embeddings','Xenova/all-MiniLM-L6-v2')).then(()=>console.log('[embed] embedding model warmed up')).catch(e=>console.warn('[embed] warmup skipped:',e.message))" || true
USER node
EXPOSE 3002
CMD ["node", "dist/server.js"]
