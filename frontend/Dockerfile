# 前端独立构建：仅安装 frontend 与 shared-api 两个 workspace 的依赖。
# 关键解耦：从 workspaces 中剔除所有 services/*（agent / embed）—— 它们各自是独立的 Node 服务，
# 自带 Dockerfile 与依赖，纳入前端镜像构建会显著拖慢速度且毫无收益（尤其 transformers/onnxruntime）。
FROM node:20-alpine AS build
WORKDIR /build
# sharp@0.32.6（vite-plugin-pwa 的传递依赖）的安装脚本默认从 GitHub Releases 下载预编译
# libvips 二进制，受限网络下会超时。改为安装系统 libvips 与编译工具链，并保持
# SHARP_IGNORE_GLOBAL_LIBVIPS 不设置（默认）：sharp 通过 pkg-config 探测系统 libvips 并从
# 源码编译 native addon，从而跳过下载。注意：不可将其设为 false/1 等任意非空值，否则 sharp
# 反而强制走下载路径并超时。
RUN apk add --no-cache vips-dev libc6-compat build-base python3
COPY package.json ./package.json
# 移除所有 services/* workspace 声明，避免 npm 解析其（含 transformers 等庞大）依赖。
RUN sed -i '/"services\//d' package.json
COPY packages/shared-api ./packages/shared-api
COPY frontend ./frontend
RUN npm install --no-audit --no-fund
RUN cd frontend && npm run build

FROM nginx:1.27-alpine
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /build/frontend/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
