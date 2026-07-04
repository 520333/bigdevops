package cache

import (
	"bigdevops/src/models"
	"context"

	pmodel "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/rulefmt"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type RuleGroup struct {
	Name  string         `yaml:"name"`
	Rules []rulefmt.Rule `yaml:"rules"`
}
type RuleGroups struct {
	Groups []RuleGroup `yaml:"groups"`
}

func (mc *MonitorCache) GetPrometheusRuleConfigYamlByIp(ip string) string {
	mc.RLock()
	defer mc.RUnlock()
	return mc.AlertRuleMap[ip]
}

func (mc *MonitorCache) GeneratePrometheusRuleConfigYaml(ctx context.Context) {
	pools, err := models.GetMonitorScrapePoolSupportAlertAll()
	if err != nil {
		mc.Sc.Logger.Error("[监控模块]扫描数据库中的采集池支持告警规则失败", zap.Error(err))
		return
	}
	if len(pools) == 0 {
		return
	}

	ruleConfig := map[string]string{}
	for _, pool := range pools {
		pool := pool
		oneMap := mc.GeneratePrometheusRuleConfigYamlOnePool(pool)
		if oneMap != nil {
			for ip, out := range oneMap {
				ruleConfig[ip] = out
			}
		}
	}

	mc.Lock()
	mc.AlertRuleMap = ruleConfig
	mc.Unlock()

}

func (mc *MonitorCache) GeneratePrometheusRuleConfigYamlOnePool(pool *models.MonitorScrapePool) map[string]string {
	rules, err := models.GetMonitorPromAlertRuleByPoolId(pool.ID)
	if err != nil {
		mc.Sc.Logger.Error("[监控模块]根据采集池id查找所有的rule规则错误", zap.Error(err), zap.Any("池子", pool.Name))
		return nil
	}
	if len(rules) == 0 {
		return nil
	}
	ruleGroups := RuleGroups{}

	//ruleMap := map[string]rulefmt.Rule{}
	for _, rule := range rules {
		rule := rule
		rule.FillFrontAllData()
		forD, _ := pmodel.ParseDuration(rule.ForTime)
		oneRule := rulefmt.Rule{
			Alert:       rule.Name,
			Expr:        rule.Expr,
			For:         forD,
			Labels:      rule.LabelsM,
			Annotations: rule.AnnotationsM,
		}

		ruleGroup := RuleGroup{
			Name:  rule.Name,
			Rules: []rulefmt.Rule{oneRule},
		}
		ruleGroups.Groups = append(ruleGroups.Groups, ruleGroup)

	}

	num := len(pool.PrometheusInstances)
	ruleMap := map[string]string{}
	for i, ip := range pool.PrometheusInstances {
		ip := ip
		myRuleGroups := RuleGroups{}
		for j, group := range ruleGroups.Groups {
			if j%num == i {
				myRuleGroups.Groups = append(myRuleGroups.Groups, group)
			}
		}
		myOut, err := yaml.Marshal(myRuleGroups)
		if err != nil {
			mc.Sc.Logger.Error("[监控模块]生成采集池中单一IP要处理的rule文件yaml解析错误",
				zap.Error(err), zap.Any("池子", pool.Name),
				zap.Any("ip", ip))
			continue
		}
		//fileName := fmt.Sprintf("rule_%s_%s.yaml", pool.Name, ip)
		//_ = os.WriteFile(fileName, myOut, 0666)
		ruleMap[ip] = string(myOut)
	}
	return ruleMap
}
