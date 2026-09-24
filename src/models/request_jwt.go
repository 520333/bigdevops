package models

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UserTokenState 用户设备Token状态管理 (支持单设备互斥、临期续签平滑过渡与管理员强退)
type UserTokenState struct {
	LatestToken            string    `json:"latestToken"`            // 当前最新签发的合法活跃 Token
	PreviousToken          string    `json:"previousToken"`          // 上一版合法 Token（刚被续期替换下来的旧 Token）
	PreviousTokenExpiresAt time.Time `json:"previousTokenExpiresAt"` // 上一版旧 Token 的过渡宽限截止时间（默认60秒）
	KickedOut              bool      `json:"kickedOut"`              // 是否被管理员手动强制下线
	LastRenewTime          time.Time `json:"lastRenewTime"`          // 上次自动续签生成时间（用于并发请求防抖冷却）
}

// UserActiveTokens 全局维护：username -> *UserTokenState
var UserActiveTokens sync.Map

// OnlineSession 在线用户会话结构
type OnlineSession struct {
	ID             uint      `json:"id"`
	Username       string    `json:"userName"`
	RealName       string    `json:"realName"`
	Roles          []string  `json:"roles"`
	LoginTime      time.Time `json:"loginTime"`
	LastActiveTime time.Time `json:"lastActiveTime"`
	IP             string    `json:"ip"`
	Browser        string    `json:"browser"`
	OS             string    `json:"os"`
	Token          string    `json:"token"`
}

// OnlineUserSessions 全局维护：username -> OnlineSession
var OnlineUserSessions sync.Map

// SetUserActiveToken 用户主动登录/重新登录时记录最新活跃 Token，清空旧 Token 宽限期（确保单设备互斥安全）
func SetUserActiveToken(username string, token string) {
	UserActiveTokens.Store(username, &UserTokenState{
		LatestToken:   token,
		PreviousToken: "",
		KickedOut:     false,
	})
}

// RenewUserToken 用户临期自动续签：支持防并发高频重复签发冷却与旧 Token 60秒过渡宽限期
// 返回 (最新Token, 是否全新签发, 错误)
func RenewUserToken(user *SystemUser, oldToken string, sc *config.ServerConfig) (string, bool, error) {
	now := time.Now()
	username := user.Username

	val, ok := UserActiveTokens.Load(username)
	if ok && val != nil {
		if state, ok2 := val.(*UserTokenState); ok2 {
			// 1. 如果已被管理员强制踢下线，拒绝续签
			if state.KickedOut {
				return "", false, errors.New("账号已被强制下线")
			}
			// 2. 防并发重复签发冷却：若距离上次续签不足 30 秒，直接复用已签发的最新 Token
			if !state.LastRenewTime.IsZero() && now.Sub(state.LastRenewTime) < 30*time.Second && state.LatestToken != "" {
				return state.LatestToken, false, nil
			}
		}
	}

	// 3. 生成新 Token
	newToken, err := GenJWTToken(user, sc)
	if err != nil {
		return "", false, err
	}

	// 4. 设定旧 Token 宽限期为 60 秒（充分覆盖并发请求与前端异步存储延迟）
	gracePeriod := 60 * time.Second
	UserActiveTokens.Store(username, &UserTokenState{
		LatestToken:            newToken,
		PreviousToken:          oldToken,
		PreviousTokenExpiresAt: now.Add(gracePeriod),
		KickedOut:              false,
		LastRenewTime:          now,
	})

	return newToken, true, nil
}

// CheckUserTokenStatus 综合校验当前 Token 状态
// 返回: (valid: 是否合法通过, isKickedOut: 是否是被管理员强退, latestToken: 当前系统最新有效Token)
func CheckUserTokenStatus(username string, currentToken string) (bool, bool, string) {
	val, ok := UserActiveTokens.Load(username)
	if !ok || val == nil {
		// 服务刚重启或该用户尚无记录时，自动自愈当前有效 Token 为最新 Token
		UserActiveTokens.Store(username, &UserTokenState{
			LatestToken: currentToken,
		})
		return true, false, currentToken
	}

	state, ok2 := val.(*UserTokenState)
	if !ok2 {
		// 兼容可能遗留的纯字符串格式
		if strVal, isStr := val.(string); isStr {
			if strVal == "KICKED_OUT" {
				return false, true, ""
			}
			if strVal == currentToken {
				return true, false, currentToken
			}
		}
		return false, false, ""
	}

	// 1. 管理员主动踢出
	if state.KickedOut {
		return false, true, ""
	}

	// 2. 当前请求携带的是最新 Token
	if state.LatestToken == currentToken {
		return true, false, state.LatestToken
	}

	// 3. 当前请求携带的是刚被续签替换的上一版旧 Token，且处于 60 秒宽限期内 (并发请求平滑过渡)
	if state.PreviousToken == currentToken && time.Now().Before(state.PreviousTokenExpiresAt) {
		return true, false, state.LatestToken
	}

	// 4. 既非最新 Token 也过了宽限期，判定为被新设备登录顶号
	return false, false, state.LatestToken
}

// RegisterOnlineSession 注册/更新在线用户会话
func RegisterOnlineSession(dbUser *SystemUser, token, ip, ua string) {
	var roles []string
	for _, r := range dbUser.Roles {
		roles = append(roles, r.RoleName)
	}
	now := time.Now()
	ip = common.NormalizeIP(ip)
	browser, os := common.ParseUserAgent(ua)
	session := OnlineSession{
		ID:             dbUser.ID,
		Username:       dbUser.Username,
		RealName:       dbUser.RealName,
		Roles:          roles,
		LoginTime:      now,
		LastActiveTime: now,
		IP:             ip,
		Browser:        browser,
		OS:             os,
		Token:          token,
	}
	OnlineUserSessions.Store(dbUser.Username, session)
	SetUserActiveToken(dbUser.Username, token)
}

// EnsureOnlineSession 确保在线用户会话存在：自动自愈注册服务重启前已登录的用户，并实时刷新活跃时间、IP与客户端环境
func EnsureOnlineSession(claims *UserCustomClaims, token, ip, ua string) {
	if claims == nil || claims.Username == "" {
		return
	}
	ip = common.NormalizeIP(ip)
	browser, os := common.ParseUserAgent(ua)
	if val, ok := OnlineUserSessions.Load(claims.Username); ok {
		if session, ok2 := val.(OnlineSession); ok2 {
			session.LastActiveTime = time.Now()
			if ip != "" {
				session.IP = ip
			}
			if token != "" {
				session.Token = token
			}
			if browser != "" && browser != "未知浏览器" {
				session.Browser = browser
			}
			if os != "" && os != "未知系统" {
				session.OS = os
			}
			OnlineUserSessions.Store(claims.Username, session)
			return
		}
	}

	// 内存中尚无该用户（如服务重启后携带旧有效Token首次访问），自动自愈补齐会话
	var roles []string
	userID := uint(0)
	realName := ""
	if claims.SystemUser != nil {
		userID = claims.SystemUser.ID
		realName = claims.SystemUser.RealName
		for _, r := range claims.SystemUser.Roles {
			roles = append(roles, r.RoleName)
		}
	}
	now := time.Now()
	loginTime := now
	if claims.IssuedAt != nil {
		loginTime = claims.IssuedAt.Time
	}
	session := OnlineSession{
		ID:             userID,
		Username:       claims.Username,
		RealName:       realName,
		Roles:          roles,
		LoginTime:      loginTime,
		LastActiveTime: now,
		IP:             ip,
		Browser:        browser,
		OS:             os,
		Token:          token,
	}
	OnlineUserSessions.Store(claims.Username, session)
	SetUserActiveToken(claims.Username, token)
}

// UpdateSessionActiveTime 刷新用户最近活跃时间与当前IP
func UpdateSessionActiveTime(username string, ip ...string) {
	if val, ok := OnlineUserSessions.Load(username); ok {
		if session, ok2 := val.(OnlineSession); ok2 {
			session.LastActiveTime = time.Now()
			if len(ip) > 0 && ip[0] != "" {
				session.IP = common.NormalizeIP(ip[0])
			} else {
				session.IP = common.NormalizeIP(session.IP)
			}
			OnlineUserSessions.Store(username, session)
		}
	}
}

// RemoveOnlineSession 移除在线会话（主动登出）
func RemoveOnlineSession(username string) {
	OnlineUserSessions.Delete(username)
	ClearUserActiveToken(username)
}

// KickoutOnlineUser 管理员强制将用户踢下线
func KickoutOnlineUser(username string) {
	if val, ok := OnlineUserSessions.Load(username); ok {
		if session, ok2 := val.(OnlineSession); ok2 {
			RecordLoginLogDirect(session.Username, session.RealName, session.IP, session.OS, session.Browser, "强退下线", 1, "管理员强制下线")
		}
	}
	OnlineUserSessions.Delete(username)
	// 标记为已强退，阻断该用户旧 Token 继续通过单设备校验自愈
	UserActiveTokens.Store(username, &UserTokenState{
		KickedOut: true,
	})
}

// GetOnlineSessionList 获取所有当前有效的在线用户会话
func GetOnlineSessionList(sc *config.ServerConfig) []OnlineSession {
	var list []OnlineSession
	OnlineUserSessions.Range(func(key, value interface{}) bool {
		if session, ok := value.(OnlineSession); ok {
			// 校验对应 Token 是否依然合法有效且未过期
			if _, err := ParseToken(session.Token, sc); err == nil {
				session.IP = common.NormalizeIP(session.IP)
				list = append(list, session)
			} else {
				// Token 已过期失效，自动清理
				OnlineUserSessions.Delete(key)
				UserActiveTokens.Delete(key)
			}
		}
		return true
	})
	return list
}

// IsLatestUserToken 校验当前 Token 是否为该用户最新活跃 Token (兼容旧方法名)
func IsLatestUserToken(username string, currentToken string) bool {
	valid, _, _ := CheckUserTokenStatus(username, currentToken)
	return valid
}

// ClearUserActiveToken 用户退出登录时清理
func ClearUserActiveToken(username string) {
	UserActiveTokens.Delete(username)
}

func TokenNext(dbUser *SystemUser, c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	token, err := GenJWTToken(dbUser, sc)
	if err != nil {
		sc.Logger.Error("生成token失败", zap.Error(err))
		common.FailWithMessage("生成token失败", c)
		return
	}

	// 提取客户端真实 IP 并注册在线会话
	clientIP := common.GetRealClientIP(c)
	RegisterOnlineSession(dbUser, token, clientIP, c.Request.UserAgent())

	// 记录登录审计日志
	loginType := "密码登录"
	if c.Request != nil && c.Request.URL != nil {
		pathLower := strings.ToLower(c.Request.URL.Path)
		if strings.Contains(pathLower, "oidc") {
			loginType = "OIDC单点"
		} else if strings.Contains(pathLower, "dingtalk") {
			loginType = "钉钉扫码"
		}
	}
	RecordLoginLog(c, dbUser.Username, dbUser.RealName, loginType, 1, "登录成功")

	userRsp := UserLoginResponse{
		SystemUser: dbUser,
		Token:      token,
	}
	common.OkWithDetailed(userRsp, "登录成功", c)
}

func GenJWTToken(dbUser *SystemUser, sc *config.ServerConfig) (string, error) {
	if dbUser == nil {
		return "", errors.New("user is nil")
	}

	// 浅拷贝并瘦身：严禁将角色关联的全量菜单树 (Menus) 和其他重型关联塞入 JWT Payload，
	// 避免 JWT Token 膨胀至数十 KB 导致击穿 Nginx / 网关 (HTTP/1 与 HTTP/2) 缓冲区引发网络错误 (ERR_CONNECTION_RESET)
	cleanRoles := make([]*SystemRole, len(dbUser.Roles))
	for i, r := range dbUser.Roles {
		if r != nil {
			cleanRoles[i] = &SystemRole{
				Model:     r.Model,
				RoleName:  r.RoleName,
				RoleValue: r.RoleValue,
			}
		}
	}

	cleanUser := *dbUser
	cleanUser.Roles = cleanRoles
	cleanUser.OpsNodes = nil
	cleanUser.StaticReceiveUsers = nil
	cleanUser.FirstUpgradeUsers = nil
	cleanUser.MonitorOnDutyGroup = nil
	cleanUser.RolesFront = nil

	c := UserCustomClaims{
		SystemUser: &cleanUser,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(), // 唯一JWT标识(JTI)，确保每次签发的Token完全独立唯一，防止同一秒内/高频登录生成相同Token
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    sc.JWTC.Issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(sc.JWTC.ExpiresDuration)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString([]byte(sc.JWTC.SigningKey))
}

func ParseToken(jwtLongToken string, sc *config.ServerConfig) (*UserCustomClaims, error) {
	tokenClaims, err := jwt.ParseWithClaims(
		jwtLongToken,
		&UserCustomClaims{},
		func(token *jwt.Token) (i interface{}, e error) {
			return []byte(sc.JWTC.SigningKey), nil
		},
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) || strings.Contains(err.Error(), "token is expired") {
			sc.Logger.Warn("用户token已过期", zap.Error(err))
		} else {
			sc.Logger.Error("根据长tokenString解析错误", zap.Error(err))
		}
		return nil, err
	}
	if claims, ok := tokenClaims.Claims.(*UserCustomClaims); ok && tokenClaims.Valid {
		return claims, nil
	}
	return nil, err
}
