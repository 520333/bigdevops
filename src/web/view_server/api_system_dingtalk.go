package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"bigdevops/src/web/middleware"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

type DingTalkCallbackReq struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state"`
}

// DingTalkTokenResp 钉钉换取用户访问凭证响应 (OAuth2)
type DingTalkTokenResp struct {
	AccessToken  string `json:"accessToken"`
	ExpireIn     int    `json:"expireIn"`
	RefreshToken string `json:"refreshToken"`
	CorpId       string `json:"corpId"`
}

// DingTalkUserResp 钉钉获取个人信息响应 (OAuth2 me 接口)
type DingTalkUserResp struct {
	Nick      string `json:"nick"`
	AvatarUrl string `json:"avatarUrl"`
	Mobile    string `json:"mobile"`
	OpenId    string `json:"openId"`
	UnionId   string `json:"unionId"`
	Email     string `json:"email"`
}

// DingTalkGetTokenResp 企业内部应用获取 access_token
type DingTalkGetTokenResp struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// DingTalkGetByUnionIdResp 根据 unionId 获取企业内部 userId
type DingTalkGetByUnionIdResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	Result  struct {
		ContactType int    `json:"contact_type"`
		UserId      string `json:"userid"`
	} `json:"result"`
}

// DingTalkEnterpriseUserDetail 企业员工通讯录详情
type DingTalkEnterpriseUserDetail struct {
	UserId    string `json:"userid"`
	Name      string `json:"name"`
	Mobile    string `json:"mobile"`
	Email     string `json:"email"`
	OrgEmail  string `json:"org_email"`
	JobNumber string `json:"job_number"`
	Title     string `json:"title"`
	Avatar    string `json:"avatar"`
}

type DingTalkUserGetResp struct {
	ErrCode int                          `json:"errcode"`
	ErrMsg  string                       `json:"errmsg"`
	Result  DingTalkEnterpriseUserDetail `json:"result"`
}

// 缓存企业应用的 app_access_token
var (
	appTokenLock    sync.RWMutex
	cachedAppToken  string
	appTokenExpires time.Time
)

func getDingTalkAppAccessToken(client *resty.Client, appKey, appSecret string) (string, error) {
	appTokenLock.RLock()
	if cachedAppToken != "" && time.Now().Before(appTokenExpires) {
		token := cachedAppToken
		appTokenLock.RUnlock()
		return token, nil
	}
	appTokenLock.RUnlock()

	appTokenLock.Lock()
	defer appTokenLock.Unlock()

	if cachedAppToken != "" && time.Now().Before(appTokenExpires) {
		return cachedAppToken, nil
	}

	var res DingTalkGetTokenResp
	resp, err := client.R().
		SetQueryParam("appkey", appKey).
		SetQueryParam("appsecret", appSecret).
		SetResult(&res).
		Get("https://oapi.dingtalk.com/gettoken")

	if err != nil || !resp.IsSuccess() || res.ErrCode != 0 {
		return "", fmt.Errorf("获取企业应用凭证失败: %v, errmsg: %s", err, res.ErrMsg)
	}

	cachedAppToken = res.AccessToken
	expireSeconds := res.ExpiresIn - 600
	if expireSeconds <= 0 {
		expireSeconds = 3600
	}
	appTokenExpires = time.Now().Add(time.Duration(expireSeconds) * time.Second)

	return cachedAppToken, nil
}

func getDingTalkUserIdByUnionId(client *resty.Client, appToken, unionId string) (string, error) {
	var res DingTalkGetByUnionIdResp
	resp, err := client.R().
		SetQueryParam("access_token", appToken).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]string{"unionid": unionId}).
		SetResult(&res).
		Post("https://oapi.dingtalk.com/topapi/user/getbyunionid")

	if err != nil || !resp.IsSuccess() || res.ErrCode != 0 {
		return "", fmt.Errorf("通过 unionId 反查 userId 失败: %v, errmsg: %s", err, res.ErrMsg)
	}
	return res.Result.UserId, nil
}

func getDingTalkUserDetail(client *resty.Client, appToken, userId string) (*DingTalkEnterpriseUserDetail, error) {
	var res DingTalkUserGetResp
	resp, err := client.R().
		SetQueryParam("access_token", appToken).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{"userid": userId, "language": "zh_CN"}).
		SetResult(&res).
		Post("https://oapi.dingtalk.com/topapi/v2/user/get")

	if err != nil || !resp.IsSuccess() || res.ErrCode != 0 {
		return nil, fmt.Errorf("获取企业员工通讯录详情失败: %v, errmsg: %s", err, res.ErrMsg)
	}
	return &res.Result, nil
}

// @Summary      获取钉钉单点登录跳转URL
// @Description  获取钉钉 OAuth2 登录授权地址
// @Tags         auth-sso
// @Produce      json
// @Router       /auth/dingtalk/login [get]
func GetDingTalkLoginUrl(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	if sc.DingTalkSSOC == nil || !sc.DingTalkSSOC.Enable {
		common.ReqBadFailWithMessage("钉钉单点登录未开启", c)
		return
	}

	authUrl := fmt.Sprintf(
		"https://login.dingtalk.com/oauth2/auth?client_id=%s&response_type=code&scope=openid&redirect_uri=%s&state=dingtalk_sso&prompt=consent",
		sc.DingTalkSSOC.ClientID,
		url.QueryEscape(sc.DingTalkSSOC.RedirectURI),
	)

	common.OkWithData(gin.H{"url": authUrl}, c)
}

// @Summary      钉钉单点登录回调认证
// @Description  接收 authCode 并换取用户信息完成免密登录
// @Tags         auth-sso
// @Accept       json
// @Produce      json
// @Router       /auth/dingtalk/callback [post]
func DingTalkCallback(c *gin.Context) {
	var req DingTalkCallbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage("参数错误：未提供授权临时码 code", c)
		return
	}

	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	if sc.DingTalkSSOC == nil || !sc.DingTalkSSOC.Enable {
		common.ReqBadFailWithMessage("钉钉单点登录未开启", c)
		return
	}

	client := resty.New().SetTimeout(8 * time.Second)

	// 1. 调用钉钉开放平台接口通过 code 换取 userAccessToken
	var tokenResp DingTalkTokenResp
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{
			"clientId":     sc.DingTalkSSOC.ClientID,
			"clientSecret": sc.DingTalkSSOC.ClientSecret,
			"code":         req.Code,
			"grantType":    "authorization_code",
		}).
		SetResult(&tokenResp).
		Post("https://api.dingtalk.com/v1.0/oauth2/userAccessToken")

	if err != nil || !resp.IsSuccess() || tokenResp.AccessToken == "" {
		sc.Logger.Error("钉钉换取 userAccessToken 失败",
			zap.Error(err),
			zap.Int("http_code", resp.StatusCode()),
			zap.String("resp", resp.String()),
		)
		common.ReqBadFailWithMessage(fmt.Sprintf("换取钉钉用户凭证失败: %s", resp.String()), c)
		return
	}

	// 2. 使用 userAccessToken 获取员工个人信息 (包含 unionId)
	var userInfo DingTalkUserResp
	infoResp, err := client.R().
		SetHeader("x-acs-dingtalk-access-token", tokenResp.AccessToken).
		SetResult(&userInfo).
		Get("https://api.dingtalk.com/v1.0/contact/users/me")

	if err != nil || !infoResp.IsSuccess() {
		sc.Logger.Error("获取钉钉用户信息失败",
			zap.Error(err),
			zap.Int("http_code", infoResp.StatusCode()),
			zap.String("resp", infoResp.String()),
		)
		common.ReqBadFailWithMessage(fmt.Sprintf("获取钉钉用户信息失败: %s", infoResp.String()), c)
		return
	}

	// 3. 【核心增强】通过 unionId 自动反查该员工在企业内的真实 UserId 与完整通讯录信息
	enterpriseUserId := ""
	var enterpriseDetail DingTalkEnterpriseUserDetail

	appToken, tokenErr := getDingTalkAppAccessToken(client, sc.DingTalkSSOC.ClientID, sc.DingTalkSSOC.ClientSecret)
	if tokenErr == nil && appToken != "" && userInfo.UnionId != "" {
		userId, idErr := getDingTalkUserIdByUnionId(client, appToken, userInfo.UnionId)
		if idErr == nil && userId != "" {
			enterpriseUserId = userId
			sc.Logger.Info("成功通过 unionId 反查到员工企业内部 UserId",
				zap.String("unionId", userInfo.UnionId),
				zap.String("userId", enterpriseUserId),
			)
			// 进一步获取通讯录详细信息 (包含手机号、工号、企业邮箱)
			detail, detailErr := getDingTalkUserDetail(client, appToken, userId)
			if detailErr == nil && detail != nil {
				enterpriseDetail = *detail
			}
		} else {
			sc.Logger.Warn("通过 unionId 反查 userId 失败，将采用兜底账号生成策略", zap.Error(idErr))
		}
	}

	// 4. 账号与信息确定 (用户名优先采用企业 UserId / 真实工号)
	targetUsername := ""
	if enterpriseUserId != "" {
		targetUsername = enterpriseUserId
	} else if enterpriseDetail.JobNumber != "" {
		targetUsername = enterpriseDetail.JobNumber
	} else if enterpriseDetail.Mobile != "" {
		targetUsername = enterpriseDetail.Mobile
	} else if userInfo.Mobile != "" {
		targetUsername = userInfo.Mobile
	} else if userInfo.UnionId != "" {
		if len(userInfo.UnionId) > 8 {
			targetUsername = "dd_" + userInfo.UnionId[:8]
		} else {
			targetUsername = "dd_" + userInfo.UnionId
		}
	} else {
		targetUsername = userInfo.Nick
	}

	// 补齐姓名、手机号、企业邮箱、头像
	realName := userInfo.Nick
	if enterpriseDetail.Name != "" {
		realName = enterpriseDetail.Name
	}
	mobile := userInfo.Mobile
	if enterpriseDetail.Mobile != "" {
		mobile = enterpriseDetail.Mobile
	}
	email := userInfo.Email
	if enterpriseDetail.OrgEmail != "" {
		email = enterpriseDetail.OrgEmail
	} else if enterpriseDetail.Email != "" {
		email = enterpriseDetail.Email
	}
	avatar := userInfo.AvatarUrl
	if enterpriseDetail.Avatar != "" {
		avatar = enterpriseDetail.Avatar
	}

	// 5. 账号查找与匹配 (专属钉钉号匹配，彻底与 Keycloak 隔离，互不干扰)
	var dbUser *models.SystemUser

	// 5.1 顶级优先：通过专属钉钉号 (DingTalkUserId / DingTalkUnionId) 精准查找已绑定用户
	if enterpriseUserId != "" || userInfo.UnionId != "" {
		var u models.SystemUser
		query := models.Db
		if enterpriseUserId != "" && userInfo.UnionId != "" {
			query = query.Where("ding_talk_user_id = ? OR ding_talk_union_id = ?", enterpriseUserId, userInfo.UnionId)
		} else if enterpriseUserId != "" {
			query = query.Where("ding_talk_user_id = ?", enterpriseUserId)
		} else {
			query = query.Where("ding_talk_union_id = ?", userInfo.UnionId)
		}
		if query.First(&u).Error == nil {
			dbUser = &u
		}
	}

	// 5.2 次级匹配：若尚未绑定钉钉字段，通过真实手机号关联老员工账号完成绑定
	if dbUser == nil && mobile != "" {
		var u models.SystemUser
		if models.Db.Where("mobile = ?", mobile).First(&u).Error == nil {
			dbUser = &u
		}
	}

	// 5.3 智能关联：若本地已有未绑定钉钉的同名用户 (如 Keycloak 创建的 dawn)，自动认领绑定，绝不新建 manager4123 账号
	if dbUser == nil && realName != "" {
		var matchedUsers []models.SystemUser
		if models.Db.Where("real_name = ? AND (ding_talk_user_id IS NULL OR ding_talk_user_id = '')", realName).Find(&matchedUsers).Error == nil {
			if len(matchedUsers) >= 1 {
				// 优先选取非临时命名的主账号 (排除以纯中文或 manager/dd_ 命名的老记录)
				selectedIndex := 0
				for i, u := range matchedUsers {
					if u.Username != realName && !strings.HasPrefix(u.Username, "manager") && !strings.HasPrefix(u.Username, "dd_") {
						selectedIndex = i
						break
					}
				}
				dbUser = &matchedUsers[selectedIndex]
				sc.Logger.Info("钉钉登录成功关联到已有本地主账号，将写入钉钉号",
					zap.String("username", dbUser.Username),
					zap.String("real_name", realName),
					zap.String("dingTalkUserId", enterpriseUserId),
				)
			}
		}
	}

	// 5.4 若依然不存在，自动为新员工创建专属账号并写入钉钉号
	if dbUser == nil {
		newUser := &models.SystemUser{
			Username:        targetUsername,
			RealName:        realName,
			Email:           email,
			Mobile:          mobile,
			Avatar:          avatar,
			DingTalkUserId:  enterpriseUserId,
			DingTalkUnionId: userInfo.UnionId,
			Enable:          1, // 正常启用
			HomePath:        "/dashboard/analysis",
			Password:        common.BcryptHash("123456"),
		}
		if err := newUser.CreateOne(); err != nil {
			sc.Logger.Error("钉钉SSO自动创建用户失败", zap.Error(err))
			common.ReqBadFailWithMessage(fmt.Sprintf("钉钉SSO自动开户失败: %v", err), c)
			return
		}
		dbUser, _ = models.GetUserByUsername(targetUsername)

		// 默认分配普通用户角色 (user)
		var rolesToAssign []*models.SystemRole
		if defaultRole, err := models.GetRoleByRoleValue("user"); err == nil {
			rolesToAssign = append(rolesToAssign, defaultRole)
		} else if allRoles, err := models.GetRoleAll(); err == nil && len(allRoles) > 0 {
			rolesToAssign = append(rolesToAssign, allRoles[0])
		}
		if len(rolesToAssign) > 0 && dbUser != nil {
			_ = dbUser.UpdateOne(rolesToAssign)
			dbUser, _ = models.GetUserByUsername(targetUsername)
		}

		// 记录自动开户审计日志
		middleware.RecordAuditLogManual(
			c,
			dbUser.ID,
			dbUser.Username,
			dbUser.RealName,
			"用户管理",
			"钉钉SSO自动开户",
			c.Request.Method,
			c.Request.URL.Path,
			200,
			0,
			fmt.Sprintf(`{"username":"%s","real_name":"%s","mobile":"%s","ding_talk_user_id":"%s","source":"dingtalk_sso"}`, dbUser.Username, dbUser.RealName, dbUser.Mobile, dbUser.DingTalkUserId),
		)
	} else {
		// 已有账号：自动回填缺失的钉钉号字段，并同步最新的姓名/手机号/邮箱/头像
		patchFields := map[string]interface{}{}
		if dbUser.DingTalkUserId == "" && enterpriseUserId != "" {
			patchFields["ding_talk_user_id"] = enterpriseUserId
		}
		if dbUser.DingTalkUnionId == "" && userInfo.UnionId != "" {
			patchFields["ding_talk_union_id"] = userInfo.UnionId
		}
		if dbUser.RealName == "" && realName != "" {
			patchFields["real_name"] = realName
		}
		if dbUser.Mobile == "" && mobile != "" {
			patchFields["mobile"] = mobile
		}
		if dbUser.Email == "" && email != "" {
			patchFields["email"] = email
		}
		if dbUser.Avatar == "" && avatar != "" {
			patchFields["avatar"] = avatar
		}
		if len(patchFields) > 0 {
			_ = models.Db.Model(dbUser).Updates(patchFields)
		}
	}

	// 6. 重新拉取完整用户信息 (预加载关联的 Roles 与 Menus，彻底解决角色显示为“无”的问题)
	if fullUser, err := models.GetUserByUsername(dbUser.Username); err == nil && fullUser != nil {
		dbUser = fullUser
	}

	// 6.1 若该用户尚未分配任何角色，赋予默认普通用户角色 (user)
	if len(dbUser.Roles) == 0 {
		if defaultRole, err := models.GetRoleByRoleValue("user"); err == nil {
			_ = dbUser.UpdateOne([]*models.SystemRole{defaultRole})
			if fullUser, err := models.GetUserByUsername(dbUser.Username); err == nil && fullUser != nil {
				dbUser = fullUser
			}
		}
	}

	// 7. 记录单点登录审计日志
	middleware.RecordAuditLogManual(
		c,
		dbUser.ID,
		dbUser.Username,
		dbUser.RealName,
		"用户认证",
		"钉钉SSO登录成功",
		c.Request.Method,
		c.Request.URL.Path,
		200,
		0,
		fmt.Sprintf(`{"auth_type":"dingtalk_sso","username":"%s","real_name":"%s"}`, dbUser.Username, dbUser.RealName),
	)

	// 8. 颁发平台原生 JWT 并注册在线会话返回前端
	models.TokenNext(dbUser, c)
}
