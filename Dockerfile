# 多阶段构建
# 第一阶段：编译
FROM golang:1.24.3-alpine AS builder

# 安装必要的构建工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 编译应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server

# 第二阶段：运行时
FROM alpine:latest

# 创建非 root 用户
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata git git-lfs openssh-client

# 创建 /root/.ssh 目录并生成 known_hosts
RUN mkdir -p /root/.ssh \
    && ssh-keyscan -t rsa,ecdsa,ed25519 github.com > /root/.ssh/known_hosts \
    && chmod 755 /root/.ssh \
    && chmod 644 /root/.ssh/known_hosts \
    && chown -R appuser:appgroup /root/.ssh

# 设置工作目录
WORKDIR /app

# 从 builder 阶段复制编译好的应用
COPY --from=builder /app/main .

# 创建数据目录
RUN mkdir -p /app/data && chown -R appuser:appgroup /app

# 切换到非 root 用户
USER appuser

# 暴露端口
EXPOSE 37080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:37080/health || exit 1

# 启动应用
CMD ["./main"] 