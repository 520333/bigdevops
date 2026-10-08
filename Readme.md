# BigDevOps 大运维平台服务端

企业级一站式 DevOps 运维管理平台服务端，基于 **Go (Golang)** + **Gin** + **GORM** + **Casbin** + **gRPC** 架构开发，提供 CMDB 服务树、CI/CD 持续交付、Kubernetes 多集群管理、作业批量执行、Prometheus 监控告警联动等核心能力。

---

## 🏗️ 系统架构与核心组件

项目采用单仓多二进制组件（Monorepo Multi-Binary）架构：

| 组件名称 | 源码入口 | 说明 |
| :--- | :--- | :--- |
| **Server (主服务)** | `cmd/server/main.go` | 核心控制中心，提供 RESTful API、WebSocket 终端、Casbin 鉴权、定时任务及 gRPC Master 服务 |
| **Agent (主机节点代理)** | `cmd/agent/main.go` | 轻量级主机守护进程，通过 gRPC 与 Server 保持心跳长连接，执行命令与作业分发 |
| **Alert Webhook (告警中间件)** | `cmd/alert_webhook/main.go` | 接收 Prometheus Alertmanager 告警 Webhook，实现告警分发、抑制、升级以及飞书/钉钉推送 |

---

## 🛠️ 技术栈

- **Web 框架**：[Gin](https://github.com/gin-gonic/gin)
- **ORM & 存储**：[GORM](https://gorm.io/) + MySQL
- **权限与认证**：[Casbin v3](https://github.com/casbin/casbin) (RBAC 接口级权限控制) + [JWT v5](https://github.com/golang-jwt/jwt)
- **高性能通信**：gRPC + Protocol Buffers (支持双向流、心跳巡检与批量作业)
- **日志体系**：[Uber Zap](https://go.uber.org/zap) + Lumberjack (日志自动轮转与归档)
- **API 文档**：Swagger (swaggo/gin-swagger)
- **云与自动化集成**：
  - Jenkins API (`gojenkins`)
  - Kubernetes Client-Go
  - GitLab SDK (`go-gitlab`)
  - 阿里云 ECS SDK
- **SSO 单点登录**：OIDC (Keycloak/Authing) + 钉钉扫码登录

---

## 🚀 本地开发与启动

### 1. 环境准备
- **Go 语言环境**：`Go >= 1.20` (推荐 1.22+)
- **MySQL 数据库**：`>= 5.7` 或 `8.0`
- **配置文件**：
  - 复制模版配置：`cp server-template.yml server.yml`
  - 检查并修改 `server.yml` 中的数据库连接串与 Jenkins/K8s/GitLab 配置

### 2. 编译与本地运行

```bash
# 启动 Server 主服务
go run cmd/server/main.go

# 或者编译二进制运行
go build -o server cmd/server/main.go
./server -c server.yml

# 启动 Agent 节点守护端 (可选)
go run cmd/agent/main.go -c agent.yml

# 启动 Alert Webhook 告警服务 (可选)
go run cmd/alert_webhook/main.go -c alert_webhook.yml
```

---

## 📁 目录结构概览

```text
bigdevops/
├── cmd/                      # 应用程序入口
│   ├── server/               # 主服务入口 (cmd/server/main.go)
│   ├── agent/                # 节点端入口 (cmd/agent/main.go)
│   └── alert_webhook/        # 告警服务入口 (cmd/alert_webhook/main.go)
├── src/                      # 核心业务实现源码
│   ├── agent/                # Agent 采集与上报业务逻辑
│   ├── alert_webhook/        # 告警解析、路由升级与消息投递
│   ├── cache/                # 内存缓存 (Jenkins 客户端池、用户在线会话等)
│   ├── common/               # 全局通用常量、响应统一结构体 (Ok/Fail)、辅助函数
│   ├── config/               # YAML 配置文件解析映射结构体
│   ├── cron/                 # 后台定时任务调度 (Jenkins 状态同步、资源巡检等)
│   ├── models/               # 数据模型定义 (GORM 表结构、数据库初始化)
│   ├── rpc/                  # gRPC 协议与远程调用服务实现
│   └── web/                  # Web API 业务逻辑
│       ├── middleware/       # Gin 中间件 (JWT 认证、Casbin 鉴权、审计日志、Zap 访问日志)
│       └── view_server/      # RESTful 接口控制器 (CI/CD, K8s, 服务树, 角色, 用户等)
├── docs/                     # Swagger 自动生成的 API 文档资源
├── deploy/                   # 独立 Dockerfile 与部署配置
├── sql/                      # 基础数据库初始化脚本
├── server.yml                # Server 运行时配置文件
├── agent.yml                 # Agent 配置文件
└── docker-compose.yml        # Docker Compose 容器编排文件
```

---

## 🔧 开发常用指令

### 1. Swagger API 文档生成
```bash
# 安装 swag 命令行生成工具
go install github.com/swaggo/swag/cmd/swag@latest

# 更新 API 注解后重新生成文档
swag init -g cmd/server/main.go -o docs
```
> 生成后访问本地 Swagger 页面：`http://localhost:8080/swagger/index.html`

### 2. gRPC Proto 编译
```bash
# 进入 proto 目录编译生成 pb.go 和 grpc.pb.go
cd src/rpc/proto  # 或 src/proto
protoc --go_out=. --go-grpc_out=. *.proto
```

### 3. 核心依赖安装参考
```bash
# Web 框架与日志
go get -u github.com/gin-gonic/gin
go get -u go.uber.org/zap
go get -u gopkg.in/natefinch/lumberjack.v2
go get -u github.com/gin-contrib/zap
go get -u github.com/gin-contrib/requestid

# 数据库与鉴权
go get -u gorm.io/gorm
go get -u gorm.io/driver/mysql
go get github.com/casbin/casbin/v3
go get -u github.com/golang-jwt/jwt/v5

# 监控与第三方生态
go get -u github.com/zsais/go-gin-prometheus
go get github.com/bndr/gojenkins
go get github.com/gorilla/websocket
go get github.com/xanzy/go-gitlab
go get github.com/aliyun/alibaba-cloud-sdk-go/services/ecs
go get github.com/coreos/go-oidc/v3/oidc
```

---

## 🐳 Docker 镜像构建与部署

### 1. 使用通用 Dockerfile 构建镜像
```bash
# 构建 server 镜像
docker build -t bigdevops-server:latest --build-arg APP_NAME=server .

# 构建 agent 镜像
docker build -t bigdevops-agent:latest --build-arg APP_NAME=agent .

# 构建 alert_webhook 镜像
docker build -t bigdevops-alert-webhook:latest --build-arg APP_NAME=alert_webhook .
```

### 2. 使用专用 Dockerfile 构建指定服务
```bash
docker build -t bigdevops-server:latest -f deploy/Dockerfile.server .
docker build -t bigdevops-agent:latest -f deploy/Dockerfile.agent .
docker build -t bigdevops-alert-webhook:latest -f deploy/Dockerfile.alert_webhook .
```

### 3. Docker Compose 一键启动所有服务
```bash
docker-compose up -d --build
```