# 运维平台
```shell
# web
go get -u github.com/gin-gonic/gin

# 日志格式化
go get -u go.uber.org/zap
go get -u gopkg.in/natefinch/lumberjack.v2
# gin日志整合zap
go get -u github.com/gin-contrib/zap
go get -u github.com/gin-contrib/requestid

# gin prometheus
go get -u github.com/zsais/go-gin-prometheus

# jwt
go get -u github.com/golang-jwt/jwt/v5

# gorm mysql
go get -u gorm.io/gorm
go get -u gorm.io/driver/mysql

# casbin 
go get github.com/casbin/casbin/v3

# goroutine
go get github.com/ning1875/errgroup-signal

# 阿里云
go get github.com/aliyun/alibaba-cloud-sdk-go/services/ecs
go get github.com/gammazero/workerpool
go get go.uber.org/zap
go get k8s.io/apimachinery/pkg/util/wait

# aws


# 代码仓库
go get github.com/xanzy/go-gitlab
go get code.gitea.io/sdk/gitea
```

## swagger文档
```shell
# 1. 安装 swag 命令行生成工具
go install github.com/swaggo/swag/cmd/swag@latest
# 2. 安装 gin-swagger 运行时依赖
go get -u github.com/swaggo/gin-swagger
go get -u github.com/swaggo/files

# 每次新增/修改了 API 注解后，重新运行指令更新文档：
swag init -g cmd/server/main.go -o docs

```
## agent grpc相关
cd src/proto
protoc --go_out=. --go-grpc_out=. *.proto

# websocket
go get github.com/gorilla/websocket