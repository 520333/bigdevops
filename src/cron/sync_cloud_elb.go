package cron

import (
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/alb"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/nlb"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/slb"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awsElbv1 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	awsElbv1Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing/types"
	awsElbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	awsElbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	awsSts "github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gammazero/workerpool"
	"go.uber.org/zap"
)

func (cm *CronManager) RunSyncCloudResourceElb(ctx context.Context) {
	start := time.Now()
	// 首先应该判断上次的结果
	// 注意这里应该是 GetElbSynced，而不是 GetEcsSynced
	if !cm.GetElbSynced() {
		cm.Sc.Logger.Info("ELB上次同步还未完成，本次cron跳过")
		return
	}
	// 上次完成了，开始新的cron
	cm.SetElbSynced(false)

	// 获取本地的uid对应的hashM (需要您在 models 中实现这个方法)
	dbUidHashM, err := models.GetResourceLbUidAndHash()
	if err != nil {
		cm.Sc.Logger.Error("获取本地ELB Hash失败", zap.Error(err))
	}
	if dbUidHashM == nil {
		dbUidHashM = make(map[string]string)
	}

	// 多个账号汇总
	allElb := &sync.Map{}
	wp := workerpool.New(5)

	// 1. 提交阿里云同步任务
	for _, alic := range cm.Sc.PublicCloudSyncC.AliCloud {
		if !alic.Enable {
			cm.Sc.Logger.Info("阿里云同步任务已禁用，跳过同步", zap.String("region", alic.RegionId))
			continue
		}
		alic := alic
		wp.Submit(func() {
			cm.RunSyncOneCloudElbAli(alic, allElb)
		})
	}

	for _, awsc := range cm.Sc.PublicCloudSyncC.AwsCloud {
		if !awsc.Enable {
			cm.Sc.Logger.Info("AWS 账号配置已禁用，跳过 ELB 同步", zap.String("Region", awsc.RegionId))
			continue
		}
		awsc := awsc
		wp.Submit(func() {
			cm.RunSyncOneCloudElbAws(ctx, awsc, allElb)
		})
	}
	// 等待所有云厂商的协程抓取完毕
	wp.StopWait()

	// ================== 开始增量比对 (Add, Mod, Del) ==================
	toAddSet := make([]*models.ResourceElb, 0)
	toModSet := make([]*models.ResourceElb, 0)
	var toDelUids []string
	localUidSet := make(map[string]struct{})
	var toAddNum, toModNum, toDelNum int
	var suAddNum, suModNum, suDelNum int

	// 遍历远端抓取到的数据
	allElb.Range(func(k, v interface{}) bool {
		uid := k.(string)
		elbObj := v.(*models.ResourceElb)
		localUidSet[uid] = struct{}{}

		dbHash, ok := dbUidHashM[uid]
		if !ok {
			// 本地没有，新增
			toAddSet = append(toAddSet, elbObj)
			toAddNum++
		} else {
			// 本地有，对比Hash
			if dbHash != elbObj.Hash {
				toModSet = append(toModSet, elbObj)
				toModNum++
			}
		}
		return true
	})

	// 遍历本地找出需要删除的
	for uid := range dbUidHashM {
		if _, ok := localUidSet[uid]; !ok {
			toDelUids = append(toDelUids, uid)
			toDelNum++
		}
	}

	// ================== 执行 DB 操作 ==================
	for _, obj := range toAddSet {
		err := obj.CreateOne()
		if err != nil {
			cm.Sc.Logger.Error("新增elb错误", zap.Error(err), zap.String("id", obj.LoadBalancerId))
			continue
		}
		suAddNum++
	}

	// 修改
	for _, obj := range toModSet {
		err := obj.UpdateOne()
		if err != nil {
			cm.Sc.Logger.Error("更新elb错误", zap.Error(err), zap.String("id", obj.LoadBalancerId))
			continue
		}
		suModNum++
	}

	// 删除
	for _, uid := range toDelUids {
		dbObj, err := models.GetResourceLbByInstanceId(uid)
		if err != nil || dbObj == nil {
			continue
		}
		err = dbObj.DeleteOne()
		if err != nil {
			cm.Sc.Logger.Error("删除elb错误", zap.Error(err), zap.String("uid", uid))
			continue
		}
		suDelNum++
	}

	tookSeconds := time.Since(start).Seconds()
	cm.Sc.Logger.Info("同步elb结果打印",
		zap.Int("公有云总数", len(localUidSet)),
		zap.Int("本地数据库", len(dbUidHashM)),
		zap.Int("toAddNum", toAddNum),
		zap.Int("toModNum", toModNum),
		zap.Int("toDelNum", toDelNum),
		zap.Int("suAddNum", suAddNum),
		zap.Int("suModNum", suModNum),
		zap.Int("suDelNum", suDelNum),
		zap.Float64("timeTook", tookSeconds),
	)

	// 本次同步结束 标记完成
	cm.SetElbSynced(true)
}

// ConvertSlbCloudAli 将 ali CLB 对象转换为本地 DB 模型
func (cm *CronManager) ConvertSlbCloudAli(lb slb.LoadBalancer, account string, realAccount string) *models.ResourceElb {
	dbLb := &models.ResourceElb{
		ResourceCommon: models.ResourceCommon{
			Vendor:      "aliyun",
			AccountName: fmt.Sprintf("%s(%s)", account, realAccount),
			ZoneId:      lb.RegionId,
		},
		LoadBalancerId:    lb.LoadBalancerId,
		LoadBalancerName:  lb.LoadBalancerName,
		LoadBalancerType:  "clb", // SLB 统一标记为 clb
		Status:            lb.LoadBalancerStatus,
		AddressType:       lb.AddressType,
		PublicIpAddresses: []string{lb.Address}, // 早期 SLB 直接给 IP
		VpcId:             lb.VpcId,
	}
	dbLb.Hash = dbLb.GenHash()
	return dbLb
}

// ConvertAlbCloudAli 将 ali ALB 对象转换为本地 DB 模型
func (cm *CronManager) ConvertAlbCloudAli(lb alb.LoadBalancer, account string, realAccount string) *models.ResourceElb {
	dbLb := &models.ResourceElb{
		ResourceCommon: models.ResourceCommon{
			Vendor:      "aliyun",
			AccountName: fmt.Sprintf("%s(%s)", account, realAccount),
			//ZoneId:      lb,
		},
		LoadBalancerId:   lb.LoadBalancerId,
		LoadBalancerName: lb.LoadBalancerName,
		LoadBalancerType: "alb",
		Status:           lb.LoadBalancerStatus,
		AddressType:      lb.AddressAllocatedMode,
		DNSName:          lb.DNSName,
		VpcId:            lb.VpcId,
	}
	dbLb.Hash = dbLb.GenHash()
	return dbLb
}

// ConvertNlbCloudAli 将 ali NLB 对象转换为本地 DB 模型
func (cm *CronManager) ConvertNlbCloudAli(lb nlb.LoadbalancerInfo, account string, realAccount string) *models.ResourceElb {
	dbLb := &models.ResourceElb{
		ResourceCommon: models.ResourceCommon{
			Vendor:      "aliyun",
			AccountName: fmt.Sprintf("%s(%s)", account, realAccount),
			ZoneId:      lb.RegionId,
		},
		// ⚠️ 注意这里的字段取值：全是小写 b
		LoadBalancerId:   lb.LoadBalancerId,
		LoadBalancerName: lb.LoadBalancerName,
		LoadBalancerType: "nlb",
		Status:           lb.LoadBalancerStatus,
		AddressType:      lb.AddressType,
		DNSName:          lb.DNSName,
		VpcId:            lb.VpcId,
	}
	dbLb.Hash = dbLb.GenHash()
	return dbLb
}

// RunSyncOneCloudElbAli 阿里云负载均衡同步主逻辑
func (cm *CronManager) RunSyncOneCloudElbAli(alic *config.AliCloud, allElb *sync.Map) {
	cm.Sc.Logger.Info("elb 同步阿里云开始", zap.String("region", alic.RegionId), zap.String("account", alic.AccountName))

	// 1. 获取真实 Account ID
	stsClient, _ := sts.NewClientWithAccessKey(alic.RegionId, alic.AccessKeyId, alic.AccessKeySecret)
	stsReq := sts.CreateGetCallerIdentityRequest()
	stsReq.Scheme = "https"
	realAccountId := ""
	if stsResp, err := stsClient.GetCallerIdentity(stsReq); err == nil {
		realAccountId = stsResp.AccountId
	}

	// --- 2. 同步传统型 CLB (原 SLB) ---
	if slbClient, err := slb.NewClientWithAccessKey(alic.RegionId, alic.AccessKeyId, alic.AccessKeySecret); err == nil {
		req := slb.CreateDescribeLoadBalancersRequest()
		req.Scheme = "https"
		req.PageSize = requests.NewInteger(100) // 默认拉取最多100条
		if resp, err := slbClient.DescribeLoadBalancers(req); err == nil {
			for _, item := range resp.LoadBalancers.LoadBalancer {
				dbLb := cm.ConvertSlbCloudAli(item, alic.AccountName, realAccountId)
				if dbLb != nil {
					allElb.Store(dbLb.LoadBalancerId, dbLb)
				}
			}
		} else {
			cm.Sc.Logger.Error("查询阿里云 CLB 失败", zap.Error(err), zap.String("region", alic.RegionId))
		}
	}

	// --- 3. 同步应用型 ALB ---
	if albClient, err := alb.NewClientWithAccessKey(alic.RegionId, alic.AccessKeyId, alic.AccessKeySecret); err == nil {
		req := alb.CreateListLoadBalancersRequest()
		req.Scheme = "https"
		req.MaxResults = requests.NewInteger(100)
		if resp, err := albClient.ListLoadBalancers(req); err == nil {
			for _, item := range resp.LoadBalancers {
				// 👈 注意这里，加上了 alic.RegionId 作为第四个参数
				dbLb := cm.ConvertAlbCloudAli(item, alic.AccountName, realAccountId)
				if dbLb != nil {
					allElb.Store(dbLb.LoadBalancerId, dbLb)
				}
			}
		} else {
			cm.Sc.Logger.Error("查询阿里云 ALB 失败", zap.Error(err), zap.String("region", alic.RegionId))
		}
	}

	// --- 4. 同步网络型 NLB ---
	if nlbClient, err := nlb.NewClientWithAccessKey(alic.RegionId, alic.AccessKeyId, alic.AccessKeySecret); err == nil {
		req := nlb.CreateListLoadBalancersRequest()
		req.Scheme = "https"
		req.MaxResults = requests.NewInteger(100)
		if resp, err := nlbClient.ListLoadBalancers(req); err == nil {
			for _, item := range resp.LoadBalancers {
				dbLb := cm.ConvertNlbCloudAli(item, alic.AccountName, realAccountId)
				if dbLb != nil {
					allElb.Store(dbLb.LoadBalancerId, dbLb)
				}
			}
		} else {
			cm.Sc.Logger.Error("查询阿里云 NLB 失败", zap.Error(err), zap.String("region", alic.RegionId))
		}
	}

}

// ConvertElbCloudAwsV1 转换 AWS 第一代经典型 LB (CLB)
func (cm *CronManager) ConvertElbCloudAwsV1(lb awsElbv1Types.LoadBalancerDescription, account string, realAccount string) *models.ResourceElb {
	safeTime := func(t *time.Time) *time.Time {
		if t == nil {
			return nil
		}
		newT := *t
		return &newT
	}

	dbLb := &models.ResourceElb{
		ResourceCommon: models.ResourceCommon{
			Vendor:      "aws",
			AccountName: fmt.Sprintf("%s(%s)", account, realAccount),
		},
		// 注意：AWS v1 的 LB 没有 ARN，它的全局唯一标识就是名称
		LoadBalancerId:   aws.ToString(lb.LoadBalancerName),
		LoadBalancerName: aws.ToString(lb.LoadBalancerName),
		LoadBalancerType: "clb",
		DNSName:          aws.ToString(lb.DNSName),
		VpcId:            aws.ToString(lb.VPCId),
		CreationTime:     safeTime(lb.CreatedTime),
		AddressType:      aws.ToString(lb.Scheme), // internet-facing 或者 internal
		Status:           "active",                // CLB只要存在通常视为 active，具体健康状态取决于后端实例
	}

	// 提取绑定的安全组
	var sgIds []string
	for _, sg := range lb.SecurityGroups {
		sgIds = append(sgIds, sg)
	}
	sort.Strings(sgIds)
	dbLb.SecurityGroupIds = sgIds

	dbLb.Hash = dbLb.GenHash()
	return dbLb
}

// ConvertElbCloudAwsV2 转换 AWS 第二代 LB (ALB/NLB/GWLB)
func (cm *CronManager) ConvertElbCloudAwsV2(lb awsElbv2Types.LoadBalancer, account string, realAccount string) *models.ResourceElb {
	safeTime := func(t *time.Time) *time.Time {
		if t == nil {
			return nil
		}
		newT := *t
		return &newT
	}

	dbLb := &models.ResourceElb{
		ResourceCommon: models.ResourceCommon{
			Vendor:      "aws",
			AccountName: fmt.Sprintf("%s(%s)", account, realAccount),
		},
		// AWS v2 的全局唯一标识是 ARN
		LoadBalancerId:   aws.ToString(lb.LoadBalancerArn),
		LoadBalancerName: aws.ToString(lb.LoadBalancerName),
		LoadBalancerType: string(lb.Type),       // application, network, gateway
		Status:           string(lb.State.Code), // active, provisioning, failed 等
		DNSName:          aws.ToString(lb.DNSName),
		VpcId:            aws.ToString(lb.VpcId),
		AddressType:      string(lb.Scheme), // internet-facing 或者 internal
		CreationTime:     safeTime(lb.CreatedTime),
	}

	// 提取绑定的安全组 (NLB 可能没有安全组)
	var sgIds []string
	for _, sg := range lb.SecurityGroups {
		sgIds = append(sgIds, sg)
	}
	sort.Strings(sgIds)
	dbLb.SecurityGroupIds = sgIds

	dbLb.Hash = dbLb.GenHash()
	return dbLb
}

// RunSyncOneCloudElbAws AWS 负载均衡同步逻辑
func (cm *CronManager) RunSyncOneCloudElbAws(ctx context.Context, awsConf *config.AwsCloud, allElb *sync.Map) {
	cm.Sc.Logger.Info("elb 同步AWS开始", zap.String("region", awsConf.RegionId), zap.String("account", awsConf.AccountName))

	// 1. 初始化 AWS 基础配置
	cfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithRegion(awsConf.RegionId),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awsConf.AccessKeyId, awsConf.SecretAccessKey, "")),
	)
	if err != nil {
		cm.Sc.Logger.Error("AWS Config失败", zap.Error(err))
		return
	}

	// 2. 获取真实 12 位 Account ID
	stsClient := awsSts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &awsSts.GetCallerIdentityInput{})
	realAccountId := ""
	if err == nil {
		realAccountId = aws.ToString(identity.Account)
	}

	// --- 3. 获取 AWS ELB v1 (Classic Load Balancers) ---
	clientV1 := awsElbv1.NewFromConfig(cfg)
	paginatorV1 := awsElbv1.NewDescribeLoadBalancersPaginator(clientV1, &awsElbv1.DescribeLoadBalancersInput{})
	for paginatorV1.HasMorePages() {
		page, err := paginatorV1.NextPage(ctx)
		if err != nil {
			cm.Sc.Logger.Error("AWS ELB v1 查询失败", zap.Error(err))
			break
		}
		for _, lb := range page.LoadBalancerDescriptions {
			dbLb := cm.ConvertElbCloudAwsV1(lb, awsConf.AccountName, realAccountId)
			if dbLb != nil {
				allElb.Store(dbLb.LoadBalancerId, dbLb)
			}
		}
	}

	// --- 4. 获取 AWS ELB v2 (ALB, NLB, GWLB) ---
	clientV2 := awsElbv2.NewFromConfig(cfg)
	paginatorV2 := awsElbv2.NewDescribeLoadBalancersPaginator(clientV2, &awsElbv2.DescribeLoadBalancersInput{})
	for paginatorV2.HasMorePages() {
		page, err := paginatorV2.NextPage(ctx)
		if err != nil {
			cm.Sc.Logger.Error("AWS ELB v2 查询失败", zap.Error(err))
			break
		}
		for _, lb := range page.LoadBalancers {
			dbLb := cm.ConvertElbCloudAwsV2(lb, awsConf.AccountName, realAccountId)
			if dbLb != nil {
				allElb.Store(dbLb.LoadBalancerId, dbLb)
			}
		}
	}
}
