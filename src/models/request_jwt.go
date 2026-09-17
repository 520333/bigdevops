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

// UserActiveTokens 全局维护：username -> 当前合法的最新 Token (用于单设备登录/顶号互斥控制)
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

// SetUserActiveToken 记录用户的最新活跃 Token
func SetUserActiveToken(username string, token string) {
	UserActiveTokens.Store(username, token)
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
	OnlineUserSessions.Delete(username)
	// 标记为已强退，阻断该用户旧 Token 继续通过单设备校验自愈
	UserActiveTokens.Store(username, "KICKED_OUT")
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

// IsLatestUserToken 校验当前 Token 是否为该用户最新活跃 Token
func IsLatestUserToken(username string, currentToken string) bool {
	val, ok := UserActiveTokens.Load(username)
	if !ok {
		// 服务刚重启或该用户尚无记录时，将当前有效 Token 作为最新 Token
		UserActiveTokens.Store(username, currentToken)
		return true
	}
	return val.(string) == currentToken
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

	userRsp := UserLoginResponse{
		SystemUser: dbUser,
		Token:      token,
	}
	common.OkWithDetailed(userRsp, "登录成功", c)
}

func GenJWTToken(dbUser *SystemUser, sc *config.ServerConfig) (string, error) {
	c := UserCustomClaims{
		SystemUser: dbUser,
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
