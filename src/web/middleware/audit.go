package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"strings"
	"time"

	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuditLogMiddleWare 操作审计中间件
func AuditLogMiddleWare() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		path := c.Request.URL.Path

		// 记录常规写操作 (POST, PUT, DELETE)
		if method != "POST" && method != "PUT" && method != "DELETE" {
			c.Next()
			return
		}

		// 排除日志查询、健康检查以及由“登录日志”独立管理的认证接口
		pathLower := strings.ToLower(path)
		if strings.Contains(path, "/getAuditLogList") ||
			strings.Contains(path, "/getLoginLogList") ||
			strings.Contains(path, "/getOnlineUserList") ||
			strings.Contains(path, "/health") ||
			strings.Contains(pathLower, "login") ||
			strings.Contains(pathLower, "logout") ||
			strings.Contains(pathLower, "oidc") ||
			strings.Contains(pathLower, "dingtalk") {
			c.Next()
			return
		}

		startTime := time.Now()

		// 读取 Request Body 并放回，避免影响后续 c.ShouldBind
		var reqBodyStr string
		if c.Request.Body != nil {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				reqBodyStr = sanitizeRequestBody(bodyBytes)
			}
		}

		// 执行后续业务逻辑
		c.Next()

		latency := time.Since(startTime).Milliseconds()
		statusCode := c.Writer.Status()

		// 提取当前操作用户
		userName := ""
		if val, exists := c.Get(common.GIN_CTX_JWT_USER_NAME); exists {
			userName, _ = val.(string)
		}
		// 如果上下文未提取到用户名（如退出登录 /logout 或未挂载 JWT 中间件接口），尝试从 Authorization Header 解析
		if userName == "" {
			token := c.GetHeader("Authorization")
			if token != "" {
				parts := strings.SplitN(token, " ", 2)
				tokenStr := token
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenStr = parts[1]
				}
				if scInter, ok := c.Get(common.GIN_CTX_CONFIG_CONFIG); ok && scInter != nil {
					if sc, ok2 := scInter.(*config.ServerConfig); ok2 {
						if claims, err := models.ParseToken(tokenStr, sc); err == nil && claims != nil {
							userName = claims.Username
						}
					}
				}
			}
		}
		// 如果依然未提取到用户名（如常规账号密码登录请求），尝试从请求报文中提取
		if userName == "" && strings.Contains(strings.ToLower(path), "login") && reqBodyStr != "" {
			var loginReq map[string]interface{}
			if err := json.Unmarshal([]byte(reqBodyStr), &loginReq); err == nil {
				if u, ok := loginReq["username"].(string); ok && u != "" {
					userName = u
				} else if u, ok := loginReq["userName"].(string); ok && u != "" {
					userName = u
				} else if u, ok := loginReq["account"].(string); ok && u != "" {
					userName = u
				}
			}
		}

		// 提取客户端真实 IP (智能穿透代理并自动将IPv6回环::1转换为127.0.0.1)
		clientIP := GetRealClientIP(c)

		var logger *zap.Logger
		if scInter, ok := c.Get(common.GIN_CTX_CONFIG_CONFIG); ok && scInter != nil {
			if sc, ok2 := scInter.(*config.ServerConfig); ok2 {
				logger = sc.Logger
			}
		}

		// 异步入库，保证零延迟
		go func(uName, uMethod, uPath, uIp, reqStr string, status int, cost int64, l *zap.Logger) {
			// 查询当前用户信息
			var userID uint
			var realName string
			if uName != "" {
				user, err := models.GetUserByUsername(uName)
				if err == nil && user != nil {
					userID = user.ID
					realName = user.RealName
				}
			} else {
				uName = "匿名用户/外部调用"
			}

			module, action := deduceModuleAndAction(uPath, uMethod, status)

			auditLog := &models.SystemAuditLog{
				UserID:   userID,
				Username: uName,
				RealName: realName,
				Module:   module,
				Action:   action,
				Method:   uMethod,
				Path:     uPath,
				Ip:       uIp,
				Status:   status,
				Latency:  cost,
				ReqBody:  reqStr,
			}

			if err := auditLog.Create(); err != nil && l != nil {
				l.Error("保存操作审计日志失败", zap.Error(err), zap.String("path", uPath))
			}
		}(userName, method, path, clientIP, reqBodyStr, statusCode, latency, logger)
	}
}

// RecordAuditLogManual 手动记录一条审计日志（用于单点登录、外部回调等特定流程）
func RecordAuditLogManual(c *gin.Context, userID uint, username, realName, module, action, method, path string, status int, latency int64, reqBody string) {
	clientIP := GetRealClientIP(c)
	var logger *zap.Logger
	if scInter, ok := c.Get(common.GIN_CTX_CONFIG_CONFIG); ok && scInter != nil {
		if sc, ok2 := scInter.(*config.ServerConfig); ok2 {
			logger = sc.Logger
		}
	}

	go func(l *zap.Logger) {
		auditLog := &models.SystemAuditLog{
			UserID:   userID,
			Username: username,
			RealName: realName,
			Module:   module,
			Action:   action,
			Method:   method,
			Path:     path,
			Ip:       clientIP,
			Status:   status,
			Latency:  latency,
			ReqBody:  reqBody,
		}
		if err := auditLog.Create(); err != nil && l != nil {
			l.Error("手动保存操作审计日志失败", zap.Error(err), zap.String("path", path))
		}
	}(logger)
}

// sanitizeRequestBody 对请求报文中的敏感字段（密码、秘钥等）进行脱敏处理
func sanitizeRequestBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	// 尝试作为 JSON 处理
	var jsonObj interface{}
	if err := json.Unmarshal(body, &jsonObj); err != nil {
		// 非 JSON 直接截取长度
		s := string(body)
		if len(s) > 2000 {
			return s[:2000] + "...(截断)"
		}
		return s
	}

	maskSensitiveFields(jsonObj)
	sanitizedBytes, err := json.Marshal(jsonObj)
	if err != nil {
		return string(body)
	}

	result := string(sanitizedBytes)
	if len(result) > 4000 {
		return result[:4000] + "...(截断)"
	}
	return result
}

// maskSensitiveFields 递归脱敏敏感键
func maskSensitiveFields(obj interface{}) {
	sensitiveKeys := map[string]bool{
		"password":    true,
		"reqpassword": true,
		"oldpassword": true,
		"newpassword": true,
		"secret":      true,
		"accesstoken": true,
		"token":       true,
		"signing_key": true,
	}

	switch val := obj.(type) {
	case map[string]interface{}:
		for k, v := range val {
			kLower := strings.ToLower(k)
			if sensitiveKeys[kLower] || strings.Contains(kLower, "password") {
				val[k] = "******"
			} else {
				maskSensitiveFields(v)
			}
		}
	case []interface{}:
		for _, item := range val {
			maskSensitiveFields(item)
		}
	}
}

// deduceModuleAndAction 根据路径、方法与状态码智能推断模块名与动作
func deduceModuleAndAction(path, method string, status int) (string, string) {
	p := strings.ToLower(path)
	module := "系统服务"

	if strings.Contains(p, "/login") {
		if status >= 200 && status < 300 {
			return "用户认证", "登录成功"
		}
		return "用户认证", "登录失败"
	} else if strings.Contains(p, "/logout") {
		return "用户认证", "退出登录"
	} else if strings.Contains(p, "/system/account") || strings.Contains(p, "/system/user") {
		module = "用户管理"
	} else if strings.Contains(p, "/system/role") {
		module = "角色管理"
	} else if strings.Contains(p, "/system/menu") {
		module = "菜单管理"
	} else if strings.Contains(p, "/system/api") {
		module = "接口授权"
	} else if strings.Contains(p, "/system/settings") {
		module = "系统设置"
	} else if strings.Contains(p, "/password") {
		module = "密码中心"
	} else if strings.Contains(p, "/servicetree") || strings.Contains(p, "/stree") {
		module = "CMDB服务树"
	} else if strings.Contains(p, "/monitor/promscrapejob") || strings.Contains(p, "/scrapejob") {
		module = "采集任务"
	} else if strings.Contains(p, "/monitor/promscrapepool") || strings.Contains(p, "/prompool") {
		module = "采集池管理"
	} else if strings.Contains(p, "/monitor/promalertrule") {
		module = "告警规则"
	} else if strings.Contains(p, "/monitor/promrecordrule") {
		module = "聚合规则"
	} else if strings.Contains(p, "/monitor/alertmanagersendgroup") || strings.Contains(p, "/sendgroup") {
		module = "告警发送组"
	} else if strings.Contains(p, "/monitor/alertevent") || strings.Contains(p, "/event") {
		module = "告警事件"
	} else if strings.Contains(p, "/monitor/onduty") {
		module = "值班排班"
	} else if strings.Contains(p, "/workorder") {
		module = "工单服务"
	} else if strings.Contains(p, "/k8s") {
		module = "容器集群"
	} else if strings.Contains(p, "/cicd") {
		module = "持续交付"
	} else if strings.Contains(p, "/jobexec") {
		module = "任务执行"
	} else if strings.Contains(p, "/artifactory") {
		module = "制品库"
	}

	action := "数据提交"
	if strings.Contains(p, "create") || strings.Contains(p, "add") {
		action = "新增"
	} else if strings.Contains(p, "update") || strings.Contains(p, "edit") || strings.Contains(p, "set") || strings.Contains(p, "change") {
		action = "修改"
	} else if strings.Contains(p, "delete") || strings.Contains(p, "remove") || method == "DELETE" {
		action = "删除"
	} else if strings.Contains(p, "reling") {
		action = "认领"
	} else if strings.Contains(p, "silence") {
		action = "屏蔽"
	} else if strings.Contains(p, "unsilence") {
		action = "解除屏蔽"
	} else if strings.Contains(p, "exec") || strings.Contains(p, "run") {
		action = "执行"
	} else if strings.Contains(p, "deploy") {
		action = "发布"
	} else if method == "POST" {
		action = "提交操作"
	}

	return module, action
}

// GetRealClientIP 获取真实的客户端IP地址
func GetRealClientIP(c *gin.Context) string {
	// 1. 优先获取反向代理/网关头 X-Forwarded-For
	xForwardedFor := c.Request.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		// X-Forwarded-For 格式为: client, proxy1, proxy2
		parts := strings.Split(xForwardedFor, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" && part != "unknown" {
				return normalizeIP(part)
			}
		}
	}

	// 2. 其次尝试 X-Real-IP
	xRealIP := c.Request.Header.Get("X-Real-IP")
	if xRealIP != "" && xRealIP != "unknown" {
		return normalizeIP(xRealIP)
	}

	// 3. 再次获取 Gin 封装的 ClientIP
	clientIP := c.ClientIP()
	if clientIP != "" && clientIP != "unknown" {
		return normalizeIP(clientIP)
	}

	// 4. 最后兜底 RemoteAddr
	remoteIP, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil && remoteIP != "" {
		return normalizeIP(remoteIP)
	}

	return normalizeIP(c.Request.RemoteAddr)
}

// normalizeIP 规范化IP展示格式
func normalizeIP(ip string) string {
	ip = strings.TrimSpace(ip)
	// 本地 IPv6 回环地址 ::1 转为通用易读的 127.0.0.1
	if ip == "::1" || ip == "0:0:0:0:0:0:0:1" || ip == "[::1]" {
		return "127.0.0.1"
	}
	// 去除 IPv6 映射 IPv4 的前缀 ::ffff:
	if strings.HasPrefix(ip, "::ffff:") {
		return strings.TrimPrefix(ip, "::ffff:")
	}
	return ip
}
