package cron

import (
	"bigdevops/src/models"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

// GodaddyRecord 对应 GoDaddy API 返回的记录结构
type GodaddyRecord struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Data string `json:"data"`
	TTL  int    `json:"ttl"`
}

// RunSyncCloudResourceDns DNS 同步主入口
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

	// 1. 获取本地数据库所有 DNS 的 Hash 映射
	dbUidHashM, err := models.GetResourceDnsUidAndHash()
	if err != nil {
		cm.Sc.Logger.Error("获取本地 DNS Hash 失败", zap.Error(err))
		return
	}

	allDns := &sync.Map{}

	// 初始化 workerpool，并发数设为 5 (可根据实际情况调整)
	wp := workerpool.New(5)

	// 2. 提交 GoDaddy 同步任务（按单个域名粒度投递）
	if cm.Sc.PublicCloudSyncC.GodaddyDns != nil && cm.Sc.PublicCloudSyncC.GodaddyDns.Enable {
		for _, domain := range cm.Sc.PublicCloudSyncC.GodaddyDns.Domains {
			domain := domain // 避免闭包变量捕获问题
			wp.Submit(func() {
				cm.RunSyncOneDomainGodaddy(domain, allDns)
			})
		}
	}

	// 3. 提交 Dynadot 同步任务（按单个域名粒度投递）
	if cm.Sc.PublicCloudSyncC.DynadotDns != nil && cm.Sc.PublicCloudSyncC.DynadotDns.Enable {
		for _, domain := range cm.Sc.PublicCloudSyncC.DynadotDns.Domains {
			domain := domain // 避免闭包变量捕获问题
			wp.Submit(func() {
				cm.RunSyncOneDomainDynadot(domain, allDns)
			})
		}
	}

	// 阻塞等待所有域名的 API 抓取任务完成
	wp.StopWait()

	// 4. 增量对比与入库逻辑
	cm.RunSyncCloudResourceDnsToDb(allDns, dbUidHashM)
}

// RunSyncOneDomainGodaddy 同步单个 GoDaddy 域名记录
func (cm *CronManager) RunSyncOneDomainGodaddy(domain string, allDns *sync.Map) {
	config := cm.Sc.PublicCloudSyncC.GodaddyDns
	client := resty.New()
	var records []GodaddyRecord

	// 1. 调用 GoDaddy API
	resp, err := client.R().
		SetHeader("Authorization", fmt.Sprintf("sso-key %s:%s", config.AccessKeyId, config.AccessKeySecret)).
		SetResult(&records).
		Get(fmt.Sprintf("https://api.godaddy.com/v1/domains/%s/records", domain))

	if err != nil || resp.IsError() {
		cm.Sc.Logger.Error("同步 GoDaddy 记录失败", zap.String("domain", domain), zap.Error(err))
		return
	}

	// 2. 转换并存入全量 Map
	for _, rec := range records {
		dnsObj := &models.ResourceDns{
			Vendor: "godaddy",
			Domain: domain,
			Name:   rec.Name,
			Type:   rec.Type,
			Value:  rec.Data,
			TTL:    rec.TTL,
		}
		dnsObj.Hash = dnsObj.GenHash()

		// 注意：此处不查库，统一推迟到入库对比阶段，防止并发打挂数据库
		uid := fmt.Sprintf("%s-%s-%s", dnsObj.Domain, dnsObj.Name, dnsObj.Type)
		allDns.Store(uid, dnsObj)
	}
}

// RunSyncOneDomainDynadot 同步单个 Dynadot 域名记录
func (cm *CronManager) RunSyncOneDomainDynadot(domain string, allDns *sync.Map) {
	// config := cm.Sc.PublicCloudSyncC.DynadotDns
	// TODO: 实现 Dynadot API 的具体调用逻辑

	// 示例解析数据
	dnsObj := &models.ResourceDns{
		Vendor: "dynadot",
		Domain: domain,
		// ... 填充 Name, Type, Value, TTL
	}
	dnsObj.Hash = dnsObj.GenHash()

	uid := fmt.Sprintf("%s-%s-%s", dnsObj.Domain, dnsObj.Name, dnsObj.Type)
	allDns.Store(uid, dnsObj)
}

// RunSyncCloudResourceDnsToDb 处理全量数据与本地数据的比对并执行数据库操作
func (cm *CronManager) RunSyncCloudResourceDnsToDb(allDns *sync.Map, dbUidHashM map[string]string) {
	start := time.Now()
	toAddSet := make([]*models.ResourceDns, 0)
	toModSet := make([]*models.ResourceDns, 0)
	var toDelUids []string
	localUidSet := make(map[string]struct{})

	var toAddNum, toModNum, toDelNum int
	var suAddNum, suModNum, suDelNum int

	// 1. 遍历远端抓取到的数据，计算新增和更新
	allDns.Range(func(k, v interface{}) bool {
		uid := k.(string)
		dnsObj := v.(*models.ResourceDns)
		localUidSet[uid] = struct{}{}
		dbHash, ok := dbUidHashM[uid]
		if !ok {
			// 本地不存在，需新增
			toAddSet = append(toAddSet, dnsObj)
			toAddNum++
		} else if dbHash != dnsObj.Hash {
			// Hash 不一致，需更新
			toModSet = append(toModSet, dnsObj)
			toModNum++
		}
		return true
	})

	// 2. 遍历本地数据，计算需要删除的冗余数据
	for uid := range dbUidHashM {
		if _, ok := localUidSet[uid]; !ok {
			toDelUids = append(toDelUids, uid)
			toDelNum++
		}
	}

	// 3. 执行数据库变更 (按序: 新增 -> 更新 -> 删除)
	for _, obj := range toAddSet {
		// 入库前关联计算
		cm.associateDnsWithAsset(obj)
		if err := obj.CreateOne(); err == nil {
			suAddNum++
		}
	}

	for _, obj := range toModSet {
		old, _ := models.GetResourceDnsByUid(obj.Domain, obj.Name, obj.Type)
		if old != nil {
			obj.ID = old.ID
			// 更新前关联计算
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
			if old != nil {
				if err := old.DeleteOne(); err == nil {
					suDelNum++
				}
			}
		}
	}

	// ====================================================================
	// 👇 核心补丁：存量未关联 DNS 的强行补偿逻辑 (破除 Hash 没变导致不关联的死角)
	// ====================================================================
	var unlinkedDnsList []models.ResourceDns
	// 捞出所有目前还没关联上底层资产的 DNS
	models.Db.Where("associated_instance_id = '' OR associated_instance_id IS NULL").Find(&unlinkedDnsList)

	compensationNum := 0
	for _, unlinked := range unlinkedDnsList {
		dnsPtr := &unlinked
		cm.associateDnsWithAsset(dnsPtr)

		// 如果这次兜底匹配到了资产，直接执行更新！
		if dnsPtr.AssociatedInstanceId != "" {
			if err := dnsPtr.UpdateOne(); err == nil {
				compensationNum++
				cm.Sc.Logger.Info("DNS 存量记录终于匹配上资产了！",
					zap.String("domain", dnsPtr.Domain),
					zap.String("name", dnsPtr.Name),
					zap.String("type", dnsPtr.Type),
					zap.String("value", dnsPtr.Value),
					zap.String("associated_id", dnsPtr.AssociatedInstanceId),
				)
			}
		}
	}
	// ====================================================================

	tookSeconds := time.Since(start).Seconds()
	cm.Sc.Logger.Info("同步 DNS 结果打印",
		zap.Int("远端抓取总数", len(localUidSet)),
		zap.Int("本地数据库总数", len(dbUidHashM)),
		zap.Int("toAddNum", toAddNum),
		zap.Int("toModNum", toModNum),
		zap.Int("toDelNum", toDelNum),
		zap.Int("suAddNum", suAddNum),
		zap.Int("suModNum", suModNum),
		zap.Int("suDelNum", suDelNum),
		zap.Int("强行补偿关联数", compensationNum),
		zap.Float64("timeTook", tookSeconds),
	)
}

// associateDnsWithAsset 增强版：将 DNS 记录全方位关联到现有的 ECS/ELB/RDS 资产
func (cm *CronManager) associateDnsWithAsset(dns *models.ResourceDns) {
	if dns.Value == "" {
		return
	}

	// ====== 1. 处理 A 记录 (IP地址) ======
	if dns.Type == "A" {
		// 1.1 先去 ECS 里找 (涵盖 ECS 的公网/私网 IP)
		ecs, err := models.GetResourceEcsByIp(dns.Value)
		if err == nil && ecs != nil {
			dns.AssociatedInstanceId = ecs.InstanceId
			dns.EcsInstanceId = ecs.InstanceId // 兼容旧字段
			return
		}

		// 1.2 如果 ECS 没找到，去 ELB 找 (涵盖内部 ELB 的私网 VIP)
		elb, err := models.GetResourceElbByIp(dns.Value)
		if err == nil && elb != nil {
			dns.AssociatedInstanceId = elb.LoadBalancerId
			return
		}

		// 1.3 如果 ELB 也没找到，去 RDS 找 (涵盖数据库直接通过 IP 解析的情况)
		rds, err := models.GetResourceRdsByHostOrIp(dns.Value)
		if err == nil && rds != nil {
			dns.AssociatedInstanceId = rds.DBInstanceId
			return
		}
	}

	// ====== 2. 处理 CNAME 记录 (域名别名) ======
	if dns.Type == "CNAME" {
		// 2.1 去 ELB 里找 dns_name
		elb, err := models.GetResourceElbByDnsName(dns.Value)
		if err == nil && elb != nil {
			dns.AssociatedInstanceId = elb.LoadBalancerId
			return
		}

		// 2.2 去 RDS 里找 host 连接地址 (公有云 RDS 绝大多数提供的是 CNAME 域名)
		rds, err := models.GetResourceRdsByHostOrIp(dns.Value)
		if err == nil && rds != nil {
			dns.AssociatedInstanceId = rds.DBInstanceId
			return
		}
	}
}
