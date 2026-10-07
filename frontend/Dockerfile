# 构建上下文为仓库根目录。按 npm workspaces 在根目录安装，确保 @shared/api 的
# 依赖（axios 等）被正确提升，复现本地构建环境。
FROM node:20-alpine AS build
WORKDIR /build
# sharp@0.32.6（vite-plugin-pwa 的传递依赖）的安装脚本默认从 GitHub Releases 下载
# 预编译 libvips 二进制，受限网络下会超时。改为安装系统 libvips 与编译工具链，并保持
# SHARP_IGNORE_GLOBAL_LIBVIPS 不设置（默认）：sharp 会通过 pkg-config 探测到系统 libvips
# 并从源码编译 native addon，从而跳过下载。
# 注意：不可将其设为 false/1 等任意非空值——那样 sharp 会强制走下载路径反而超时。
RUN apk add --no-cache vips-dev libc6-compat build-base python3
COPY package.json package-lock.json ./
COPY packages/shared-api ./packages/shared-api
COPY frontend ./frontend
COPY services/agent ./services/agent
RUN npm install --no-audit --no-fund
RUN cd frontend && npm run build

FROM nginx:1.27-alpine
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /build/frontend/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
