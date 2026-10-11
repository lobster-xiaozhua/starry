# Stage 1: build TypeScript
FROM node:20-alpine AS build
WORKDIR /build
COPY package.json package-lock.json* ./
# npm 源：默认国内镜像（受限网络下 registry.npmjs.org 常 DNS 解析失败；实测腾讯云镜像延迟最低）；
# 可按部署环境覆盖：--build-arg NPM_REGISTRY=https://registry.npmjs.org
ARG NPM_REGISTRY=https://mirrors.cloud.tencent.com/npm/
RUN --mount=type=cache,target=/root/.npm \
    npm config set registry "$NPM_REGISTRY" \
    && npm config set fetch-retries 5 \
    && npm config set fetch-retry-maxtimeout 120000 \
    && npm install --no-audit --no-fund
COPY . .
ENV NODE_OPTIONS=--max-old-space-size=1536
RUN npx tsc --outDir dist

# Stage 2: runtime (pre-compiled, no tsx/typescript in runtime)
FROM node:20-alpine AS runtime
WORKDIR /app
COPY --from=build /build/package.json ./
COPY --from=build /build/node_modules ./node_modules
COPY --from=build /build/dist ./dist
USER node
EXPOSE 3001
CMD ["node", "dist/server.js"]
