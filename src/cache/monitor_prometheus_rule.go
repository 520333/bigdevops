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

func (mc *MonitorCache) GetPrometheusAlertRuleConfigYamlByIp(ip string) string {
	mc.AlertLock.RLock()
	defer mc.AlertLock.RUnlock()
	return mc.AlertRuleMap[ip]
}

func (mc *MonitorCache) GeneratePrometheusAlertRuleConfigYaml(ctx context.Context) {
	pools, err := models.GetMonitorPromScrapePoolSupportAlertAll()
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
		oneMap := mc.GeneratePrometheusAlertRuleConfigYamlOnePool(pool)
		if oneMap != nil {
			for ip, out := range oneMap {
				ruleConfig[ip] = out
			}
		}
	}

	mc.AlertLock.Lock()
	mc.AlertRuleMap = ruleConfig
	mc.AlertLock.Unlock()

}

func (mc *MonitorCache) GeneratePrometheusAlertRuleConfigYamlOnePool(pool *models.MonitorPromScrapePool) map[string]string {
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

// ====================== record 聚合规则 ======================
func (mc *MonitorCache) GetPrometheusRecordRuleConfigYamlByIp(ip string) string {
	mc.RecordLock.RLock()
	defer mc.RecordLock.RUnlock()
	return mc.RecordRuleMap[ip]
}

func (mc *MonitorCache) GeneratePrometheusRecordRuleConfigYaml(ctx context.Context) {
	pools, err := models.GetMonitorPromScrapePoolSupportRecordAll()
	if err != nil {
		mc.Sc.Logger.Error("[监控模块]扫描数据库中的采集池支持预聚合规则失败", zap.Error(err))
		return
	}
	if len(pools) == 0 {
		return
	}

	ruleConfig := map[string]string{}
	for _, pool := range pools {
		pool := pool
		oneMap := mc.GeneratePrometheusRecordRuleConfigYamlOnePool(pool)
		if oneMap != nil {
			for ip, out := range oneMap {
				ruleConfig[ip] = out
			}
		}
	}

	mc.RecordLock.Lock()
	mc.RecordRuleMap = ruleConfig
	mc.RecordLock.Unlock()

}

func (mc *MonitorCache) GeneratePrometheusRecordRuleConfigYamlOnePool(pool *models.MonitorPromScrapePool) map[string]string {
	rules, err := models.GetMonitorPromRecordRuleByPoolId(pool.ID)
	if err != nil {
		mc.Sc.Logger.Error("[监控模块]根据采集池id查找所有的rule规则错误", zap.Error(err), zap.Any("池子", pool.Name))
		return nil
	}
	if len(rules) == 0 {
		return nil
	}
	ruleGroups := RuleGroups{}

	for _, rule := range rules {
		rule := rule
		rule.FillFrontAllData()
		//forD, _ := pmodel.ParseDuration(rule.ForTime)
		oneRule := rulefmt.Rule{
			//Alert:  rule.Name,
			Record: rule.RecordName,
			Expr:   rule.Expr,
			//For:    forD,
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
			mc.Sc.Logger.Error("[监控模块]生成采集池中单一IP要处理的record rule文件yaml解析错误",
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
