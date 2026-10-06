# 构建上下文为仓库根目录。按 npm workspaces 在根目录安装，确保 @shared/api 的
# 依赖（axios 等）被正确提升，复现本地构建环境。
FROM node:20-alpine AS build
WORKDIR /build
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
