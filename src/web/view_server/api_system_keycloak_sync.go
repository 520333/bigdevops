package view_server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"bigdevops/src/web/middleware"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

// KeycloakUserItem Keycloak Admin API 返回的用户结构
type KeycloakUserItem struct {
	ID         string                 `json:"id"`
	Username   string                 `json:"username"`
	FirstName  string                 `json:"firstName"`
	LastName   string                 `json:"lastName"`
	Email      string                 `json:"email"`
	Enabled    bool                   `json:"enabled"`
	Attributes map[string]interface{} `json:"attributes"`
}

// KeycloakTokenResp 客户端凭证模式获取的访问 Token
type KeycloakTokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// @Summary      同步 Keycloak 全量用户
// @Description  通过 Keycloak Admin REST API 批量同步所有用户到本地数据库
// @Tags         system-user
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "同步结果"
// @Router       /system/syncKeycloakUsers [post]
// @Security     Bearer
func syncKeycloakUsers(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	if sc.OIDC == nil || !sc.OIDC.Enable {
		common.ReqBadFailWithMessage("系统未启用 OIDC/Keycloak 单点登录配置", c)
		return
	}

	if sc.OIDC.Issuer == "" || sc.OIDC.ClientID == "" || sc.OIDC.ClientSecret == "" {
		common.ReqBadFailWithMessage("Keycloak 配置不完整（Issuer, ClientID, ClientSecret 不能为空）", c)
		return
	}

	client := resty.New().SetTimeout(15 * time.Second)

	// 1. 通过 Client Credentials 授权模式获取 Keycloak 服务访问凭据 access_token
	tokenURL := fmt.Sprintf("%s/protocol/openid-connect/token", strings.TrimRight(sc.OIDC.Issuer, "/"))
	var tokenResp KeycloakTokenResp

	resp, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormData(map[string]string{
			"grant_type":    "client_credentials",
			"client_id":     sc.OIDC.ClientID,
			"client_secret": sc.OIDC.ClientSecret,
		}).
		SetResult(&tokenResp).
		Post(tokenURL)

	if err != nil || !resp.IsSuccess() || tokenResp.AccessToken == "" {
		errMsg := tokenResp.ErrorDesc
		if errMsg == "" {
			errMsg = tokenResp.Error
		}
		if errMsg == "" {
			errMsg = resp.String()
		}
		sc.Logger.Error("获取 Keycloak Service Account 访问令牌失败",
			zap.Error(err),
			zap.String("token_url", tokenURL),
			zap.String("response", resp.String()),
		)
		common.ReqBadFailWithMessage(fmt.Sprintf("向 Keycloak 换取管理员令牌失败：%s。请确认 Keycloak Client 是否已开启 'Service Accounts Roles'", errMsg), c)
		return
	}

	// 2. 计算 Keycloak Admin API 用户列表地址
	// 标准规范：Issuer 形如 http://host/realms/xxx 或 http://host/auth/realms/xxx
	// Admin 端点对应为 http://host/admin/realms/xxx/users 或 http://host/auth/admin/realms/xxx/users
	adminUsersURL := strings.Replace(strings.TrimRight(sc.OIDC.Issuer, "/"), "/realms/", "/admin/realms/", 1) + "/users"

	var keycloakUsers []KeycloakUserItem
	resp, err = client.R().
		SetAuthToken(tokenResp.AccessToken).
		SetQueryParam("max", "2000"). // 默认单次拉取最多2000名用户
		SetQueryParam("briefRepresentation", "false").
		SetResult(&keycloakUsers).
		Get(adminUsersURL)

	tokenRoleInfo := extractTokenRolesInfo(tokenResp.AccessToken)
	sc.Logger.Info("Keycloak Service Account Token 解析角色详情",
		zap.String("roles", tokenRoleInfo),
		zap.String("admin_url", adminUsersURL),
	)

	if err != nil || !resp.IsSuccess() {
		sc.Logger.Error("调用 Keycloak Admin API 拉取用户列表失败",
			zap.Error(err),
			zap.String("admin_url", adminUsersURL),
			zap.String("token_roles", tokenRoleInfo),
			zap.String("response", resp.String()),
		)
		common.ReqBadFailWithMessage(fmt.Sprintf("从 Keycloak 拉取用户列表失败：%s。当前令牌角色: [%s]。请检查 Client 的【客户端范围 -> %s-dedicated -> 作用域映射】中的【允许全部范围 (Full scope allowed)】是否开启", resp.String(), tokenRoleInfo, sc.OIDC.ClientID), c)
		return
	}

	if len(keycloakUsers) == 0 {
		common.OkWithDetailed(gin.H{
			"total":   0,
			"created": 0,
			"updated": 0,
		}, "Keycloak 中未发现有效用户", c)
		return
	}

	// 3. 查找系统默认普通角色 (user)，新创建的用户默认赋予此角色
	var defaultRole *models.SystemRole
	if r, rErr := models.GetRoleByRoleValue("user"); rErr == nil && r != nil {
		defaultRole = r
	}

	createdCount := 0
	updatedCount := 0

	for _, ku := range keycloakUsers {
		username := strings.TrimSpace(ku.Username)
		if username == "" {
			continue
		}

		// 提取显示名 / 真实姓名（优先使用 Keycloak 自定义属性 zh_name / displayName，其次中文拼装 姓+名）
		realName := extractDisplayName(ku)

		enableStatus := 1
		if !ku.Enabled {
			enableStatus = 0
		}

		// 检查本地数据库是否已存在该账号
		existingUser, getErr := models.GetUserByUsername(username)
		if getErr == nil && existingUser != nil && existingUser.ID > 0 {
			// 已存在：增量同步更新信息（更新姓名、邮箱、启用状态）
			updates := map[string]interface{}{}
			if realName != "" && existingUser.RealName != realName {
				updates["real_name"] = realName
			}
			if ku.Email != "" && existingUser.Email != ku.Email {
				updates["email"] = ku.Email
			}
			if existingUser.Enable != enableStatus {
				updates["enable"] = enableStatus
			}

			if len(updates) > 0 {
				_ = models.Db.Model(existingUser).Updates(updates)
				updatedCount++
			}
		} else {
			// 不存在：自动开户新建
			newUser := &models.SystemUser{
				Username: username,
				RealName: realName,
				Email:    ku.Email,
				Enable:   enableStatus,
				HomePath: "/dashboard/analysis",
				Password: common.BcryptHash("123456"), // 初始临时密码
			}
			if cErr := newUser.CreateOne(); cErr == nil {
				createdCount++
				// 分配默认普通角色
				if defaultRole != nil {
					_ = models.Db.Model(newUser).Association("Roles").Append(defaultRole)
				}
			}
		}
	}

	// 4. 记录系统操作审计日志
	currUser := ""
	if val, exists := c.Get(common.GIN_CTX_JWT_USER_NAME); exists {
		currUser, _ = val.(string)
	}
	middleware.RecordAuditLogManual(
		c,
		0,
		currUser,
		currUser,
		"用户管理",
		"同步Keycloak用户",
		c.Request.Method,
		c.Request.URL.Path,
		200,
		0,
		fmt.Sprintf(`{"total":%d,"created":%d,"updated":%d}`, len(keycloakUsers), createdCount, updatedCount),
	)

	resultMsg := fmt.Sprintf("同步完成：从 Keycloak 获取 %d 个用户，新增 %d 人，更新 %d 人", len(keycloakUsers), createdCount, updatedCount)
	sc.Logger.Info(resultMsg, zap.Int("total", len(keycloakUsers)), zap.Int("created", createdCount), zap.Int("updated", updatedCount))

	common.OkWithDetailed(gin.H{
		"total":        len(keycloakUsers),
		"createdCount": createdCount,
		"updatedCount": updatedCount,
	}, resultMsg, c)
}

func extractTokenRolesInfo(tokenStr string) string {
	parts := strings.Split(tokenStr, ".")
	if len(parts) < 2 {
		return "token格式无效"
	}
	payloadSegment := parts[1]
	if l := len(payloadSegment) % 4; l > 0 {
		payloadSegment += strings.Repeat("=", 4-l)
	}
	raw, err := base64.URLEncoding.DecodeString(payloadSegment)
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return fmt.Sprintf("解析payload失败: %v", err)
		}
	}

	var claims struct {
		RealmAccess struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
		ResourceAccess map[string]struct {
			Roles []string `json:"roles"`
		} `json:"resource_access"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return fmt.Sprintf("反序列化claims失败: %v", err)
	}

	var details []string
	if len(claims.RealmAccess.Roles) > 0 {
		details = append(details, fmt.Sprintf("realm_roles=%v", claims.RealmAccess.Roles))
	}
	for clientName, res := range claims.ResourceAccess {
		if len(res.Roles) > 0 {
			details = append(details, fmt.Sprintf("%s_roles=%v", clientName, res.Roles))
		}
	}
	if len(details) == 0 {
		return "无任何角色(可能未开启 Full scope allowed)"
	}
	return strings.Join(details, "; ")
}

// extractDisplayName 从 Keycloak 用户对象中提取最合适的中文显示名
// 优先级:
// 1. 用户自定义属性中的 zh_name (Keycloak User Profile 中配置的显示名属性)
// 2. 自定义属性中的 displayName / display_name / nickname
// 3. 中文姓+名 (如 庄 + 亨宝 = 庄亨宝)
// 4. FirstName 或 LastName
// 5. 兜底登录用户名 username
func extractDisplayName(ku KeycloakUserItem) string {
	// 1. 优先读取 Keycloak Attributes 自定义属性 (zh_name, displayName 等)
	if ku.Attributes != nil {
		attrKeys := []string{"zh_name", "displayName", "display_name", "nick_name", "nickname", "real_name"}
		for _, key := range attrKeys {
			if val, ok := ku.Attributes[key]; ok && val != nil {
				switch v := val.(type) {
				case []interface{}:
					if len(v) > 0 {
						if s := strings.TrimSpace(fmt.Sprintf("%v", v[0])); s != "" && s != "<nil>" {
							return s
						}
					}
				case []string:
					if len(v) > 0 {
						if s := strings.TrimSpace(v[0]); s != "" {
							return s
						}
					}
				case string:
					if s := strings.TrimSpace(v); s != "" {
						return s
					}
				}
			}
		}
	}

	// 2. 拼装姓与名
	first := strings.TrimSpace(ku.FirstName)
	last := strings.TrimSpace(ku.LastName)

	if last != "" && first != "" {
		isChinese := false
		for _, r := range last + first {
			if r > 127 {
				isChinese = true
				break
			}
		}
		if isChinese {
			// 中文姓名习惯：姓在前、名在后，不加空格（如 庄 + 亨宝 = 庄亨宝）
			return last + first
		}
		// 西方姓名习惯：First Last
		return first + " " + last
	}

	if first != "" {
		return first
	}
	if last != "" {
		return last
	}

	// 3. 兜底为登录名
	return strings.TrimSpace(ku.Username)
}
