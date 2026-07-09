package view_alertwebhook

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AMSilence 定义发送给 Alertmanager API 的 Silence 结构体
type AMSilence struct {
	Matchers  []AMMatcher `json:"matchers"`
	StartsAt  time.Time   `json:"startsAt"`
	EndsAt    time.Time   `json:"endsAt"`
	CreatedBy string      `json:"createdBy"`
	Comment   string      `json:"comment"`
}

type AMMatcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
}
type SilenceResponse struct {
	SilenceID string `json:"silenceID"`
}

func AlertSilence(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.AlertWebhookConfig)
	fingerprint := c.DefaultQuery("fingerprint", "")
	byName := c.DefaultQuery("by_name", "")
	byNameBool := false
	if byName == "1" {
		byNameBool = true
	}

	hour := c.DefaultQuery("hour", "")

	hourInt, err := strconv.Atoi(hour)
	if err != nil || hourInt <= 0 {
		c.String(http.StatusBadRequest, "无效的时间参数")
		return
	}

	event, err := models.GetMonitorAlertEventByFingerPrintId(fingerprint)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("通过fingerprint去查询event错误 %s", err.Error()))
		return
	}

	// 1. 构建 Alertmanager v2 API 所需的 Matchers
	event.GenMapFromKvs()
	amMatchers := make([]AMMatcher, 0) // 必须初始化，防止 json 变成 null
	if byNameBool {
		// 如果是按名称屏蔽，只添加名称这一个匹配器
		if name, ok := event.LabelsM[common.MONITOR_ALERT_NAME_KEY]; ok {
			amMatchers = append(amMatchers, AMMatcher{
				Name:    common.MONITOR_ALERT_NAME_KEY,
				Value:   name,
				IsRegex: false,
				IsEqual: true,
			})
		}
	} else {
		// 否则添加所有标签
		for k, v := range event.LabelsM {
			amMatchers = append(amMatchers, AMMatcher{
				Name:    k,
				Value:   v,
				IsRegex: false,
				IsEqual: true,
			})
		}
	}

	// 致命防御：如果没有匹配到任何标签，直接拒绝请求
	if len(amMatchers) == 0 {
		sc.Logger.Error("无法创建静默: 数据库中该 event 没有 Labels 数据", zap.String("fingerprint", fingerprint))
		c.String(http.StatusBadRequest, "静默失败：找不到该告警的标签特征，无法屏蔽")
		return
	}

	// 2. 实例化 Silence Payload
	now := time.Now()
	si := AMSilence{
		Matchers:  amMatchers,
		StartsAt:  now,
		EndsAt:    now.Add(time.Duration(hourInt) * time.Hour),
		CreatedBy: "运维平台快捷静默",
		Comment:   fmt.Sprintf("通过快捷按钮屏蔽告警，时长: %d小时", hourInt),
	}

	jsonStr, err := json.Marshal(si)
	if err != nil {
		c.String(http.StatusInternalServerError, "内部错误: 序列化失败")
		return
	}

	// 3. 发送请求给 Alertmanager
	url := fmt.Sprintf("%s/%s", sc.AlertManagerApi, "api/v2/silences")
	emptyMap := map[string]string{}

	bodyBytes, err := common.PostWithJsonString(sc.Logger, "AlertSilence",
		sc.HttpRequestGlobalTimeoutSeconds,
		url, string(jsonStr), emptyMap, emptyMap)

	if err != nil {
		sc.Logger.Error("告警静默调用alertmanager失败", zap.Error(err), zap.String("payload", string(jsonStr)))
		c.String(http.StatusBadRequest, fmt.Sprintf("调用 Alertmanager 接口失败 %v", err.Error()))
		return
	}
	// 解析屏蔽结果
	var sr *SilenceResponse
	_ = json.Unmarshal(bodyBytes, &sr)
	if sr != nil {
		event.SilenceID = sr.SilenceID
		_ = event.UpdateOne()
	}

	//c.String(http.StatusOK, string(bodyBytes))
	c.Header("Content-Type", "text/html; charset=utf-8")
	htmlResponse := fmt.Sprintf(`
        <html>
            <body style="text-align:center; padding-top:50px; font-family:sans-serif;">
                <h1>屏蔽成功</h1>
                <p><strong>返回详情:</strong> %s</p>
                <p>此页面将在 3 秒后尝试自动关闭...</p>
                <script>
                    setTimeout(function() {
                        window.opener = null;
                        window.open('', '_self');
                        window.close();
                    }, 3000);
                </script>
            </body>
        </html>
    `, string(bodyBytes))

	c.String(http.StatusOK, htmlResponse)
}

func AlertUnSilence(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.AlertWebhookConfig)
	fingerprint := c.DefaultQuery("fingerprint", "")

	event, err := models.GetMonitorAlertEventByFingerPrintId(fingerprint)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("通过fingerprint去查询event错误 %s", err.Error()))
		return
	}

	url := fmt.Sprintf("%s/api/v2/silence/%s", sc.AlertManagerApi, event.SilenceID)
	emptyMap := map[string]string{}
	_, err = common.DeleteWithId(sc.Logger, "AlertSilence",
		sc.HttpRequestGlobalTimeoutSeconds,
		url, emptyMap, emptyMap)

	if err != nil {
		sc.Logger.Error("取消告警静默调用alertmanager失败", zap.Error(err), zap.String("url", url))
		c.String(http.StatusBadRequest, fmt.Sprintf("调用 Alertmanager 接口失败 %v", err.Error()))
		return
	}

	//c.String(http.StatusOK, "取消静默成功")
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, `
            <html>
                <body style="text-align:center; padding-top:50px; font-family:sans-serif;">
                    <h1>取消告警静默成功</h1>
                    <p>此页面将在 3 秒后尝试自动关闭...</p>
                    <script>
                        setTimeout(function() {
                            // 尝试关闭窗口
                            window.opener = null;
                            window.open('', '_self');
                            window.close();
                        }, 3000);
                    </script>
                </body>
            </html>
        `)
	return

}
