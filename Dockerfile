# 阶段一：构建前端静态资源
FROM node:22-alpine AS web-builder

WORKDIR /src
COPY web/package.json web/pnpm-lock.yaml ./web/
RUN corepack enable && corepack pnpm --dir web install --frozen-lockfile
COPY web ./web
RUN mkdir -p /src/internal/static/assets
RUN corepack pnpm --dir web build

# 阶段二：构建后端
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
COPY --from=web-builder /src/internal/static/assets ./internal/static/assets

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /codex-proxy .

# 阶段三：运行
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app
COPY --from=builder /codex-proxy /app/codex-proxy
COPY config.example.yaml /app/config.example.yaml
COPY docker/entrypoint.sh /app/entrypoint.sh

RUN mkdir -p /app/auths && chmod +x /app/entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
