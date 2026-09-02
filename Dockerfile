# ==========================================
# 第一阶段：构建阶段 (Builder Stage)
# ==========================================
FROM golang:1.26.1-alpine AS builder

# 设置环境变量
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOPROXY=https://goproxy.cn,direct \
    GOTOOLCHAIN=auto

# 安装基础构建工具及 CA 证书
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /build

# 先复制依赖定义文件，利用 Docker 层级缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制项目源代码
COPY . .

# 自动生成 Swagger API 文档（防止服务器构建环境中缺失 docs 目录）
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.3 \
    && swag init -g cmd/server/main.go -o docs || true

# 编译的目标服务（可选：server, agent, alert_webhook），默认为 server
ARG APP_NAME=server

# 编译 Go 可执行程序（关闭 CGO，去除符号表和调试信息以减小体积）
RUN go build -ldflags="-s -w" -o /build/app ./cmd/${APP_NAME}

# ==========================================
# 第二阶段：运行阶段 (Runner Stage)
# ==========================================
FROM alpine:3.20 AS runner

# 设置时区和安装基础运行时依赖
RUN apk add --no-cache ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# 从构建阶段复制编译好的可执行二进制文件
COPY --from=builder /build/app /app/app

# 默认拷贝对应服务的配置文件（如果存在）
ARG APP_NAME=server
COPY ${APP_NAME}.yml* /app/

# 暴露常用的服务端口
# server: 8080 (HTTP), 8083 (gRPC)
# agent: 8082 (HTTP)
# alert_webhook: 8085 (HTTP)
EXPOSE 8080 8083 8082 8085

# 启动应用程序
ENTRYPOINT ["/app/app"]
