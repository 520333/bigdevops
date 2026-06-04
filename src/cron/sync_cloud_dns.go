package cron

import (
	"bigdevops/src/models"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

func (cm *CronManager) RunSyncCloudResourceDns(ctx context.Context) {
	cm.Sc.Logger.Info("DNS 同步任务开始....")
	if cm.Sc.PublicCloudSyncC == nil {
		cm.Sc.Logger.Error("【严重错误】PublicCloudSyncC 配置完全为空，请检查 server.yml 缩进")
		return
	}

	godaddy := cm.Sc.PublicCloudSyncC.GodaddyDns
	if godaddy == nil {
		cm.Sc.Logger.Error("【严重错误】GodaddyDns 节点为 nil，请检查是否在 public_cloud_sync 节点下")
	} else {
		cm.Sc.Logger.Info("DNS 配置检测",
			zap.Bool("GoDaddyEnabled", godaddy.Enable),
			zap.Int("DomainsCount", len(godaddy.Domains)))
	}
	cm.SetDnsSynced(false)      // 标记开始
	defer cm.SetDnsSynced(true) // 确保无论如何最后都会置回 true

	cm.Sc.Logger.Info("DNS 同步任务开始....")

	// 1. 获取本地数据库所有 DNS 的 Hash 映射
	dbUidHashM, err := models.GetResourceDnsUidAndHash()
	if err != nil {
		cm.Sc.Logger.Error("获取本地 DNS Hash 失败", zap.Error(err))
		return
	}

	allDns := &sync.Map{}
	var wg sync.WaitGroup

	// 2. 并发同步 GoDaddy
	if cm.Sc.PublicCloudSyncC.GodaddyDns != nil && cm.Sc.PublicCloudSyncC.GodaddyDns.Enable {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cm.RunSyncOneDnsGodaddy(allDns)
		}()
	}

	// 3. 并发同步 Dynadot
	if cm.Sc.PublicCloudSyncC.DynadotDns != nil && cm.Sc.PublicCloudSyncC.DynadotDns.Enable {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cm.RunSyncOneDnsDynadot(allDns)
		}()
	}

	wg.Wait()

	// 4. 增量对比与入库逻辑 (与你的 ECS/ELB 同步逻辑完全一致)
	cm.SyncDnsToDb(allDns, dbUidHashM)

}

func (cm *CronManager) SyncDnsToDb(allDns *sync.Map, dbUidHashM map[string]string) {
	start := time.Now()
	toAddSet := make([]*models.ResourceDns, 0)
	toModSet := make([]*models.ResourceDns, 0)
	var toDelUids []string
	localUidSet := make(map[string]struct{})
	var toAddNum, toModNum, toDelNum int
	var suAddNum, suModNum, suDelNum int

	// 1. 遍历远端
	allDns.Range(func(k, v interface{}) bool {
		uid := k.(string)
		dnsObj := v.(*models.ResourceDns)
		localUidSet[uid] = struct{}{}
		dbHash, ok := dbUidHashM[uid]
		if !ok {
			toAddSet = append(toAddSet, dnsObj)
			toAddNum++
		} else if dbHash != dnsObj.Hash {
			toModSet = append(toModSet, dnsObj)
			toModNum++
		}
		return true
	})

	// 2. 遍历本地找出要删除的
	for uid := range dbUidHashM {
		if _, ok := localUidSet[uid]; !ok {
			toDelUids = append(toDelUids, uid)
			toDelNum++
		}
	}

	// 3. 执行变更 (入库)
	for _, obj := range toAddSet {
		cm.associateDnsWithAsset(obj)
		if err := obj.CreateOne(); err == nil {
			suAddNum++
		}
	}
	for _, obj := range toModSet {
		old, _ := models.GetResourceDnsByUid(obj.Domain, obj.Name, obj.Type)
		if old != nil {
			obj.ID = old.ID
			cm.associateDnsWithAsset(obj)
			if err := obj.UpdateOne(); err == nil {
				suModNum++
			}
		}
	}
	for _, uid := range toDelUids {
		parts := strings.Split(uid, "-")
		if len(parts) == 3 {
			old, _ := models.GetResourceDnsByUid(parts[0], parts[1], parts[2])
			if old != nil && old.DeleteOne() == nil {
				suDelNum++
			}
		}
	}
	tookSeconds := time.Since(start).Seconds()
	cm.Sc.Logger.Info("同步dns结果打印",
		zap.Any("公有云总数", len(localUidSet)),
		zap.Any("本地数据库", len(dbUidHashM)),
		zap.Any("toAddNum", toAddNum),
		zap.Any("toModNum", toModNum),
		zap.Any("toDelNum", toDelNum),
		zap.Any("suAddNum", suAddNum),
		zap.Any("suModNum", suModNum),
		zap.Any("suDelNum", suDelNum),
		zap.Any("timeTook", tookSeconds),
	)

}
func (cm *CronManager) RunSyncOneDnsDynadot(allDns *sync.Map) {
	config := cm.Sc.PublicCloudSyncC.DynadotDns
	for _, domain := range config.Domains {
		// ... 调用 Dynadot API
		// 解析数据并转换为 *models.ResourceDns
		dnsObj := &models.ResourceDns{
			Vendor: "dynadot",
			Domain: domain,
			// ... 填充 Name, Type, Value, TTL
		}
		dnsObj.Hash = dnsObj.GenHash()

		// 自动资产关联 (通过 IP 匹配到 ECS)
		// cm.associateDnsWithAsset(dnsObj)

		uid := fmt.Sprintf("%s-%s-%s", dnsObj.Domain, dnsObj.Name, dnsObj.Type)
		allDns.Store(uid, dnsObj)
	}
}

type GodaddyRecord struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Data string `json:"data"`
	TTL  int    `json:"ttl"`
}

func (cm *CronManager) RunSyncOneDnsGodaddy(allDns *sync.Map) {
	config := cm.Sc.PublicCloudSyncC.GodaddyDns
	client := resty.New()

	for _, domain := range config.Domains {
		var records []GodaddyRecord

		// 1. 调用 GoDaddy API
		resp, err := client.R().
			SetHeader("Authorization", fmt.Sprintf("sso-key %s:%s", config.AccessKeyId, config.AccessKeySecret)).
			SetResult(&records).
			Get(fmt.Sprintf("https://api.godaddy.com/v1/domains/%s/records", domain))

		if err != nil || resp.IsError() {
			cm.Sc.Logger.Error("同步 GoDaddy 记录失败", zap.String("domain", domain), zap.Error(err))
			continue
		}

		// 2. 转换并存入 allDns
		for _, rec := range records {
			// 过滤掉部分不需要的系统记录（可选）
			dnsObj := &models.ResourceDns{
				Vendor: "godaddy",
				Domain: domain,
				Name:   rec.Name,
				Type:   rec.Type,
				Value:  rec.Data,
				TTL:    rec.TTL,
			}
			dnsObj.Hash = dnsObj.GenHash()

			// 自动关联 ECS (如果 Value 是 IP)
			cm.associateDnsWithAsset(dnsObj)

			uid := fmt.Sprintf("%s-%s-%s", dnsObj.Domain, dnsObj.Name, dnsObj.Type)
			allDns.Store(uid, dnsObj)
		}
	}
}

func (cm *CronManager) associateDnsWithAsset(dns *models.ResourceDns) {
	// 1. 尝试匹配 ECS (通过 IP 匹配)
	if dns.Type == "A" {
		ecs, err := models.GetResourceEcsByIp(dns.Value)
		if err == nil && ecs != nil {
			dns.AssociatedInstanceId = ecs.InstanceId
			dns.EcsInstanceId = ecs.InstanceId
			return
		}
	}

	// 2. 尝试匹配 ELB (通过 CNAME 匹配 dns_name)
	// 如果是 CNAME，value 往往是 ELB 的域名
	if dns.Type == "CNAME" {
		elb, err := models.GetResourceElbByDnsName(dns.Value)
		if err == nil && elb != nil {
			dns.AssociatedInstanceId = elb.LoadBalancerId
			// 如果你的 elb 表没有 ecs_instance_id，这里可以留空或关联其他字段
			return
		}
	}
}
