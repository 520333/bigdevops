package cron

import (
	"bigdevops/src/common"
	"context"
	"encoding/json"

	"go.uber.org/zap"
)

type RobotTenantAccessTokenReq struct {
	AppId     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}
type RobotTenantAccessTokenResp struct {
	Code              int    `json:"code"`
	Expire            int    `json:"expire"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
}

func (ac *AlertCache) RefreshPrivateChatToken(ctx context.Context) {
	data := RobotTenantAccessTokenReq{
		AppId:     ac.Sc.ImC.FeiShu.AppID,
		AppSecret: ac.Sc.ImC.FeiShu.AppSecret,
	}
	jsonStr, _ := json.Marshal(data)
	bodyBytes, err := common.PostWithJsonString(ac.Sc.Logger, "RefreshPrivateChatToken",
		ac.Sc.ImC.FeiShu.RequestTimeoutSeconds, "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		string(jsonStr), nil, nil)
	if err != nil {
		return
	}
	var res *RobotTenantAccessTokenResp
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		ac.Sc.Logger.Error("解析json失败", zap.Error(err), zap.Any("appid", ac.Sc.ImC.FeiShu.AppID))
		return
	}
	ac.RobotTokenLock.Lock()
	ac.RobotToken = res.TenantAccessToken
	ac.RobotTokenLock.Unlock()
}
func (ac *AlertCache) GetPrivateChatToken() string {
	ac.RobotTokenLock.RLock()
	defer ac.RobotTokenLock.RUnlock()
	return ac.RobotToken
}
