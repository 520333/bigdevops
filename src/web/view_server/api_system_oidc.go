package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"bigdevops/src/web/middleware"
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

type OidcCallbackReq struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state"`
}

// GetOidcLoginUrl GET /api/auth/oidc/login - 获取 Keycloak 登录跳转地址
func GetOidcLoginUrl(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	if sc.OIDC == nil || !sc.OIDC.Enable {
		common.ReqBadFailWithMessage("OIDC 登录未开启", c)
		return
	}
	provider, err := oidc.NewProvider(context.Background(), sc.OIDC.Issuer)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("初始化 OIDC Provider 失败: %v", err), c)
		return
	}
	oauth2Config := oauth2.Config{
		ClientID:     sc.OIDC.ClientID,
		ClientSecret: sc.OIDC.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  sc.OIDC.RedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	// 生产环境 state 建议使用 random string 存入 redis/session 校验防 CSRF
	authUrl := oauth2Config.AuthCodeURL("random-state-string")
	common.OkWithData(gin.H{"url": authUrl}, c)
}

// OidcCallback POST /api/auth/oidc/callback - 接收 Authorization Code 并换取 Token & 完成平台登录
func OidcCallback(c *gin.Context) {
	var req OidcCallbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.FailWithMessage("参数错误", c)
		return
	}
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, sc.OIDC.Issuer)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("OIDC Provider 错误: %v", err), c)
		return
	}
	oauth2Config := oauth2.Config{
		ClientID:     sc.OIDC.ClientID,
		ClientSecret: sc.OIDC.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  sc.OIDC.RedirectURL,
	}
	// 1. 使用 code 向 Keycloak 换 Token
	oauth2Token, err := oauth2Config.Exchange(ctx, req.Code)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("交换 Token 失败: %v", err), c)
		return
	}
	// 2. 提取并校验 ID Token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		common.ReqBadFailWithMessage("返回数据中未包含 id_token", c)
		return
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: sc.OIDC.ClientID})
	idToken, err := verifier.Verify(context.Background(), rawIDToken)
	if err != nil {
		models.RecordLoginLog(c, "", "", "OIDC单点", 0, fmt.Sprintf("校验 ID Token 失败: %v", err))
		common.ReqBadFailWithMessage(fmt.Sprintf("校验 ID Token 失败: %v", err), c)
		return
	}
	// 3. 解析用户 Claim 信息 (包含 Keycloak 传过来的 groups 字段)
	var claims struct {
		PreferredUsername string   `json:"preferred_username"`
		Email             string   `json:"email"`
		Name              string   `json:"name"`
		Groups            []string `json:"groups"` // Keycloak 分配的组/角色
		RealmAccess       struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}
	if err := idToken.Claims(&claims); err != nil {
		common.ReqBadFailWithMessage("解析 User Claims 失败", c)
		return
	}
	username := claims.PreferredUsername
	if username == "" {
		username = claims.Email
	}
	// 4. 自动开户/同步本地用户表 tbl_system_user
	dbUser, err := models.GetUserByUsername(username)
	if err != nil {
		// 4.1 检查是否该员工此前曾通过钉钉直接登录过 (避免同一个人先登钉钉再登Keycloak产生两个号)
		var candidateUser models.SystemUser
		matched := false

		// ① 优先通过企业真实邮箱匹配已有账号
		if claims.Email != "" && !strings.Contains(claims.Email, "@dingtalk.local") {
			if models.Db.Where("email = ?", claims.Email).First(&candidateUser).Error == nil {
				matched = true
			}
		}

		// ② 其次：若本地存在同名且带有钉钉绑定的临时账号 (如 manager4123)，自动认领平滑升级为正式 Keycloak 域账号
		if !matched && claims.Name != "" {
			if models.Db.Where("real_name = ? AND (ding_talk_user_id IS NOT NULL AND ding_talk_user_id != '')", claims.Name).First(&candidateUser).Error == nil {
				matched = true
			}
		}

		if matched {
			// 将此前由钉钉创建的账号，无感平滑升级为正式域用户名 (如将 manager4123 更新为正式的 dawn)
			updateData := map[string]interface{}{
				"username": username,
			}
			if claims.Email != "" && !strings.Contains(claims.Email, "@dingtalk.local") {
				updateData["email"] = claims.Email
			}
			if claims.Name != "" {
				updateData["real_name"] = claims.Name
			}
			_ = models.Db.Model(&candidateUser).Updates(updateData)
			dbUser, _ = models.GetUserByUsername(username)
			fmt.Printf("[OIDC SSO] 已成功将已有钉钉绑定账号平滑认领升级为正式 Keycloak 域账号: old=%s, new=%s, name=%s\n", candidateUser.Username, username, candidateUser.RealName)
		} else {
			// 4.2 真正的新员工：自动创建本地账号
			newUser := &models.SystemUser{
				Username: username,
				RealName: claims.Name,
				Email:    claims.Email,
				Enable:   1, // 正常启用
				HomePath: "/dashboard/analysis",
				Password: common.BcryptHash("123456"),
			}
			// 插入数据库
			if err := newUser.CreateOne(); err != nil {
				common.ReqBadFailWithMessage(fmt.Sprintf("自动创建本地用户失败: %v", err), c)
				return
			}
			dbUser, _ = models.GetUserByUsername(username)

			// 记录 SSO 首次自动开户审计日志
			middleware.RecordAuditLogManual(
				c,
				dbUser.ID,
				dbUser.Username,
				dbUser.RealName,
				"用户管理",
				"SSO自动开户",
				c.Request.Method,
				c.Request.URL.Path,
				200,
				0,
				fmt.Sprintf(`{"username":"%s","real_name":"%s","email":"%s","source":"oidc_sso"}`, dbUser.Username, dbUser.RealName, dbUser.Email),
			)
		}
	}

	// 5. 解析 Keycloak 组与角色并映射同步至数据库 user_roles 中间表
	var rolesToAssign []*models.SystemRole

	// 5.1 优先使用 Keycloak claims.Groups 匹配本地 Role
	for _, groupName := range claims.Groups {
		cleanGroup := strings.TrimPrefix(groupName, "/") // 移除路径斜杠如 "/user" -> "user"
		if r, err := models.GetRoleByRoleValue(cleanGroup); err == nil {
			rolesToAssign = append(rolesToAssign, r)
		} else if r, err := models.GetRoleByRoleValue(strings.ToLower(cleanGroup)); err == nil {
			rolesToAssign = append(rolesToAssign, r)
		}
	}

	// 5.2 若 Groups 未匹配到，尝试匹配 Keycloak Realm Roles
	if len(rolesToAssign) == 0 {
		for _, roleVal := range claims.RealmAccess.Roles {
			if r, err := models.GetRoleByRoleValue(roleVal); err == nil {
				rolesToAssign = append(rolesToAssign, r)
			}
		}
	}

	// 5.3 若 Keycloak 未匹配到任何角色，且本地用户尚未分配任何角色（新用户），自动赋予默认“普通用户”组 (role_value: "user")
	if len(rolesToAssign) == 0 && len(dbUser.Roles) == 0 {
		defaultRoleValue := "user" // 默认给 role_value: "user" (普通用户)
		if defaultRole, err := models.GetRoleByRoleValue(defaultRoleValue); err == nil {
			rolesToAssign = append(rolesToAssign, defaultRole)
		} else {
			// 若找不到 "user" 角色，兜底使用系统第一条角色
			if allRoles, err := models.GetRoleAll(); err == nil && len(allRoles) > 0 {
				rolesToAssign = append(rolesToAssign, allRoles[0])
			}
		}
	}

	// 只有当成功从 Keycloak 匹配到新角色或为新用户初始化默认角色时，才同步更新数据库中的角色映射
	if len(rolesToAssign) > 0 {
		_ = dbUser.UpdateOne(rolesToAssign)
		// 重新拉取最新的 dbUser (包含关联的 Roles 数据)
		if updatedUser, err := models.GetUserByUsername(username); err == nil {
			dbUser = updatedUser
		}
	}

	// 6. 记录 SSO 单点登录成功审计日志
	middleware.RecordAuditLogManual(
		c,
		dbUser.ID,
		dbUser.Username,
		dbUser.RealName,
		"用户认证",
		"SSO登录成功",
		c.Request.Method,
		c.Request.URL.Path,
		200,
		0,
		fmt.Sprintf(`{"auth_type":"oidc_sso","username":"%s","email":"%s"}`, dbUser.Username, dbUser.Email),
	)

	// 7. 调用平台已有的 TokenNext 生成平台原有 JWT 并返回前端
	models.TokenNext(dbUser, c)
}
