# syntax=docker/dockerfile:1
#
# 单镜像全栈构建：admin + user 两个 SPA 编译后由 go:embed 打进同一个二进制，
# 由同一个进程在同一端口（8080）提供服务。不再需要独立的前端容器与 nginx。

# ---- 阶段 1：构建两个前端 ----
# 固定在构建机原生架构上跑（前端产物与目标架构无关），避免 buildx 多平台构建时
# 在 QEMU 模拟的 arm64 下重跑一遍 Node —— 那样构建耗时会差一个数量级。
FROM --platform=$BUILDPLATFORM node:24.11.1-alpine AS frontend

WORKDIR /src

ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable

# 先只拷 manifest，让依赖安装层能在源码变动时命中缓存
COPY frontend/admin/package.json frontend/admin/pnpm-lock.yaml ./admin/
COPY frontend/user/package.json  frontend/user/pnpm-lock.yaml  ./user/
RUN cd admin && pnpm install --frozen-lockfile
RUN cd user  && pnpm install --frozen-lockfile

COPY frontend/admin ./admin
COPY frontend/user  ./user

# admin 走 fullstack 模式：注入 __DJ_ADMIN_BASE__ 占位符，
# 后端启动时按 web.admin_path 替换，使同一份产物能挂到任意自定义前缀。
RUN cd admin && pnpm run build:fullstack
RUN cd user  && pnpm run build

# ---- 阶段 2：构建内嵌前端的 Go 二进制 ----
# 版本必须 >= go.mod 的 go 指令，官方镜像默认 GOTOOLCHAIN=local 不会自动拉取更高工具链。
FROM --platform=$BUILDPLATFORM golang:1.26.9-bookworm@sha256:d9c68c2c51161e12fd77e4c6320687c9cd86e1af1e3ad6e6cd63ff970641453c AS builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG APP_VERSION=v1.0.0
RUN echo "Building for $TARGETOS/$TARGETARCH$TARGETVARIANT"

WORKDIR /src

ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# go:embed 只能读取包目录内的文件，所以前端产物必须落到 internal/web/dist/ 下
COPY --from=frontend /src/admin/dist ./internal/web/dist/admin
COPY --from=frontend /src/user/dist  ./internal/web/dist/user

RUN set -eux; \
    export GOOS="$TARGETOS" GOARCH="$TARGETARCH"; \
    if [ "$TARGETARCH" = "arm" ] && [ -n "$TARGETVARIANT" ]; then export GOARM="${TARGETVARIANT#v}"; fi; \
    if [ "$TARGETARCH" = "amd64" ] && [ -n "$TARGETVARIANT" ]; then export GOAMD64="${TARGETVARIANT#v}"; fi; \
    go build -trimpath -tags release,fullstack -ldflags="-s -w -X github.com/dujiao-next/internal/version.Version=${APP_VERSION} -X github.com/dujiao-next/internal/version.BuildType=release" -o /out/dujiao-next ./cmd/server; \
    go build -trimpath -ldflags="-s -w" -o /out/payment-ops ./cmd/payment-ops

# ---- 阶段 3：运行时 ----
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata \
    && mkdir -p /app/db /app/uploads /app/logs

COPY --from=builder /out/dujiao-next /app/dujiao-next
COPY --from=builder /out/payment-ops /app/payment-ops
COPY config.yml.example /app/config.yml.example

EXPOSE 8080

CMD ["./dujiao-next"]
