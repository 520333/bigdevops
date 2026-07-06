package cache

import (
	"bigdevops/src/common"
	"bigdevops/src/models"
	"context"
	"fmt"
	"strings"

	ac "github.com/prometheus/alertmanager/config"
	amcommon "github.com/prometheus/alertmanager/config/common"
	alabels "github.com/prometheus/alertmanager/pkg/labels"
	pm "github.com/prometheus/common/model"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

func (mc *MonitorCache) GetAlertManagerMainConfigYamlByIp(ip string) string {
	mc.RLock()
	defer mc.RUnlock()
	return mc.AlertManagerMainConfigMap[ip]
}

func (mc *MonitorCache) GenerateAlertManagerMainConfigYaml(ctx context.Context) {
	pools, err := models.GetMonitorAlertManagerPoolAll()
	if err != nil {
		mc.Sc.Logger.Error("[监控模块]扫描数据库中的采集池失败", zap.Error(err))
		return
	}
	if len(pools) == 0 {
		return
	}
	mainConfigMap := map[string]string{}

	for _, pool := range pools {
		pool := pool
		allConfig := mc.GenerateAlertManagerMainConfigOneYaml(pool)

		routes, receivers := mc.GenerateAlertManagerRouteConfigYamlOnePool(pool)
		if routes != nil {
			allConfig.Route.Routes = routes
		}
		if receivers != nil {
			if allConfig.Receivers != nil {
				allConfig.Receivers = append(allConfig.Receivers, receivers...)
			}
			allConfig.Receivers = receivers
		}

		out, err := yaml.Marshal(allConfig)
		// ====删除一些没用的字段====
		var m map[string]any
		_ = yaml.Unmarshal(out, &m)

		delete(m, "templates")

		if g, ok := m["global"].(map[string]any); ok {
			delete(g, "smtp_require_tls")
		}
		if g, ok := m["route"].(map[string]any); ok {
			delete(g, "continue")
		}
		if receivers, ok := m["receivers"].([]any); ok {
			for _, r := range receivers {
				if receiver, ok := r.(map[string]any); ok {
					if webhooks, ok := receiver["webhook_configs"].([]any); ok {
						for _, wh := range webhooks {
							if webhook, ok := wh.(map[string]any); ok {
								// 删除你不需要的字段
								delete(webhook, "max_alerts")
								delete(webhook, "timeout")
								delete(webhook, "url_file")
							}
						}
					}
				}
			}
		}
		out, _ = yaml.Marshal(m)

		// ======================
		if err != nil {
			mc.Sc.Logger.Error("[监控模块]根据alertmanager生成主配置文件错误", zap.Error(err), zap.Any("池子", pool.Name))
			continue
		}

		outStr := string(out)

		// 重新查一遍当前 pool 下的发送组（因为生成 route 时遍历的顺序和这里查询的顺序是一致的）
		sendGroups, err := models.GetMonitorAlertManagerSendGroupByPoolId(pool.ID)
		if err == nil {
			for _, sendGroup := range sendGroups {
				webhookUrl := fmt.Sprintf("%s/webhook?%s=%v",
					mc.Sc.MonitorComputeC.AlertWebhookAddr,
					common.MONITOR_ALERT_MATCH_KEY,
					sendGroup.ID,
				)

				// 替换第一个遇到的 url: <secret>，并加上双引号防止 YAML 特殊字符解析报错
				// 注意 YAML 序列化出来可能是 url: <secret> (带有空格)
				outStr = strings.Replace(outStr, "url: <secret>", fmt.Sprintf(`url: "%s"`, webhookUrl), 1)
			}
		}

		mc.Sc.Logger.Debug("[监控模块]根据alertmanager生成主配置文件成功", zap.Any("池子", pool.Name), zap.Any("config", outStr))
		//fileName := fmt.Sprintf("alertmanager_%s.yaml", pool.Name)
		//_ = os.WriteFile(fileName, []byte(outStr), 0666)

		for _, ip := range pool.AlertManagerInstanceId {
			mainConfigMap[ip] = outStr
		}
	}
	mc.Lock()
	mc.AlertManagerMainConfigMap = mainConfigMap
	mc.Unlock()
}

// GenerateAlertManagerMainConfigOneYaml 主配置文件
func (mc *MonitorCache) GenerateAlertManagerMainConfigOneYaml(pool *models.MonitorAlertManagerPool) *ac.Config {
	rt, _ := pm.ParseDuration(pool.ResolveTimeout)
	groupWait, _ := pm.ParseDuration(pool.GroupWait)
	groupInterval, _ := pm.ParseDuration(pool.GroupInterval)
	repeatInterval, _ := pm.ParseDuration(pool.RepeatInterval)

	pc := &ac.Config{
		Global: &ac.GlobalConfig{
			ResolveTimeout: rt,
		},
		Route: &ac.Route{
			Receiver:       pool.Receiver,
			GroupWait:      &groupWait,
			GroupInterval:  &groupInterval,
			RepeatInterval: &repeatInterval,
		},
	}
	//for _, l := range pool.GroupBy {
	//	labelName := pm.LabelName(l)
	//	if !labelName.IsValid() {
	//		continue
	//	}
	//	//pc.Route.GroupBy = append(pc.Route.GroupBy, labelName)
	//	pc.Route.GroupByStr = append(pc.Route.GroupByStr, l)
	//}
	for _, l := range pool.GroupBy {
		pc.Route.GroupByStr = append(pc.Route.GroupByStr, l)
	}
	if pc.Route.Receiver != "" {
		//pc.Receivers = []ac.Receiver{}
		pc.Receivers = append(pc.Receivers, ac.Receiver{
			Name: pc.Route.Receiver,
		})

	}

	return pc
}

// GenerateAlertManagerRouteConfigYamlOnePool 生成route配置
func (mc *MonitorCache) GenerateAlertManagerRouteConfigYamlOnePool(pool *models.MonitorAlertManagerPool) ([]*ac.Route, []ac.Receiver) {
	sendGroups, err := models.GetMonitorAlertManagerSendGroupByPoolId(pool.ID)
	if err != nil {
		mc.Sc.Logger.Error("[监控模块]根据alert池ID找到所有的发送组错误", zap.Error(err))
		return nil, nil
	}
	if len(sendGroups) == 0 {
		return nil, nil
	}
	routes := make([]*ac.Route, 0)
	receivers := make([]ac.Receiver, 0)
	for _, sendGroup := range sendGroups {
		sendGroup := sendGroup
		repeatInterval, _ := pm.ParseDuration(sendGroup.RepeatInterval)

		sendGroupMatcher, _ := alabels.NewMatcher(alabels.MatchEqual, common.MONITOR_ALERT_MATCH_KEY, fmt.Sprintf("%d", sendGroup.ID))
		route := &ac.Route{
			Receiver: sendGroup.Name,
			Continue: true,
			Matchers: []*alabels.Matcher{
				sendGroupMatcher,
			},
			// 晚上不打电话
			//MuteTimeIntervals:   nil,
			//ActiveTimeIntervals: nil,
			RepeatInterval: &repeatInterval,
		}

		webhookUrl := fmt.Sprintf("%s/webhook?%s=%v",
			mc.Sc.MonitorComputeC.AlertWebhookAddr,
			common.MONITOR_ALERT_MATCH_KEY,
			sendGroup.ID,
		)
		sendResolved := false
		if sendGroup.SendResolved == 1 {
			sendResolved = true

		}
		receiver := &ac.Receiver{
			Name: sendGroup.Name,
			WebhookConfigs: []*ac.WebhookConfig{
				{
					NotifierConfig: amcommon.NotifierConfig{
						VSendResolved: sendResolved,
					},
					URL: ac.SecretTemplateURL(webhookUrl),
				},
			},
		}
		routes = append(routes, route)
		receivers = append(receivers, *receiver)
	}
	return routes, receivers
}
