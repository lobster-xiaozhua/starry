# Node LangGraph agent 服务。tsx 为 devDependency，保留 dev 安装以直接运行 TS。
FROM node:20-alpine
WORKDIR /app
COPY package.json ./
RUN npm install --no-audit --no-fund
COPY . .
EXPOSE 3001
CMD ["npm", "start"]
