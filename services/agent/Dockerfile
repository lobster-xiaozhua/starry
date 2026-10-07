# Node LangGraph agent 服务（对话 / Agent 编排）。
# 注意：嵌入（向量化）已拆为独立服务 services/embed，本镜像不再包含
# @xenova/transformers / onnxruntime / libvips，因此构建更轻、启动更快，
# 且对话服务不再依赖 HuggingFace 模型下载的成败。
FROM node:20-bookworm
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY package.json ./
RUN npm install --no-audit --no-fund
COPY . .
EXPOSE 3001
CMD ["npm", "start"]
