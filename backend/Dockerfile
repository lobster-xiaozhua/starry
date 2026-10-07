# 多阶段构建：编译 Go 服务，最终镜像仅含二进制与 CA 证书。
# go.mod 要求 go 1.26，基础镜像直接使用 golang:1.26-alpine。
FROM golang:1.26-alpine AS build
# 模块代理：默认官方代理在部分网络不可达，此处使用可达镜像；可按部署环境覆盖 GOPROXY。
ENV GOPROXY=https://mirrors.tencent.com/go/,direct
ENV GOSUMDB=off
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# VERSION 由 compose/CI 传入（默认取 git 短哈希），注入到 /health 的 version 字段。
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/server /app/server
RUN mkdir -p /data/uploads
VOLUME ["/data/uploads"]
EXPOSE 8080
CMD ["/app/server"]
