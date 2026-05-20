package cron

import (
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/alb"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/nlb"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/slb"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsElbv1 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	awsElbv1Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing/types"
	awsElbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	awsElbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	awsSts "github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gammazero/workerpool"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
	//openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	//ecs20140526 "github.com/alibabacloud-go/ecs-20140526/v7/client"
	//"github.com/alibabacloud-go/tea/tea"
)

// SyncCloudResourceManager 定义同步manager
func (cm *CronManager) SyncCloudResourceManager(ctx context.Context) error {
	go wait.UntilWithContext(ctx, cm.RunSyncCloudResource, time.Duration(cm.Sc.PublicCloudSyncC.RunIntervalSeconds)*time.Second)
	<-ctx.Done()
	cm.Sc.Logger.Info("SyncCloudResourceManager收到其他任务退出信号")
	return nil
}

func (cm *CronManager) RunSyncCloudResourceEcs(ctx context.Context) {
	start := time.Now()
	// 首先应该判断上次的结果
	if !cm.GetEcsSynced() {
		cm.Sc.Logger.Info("ecs上次同步还未完成，本次cron跳过")
		return
	}
	// 上次完成了，开始新的cron
	cm.SetEcsSynced(false)

	// 获取本地的uid对应的hashM
	dbUidHashM, err := models.GetResourceEcsUidAndHash()
	if err != nil {
		cm.Sc.Logger.Error("", zap.Error(err))
	}
	// 多个账号
	allEcs := &sync.Map{}

	wp := workerpool.New(5)
	for _, alic := range cm.Sc.PublicCloudSyncC.AliCloud {
		if !alic.Enable {
			cm.Sc.Logger.Info("阿里云同步任务已禁用，跳过同步", zap.String("region", alic.RegionId))
			continue
		}
		alic := alic
		wp.Submit(func() {
			cm.RunSyncOneCloudEcsAli(alic, allEcs)
		})
	}

	for _, awsc := range cm.Sc.PublicCloudSyncC.AwsCloud {
		if !awsc.Enable {
			cm.Sc.Logger.Info("AWS 账号配置已禁用，跳过同步", zap.String("Region", awsc.RegionId))
			continue
		}
		awsc := awsc
		wp.Submit(func() {
			// 注意这里要把 ctx 传下去
			cm.RunSyncOneCloudEc2Aws(ctx, awsc, allEcs)
		})
	}
	// TODO 华为 aws写在这里

	wp.StopWait()
	// 汇总结果 和db中进行增量对比

	// 计算 to_add to_del to_mod
	toAddSet := make([]*models.ResourceEcs, 0)
	toModSet := make([]*models.ResourceEcs, 0)
	var toDelUids []string
	localUidSet := make(map[string]struct{})
	var toAddNum, toModNum, toDelNum int
	var suAddNum, suModNum, suDelNum int
	// 遍历远端
	rangeFunc := func(k, v interface{}) bool {
		uid := k.(string)
		ecsObj := v.(*models.ResourceEcs)
		localUidSet[uid] = struct{}{}
		dbHash, ok := dbUidHashM[uid]
		if !ok {
			//在公有云有，本地没有新增
			toAddSet = append(toAddSet, ecsObj)
			toAddNum++
		} else {
			// 存在则对比hash
			if dbHash != ecsObj.Hash {
				toModSet = append(toModSet, ecsObj)
				toModNum++
			}
		}
		return true
	}
	allEcs.Range(rangeFunc)
	// 遍历本地
	for uid := range dbUidHashM {
		_, ok := localUidSet[uid]
		if !ok {
			// 说明不在公有云 在本地 执行删除
			toDelUids = append(toDelUids, uid)
			toDelNum++
		}
	}
	// 下面开始执行同步 新增
	for _, obj := range toAddSet {
		err := obj.CreateOne()
		if err != nil {
			cm.Sc.Logger.Error("新增ecs错误",
				zap.Error(err),
				zap.Any("id", obj.InstanceId),
				zap.Any("name", obj.InstanceName),
			)
			continue
		}
		cm.Sc.Logger.Info("ecs 新增成功",
			zap.Any("id", obj.InstanceId),
			zap.Any("name", obj.InstanceName),
		)
		suAddNum++
	}
	// 更新
	for _, obj := range toModSet {
		err := obj.UpdateOne()
		if err != nil {
			cm.Sc.Logger.Error("更新ecs错误",
				zap.Error(err),
				zap.Any("id", obj.InstanceId),
				zap.Any("name", obj.InstanceName),
			)
			continue
		}
		cm.Sc.Logger.Info("ecs 更新成功",
			zap.Any("id", obj.InstanceId),
			zap.Any("name", obj.InstanceName),
		)
		suModNum++
	}
	// 删除
	for _, uid := range toDelUids {
		dbObj, err := models.GetResourceEcsByInstanceId(uid)
		if err != nil {
			cm.Sc.Logger.Error("删除前查找ecs错误",
				zap.Error(err),
				zap.Any("uid", uid),
			)
			continue
		}
		err = dbObj.DeleteOne()
		if err != nil {
			cm.Sc.Logger.Error("删除ecs错误",
				zap.Error(err),
				zap.Any("uid", uid),
			)
			continue
		}
		cm.Sc.Logger.Info("ecs 删除成功",
			zap.Any("uid", uid),
		)
		suDelNum++
	}
	tookSeconds := time.Since(start).Seconds()
	cm.Sc.Logger.Info("同步ecs结果打印",
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
	cm.SetEcsSynced(true)

}

func (cm *CronManager) RunSyncCloudResourceElb(ctx context.Context) {
	start := time.Now()
	// 首先应该判断上次的结果
	// 注意这里应该是 GetElbSynced，而不是 GetEcsSynced
	if !cm.GetElbSynced() {
		cm.Sc.Logger.Info("elb上次同步还未完成，本次cron跳过")
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

	for _, obj := range toModSet {
		err := obj.UpdateOne()
		if err != nil {
			cm.Sc.Logger.Error("更新elb错误", zap.Error(err), zap.String("id", obj.LoadBalancerId))
			continue
		}
		suModNum++
	}

	for _, uid := range toDelUids {
		dbObj, err := models.GetResourceLbById(uid)
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

func (cm *CronManager) RunSyncCloudResource(ctx context.Context) {
	cm.Sc.Logger.Info("同步公有云资源中....")
	go cm.RunSyncCloudResourceEcs(ctx)
	go cm.RunSyncCloudResourceElb(ctx)
}

// ConvertEcsCloudAli 将 ali ECS 对象转换为本地 DB 模型
func (cm *CronManager) ConvertEcsCloudAli(ins ecs.Instance, eniEipMap map[string]string, insDiskMap map[string][]string, accountName string, realAccountId string) *models.ResourceEcs {
	privateIpMap := make(map[string]struct{})
	publicIpMap := make(map[string]struct{})
	var enis []string

	// 1. 提取基础 VPC 私网 IP 和主公网 IP (保持之前的逻辑)
	for _, ip := range ins.VpcAttributes.PrivateIpAddress.IpAddress {
		privateIpMap[ip] = struct{}{}
	}
	for _, ip := range ins.InnerIpAddress.IpAddress {
		privateIpMap[ip] = struct{}{}
	}
	for _, ip := range ins.PublicIpAddress.IpAddress {
		publicIpMap[ip] = struct{}{}
	}
	if ins.EipAddress.IpAddress != "" {
		publicIpMap[ins.EipAddress.IpAddress] = struct{}{}
	}

	// 2. 遍历所有弹性网卡 (主网卡和辅助网卡都在这里)
	for _, eni := range ins.NetworkInterfaces.NetworkInterface {
		enis = append(enis, eni.NetworkInterfaceId)

		// 提取网卡绑定的私网 IP
		if eni.PrimaryIpAddress != "" {
			privateIpMap[eni.PrimaryIpAddress] = struct{}{}
		}
		for _, pSet := range eni.PrivateIpSets.PrivateIpSet {
			if pSet.PrivateIpAddress != "" {
				privateIpMap[pSet.PrivateIpAddress] = struct{}{}
			}
		}

		// 【核心修复：检查这个辅助网卡有没有挂载 EIP】
		if eip, exists := eniEipMap[eni.NetworkInterfaceId]; exists {
			publicIpMap[eip] = struct{}{}
		}
	}

	// 3. 将 Map 转回 Slice，并排序 (排序防止无序导致 Hash 不一致)
	var privateIps []string
	for ip := range privateIpMap {
		privateIps = append(privateIps, ip)
	}
	sort.Strings(privateIps)

	var publicIps []string
	for ip := range publicIpMap {
		publicIps = append(publicIps, ip)
	}
	sort.Strings(publicIps)
	sort.Strings(enis)

	// 2. 标签提取 (最好也排个序)
	var tags []string
	for _, t := range ins.Tags.Tag {
		tags = append(tags, fmt.Sprintf("%s=%s", t.TagKey, t.TagValue))
	}
	sort.Strings(tags)

	// 【新增逻辑：提取硬盘并排序】
	var diskIds []string
	if disks, exists := insDiskMap[ins.InstanceId]; exists {
		diskIds = disks
	}
	sort.Strings(diskIds)

	// 4. 时间转换闭包 (阿里云通常返回 ISO8601 格式，例如 2021-01-01T12:00Z)
	parseTime := func(tStr string) *time.Time {
		if tStr == "" {
			return nil
		}
		// 阿里云标准的 ISO8601 格式
		t, err := time.Parse("2006-01-02T15:04Z", tStr)
		if err != nil {
			// 备用兜底解析
			t, err = time.Parse(time.RFC3339, tStr)
			if err != nil {
				return nil
			}
		}
		return &t
	}

	// 5. 组装本地 DB 模型
	dbIns := &models.ResourceEcs{
		ResourceCommon: models.ResourceCommon{

			Vendor:      "aliyun",
			AccountName: fmt.Sprintf("%s(%s)", accountName, realAccountId),
			Tags:        models.StringArray(tags), // 假设你的 StringArray 底层是 []string
			ZoneId:      ins.ZoneId,
		},
		InstanceId:   ins.InstanceId,
		InstanceName: ins.InstanceName,
		InstanceType: ins.InstanceType,
		VpcId:        ins.VpcAttributes.VpcId,
		VmType:       "1",
		OSType:       ins.OSType,
		//ZoneId:            ins.ZoneId,
		Status:            ins.Status,
		Cpu:               ins.Cpu,
		Memory:            ins.Memory,
		OSName:            ins.OSName,
		Description:       ins.Description,
		ImageId:           ins.ImageId,
		HostName:          ins.HostName,
		SecurityGroupIds:  models.StringArray(ins.SecurityGroupIds.SecurityGroupId),
		PrivateIpAddress:  models.StringArray(privateIps),
		PublicIpAddresses: models.StringArray(publicIps),
		NetworkInterfaces: models.StringArray(enis),
		DiskIds:           models.StringArray(diskIds),
		StartTime:         parseTime(ins.StartTime),
		CreationTime:      parseTime(ins.CreationTime),
		ExpiredTime:       parseTime(ins.ExpiredTime),
		AutoReleaseTime:   parseTime(ins.AutoReleaseTime),
	}

	// 6. 生成 Hash (依赖于此 Hash 来判断是否需要更新)
	// 【注意】这里调用了你 models 里的 GenHash 方法
	dbIns.Hash = dbIns.GenHash()

	return dbIns
	//return nil
}
func (cm *CronManager) RunSyncOneCloudEcsAli(alic *config.AliCloud, allEcs *sync.Map) {
	cm.Sc.Logger.Info("ecs 同步阿里云开始",
		zap.Any("地区", alic.RegionId),
		zap.Any("账号", alic.AccountName),
	)
	//MockDescribeInstancesResponse(allEcs)
	//return

	client, err := ecs.NewClientWithAccessKey(
		alic.RegionId,
		alic.AccessKeyId,
		alic.AccessKeySecret,
	)
	if err != nil {
		cm.Sc.Logger.Error("初始化 ecs client 错误:",
			zap.Error(err),
			zap.Any("RegionId", alic.RegionId),
			zap.Any("账号", alic.AccountName),
		)
		return
	}

	// 🌟 新增：获取阿里云账号真实 UID
	stsClient, _ := sts.NewClientWithAccessKey(alic.RegionId, alic.AccessKeyId, alic.AccessKeySecret)
	stsReq := sts.CreateGetCallerIdentityRequest()
	stsReq.Scheme = "https"

	realAccountId := ""
	if stsResp, err := stsClient.GetCallerIdentity(stsReq); err == nil {
		realAccountId = stsResp.AccountId
		cm.Sc.Logger.Info("成功获取阿里云账号ID", zap.String("accountId", realAccountId))
	}
	// ================== 【新增逻辑：获取该区域下所有的 EIP】 ==================
	eniEipMap := make(map[string]string) // 建立 网卡ID -> 公网IP 的映射

	eipReq := ecs.CreateDescribeEipAddressesRequest()
	eipReq.Scheme = "https"
	eipReq.PageSize = "100" // 注意：如果你们的 EIP 超过 100 个，这里要自己加个 for 循环做分页
	eipResp, err := client.DescribeEipAddresses(eipReq)
	if err != nil {
		cm.Sc.Logger.Error("获取 EIP 列表失败，辅助网卡的公网 IP 可能无法同步", zap.Error(err))
	} else {
		for _, eip := range eipResp.EipAddresses.EipAddress {
			// 当 EIP 绑定在弹性网卡上时，它的 InstanceType 会是 "NetworkInterface"，InstanceId 就是 eni-xxx
			if eip.InstanceType == "NetworkInterface" && eip.InstanceId != "" {
				eniEipMap[eip.InstanceId] = eip.IpAddress
			}
		}
		cm.Sc.Logger.Info("获取 EIP 映射成功", zap.Int("EIP数量", len(eniEipMap)))
	}

	// ================== 【2. 新增逻辑：获取所有 Disk 映射】 ==================
	insDiskMap := make(map[string][]string) // 建立 实例ID -> [硬盘ID集合] 的映射
	diskReq := ecs.CreateDescribeDisksRequest()
	diskReq.Scheme = "https"
	diskReq.PageSize = "100" // 注意：如果硬盘较多也要考虑分页
	diskResp, err := client.DescribeDisks(diskReq)
	if err != nil {
		cm.Sc.Logger.Error("获取 Disk 列表失败，硬盘 ID 可能无法同步", zap.Error(err))
	} else {
		for _, disk := range diskResp.Disks.Disk {
			// 如果硬盘挂载在实例上，它的 InstanceId 会有值
			if disk.InstanceId != "" {
				// 👇 【核心修改】：在这里将 disk.Size (单位: GB) 拼接到 disk.DiskId 后面
				diskInfo := fmt.Sprintf("%s(%dG)", disk.DiskId, disk.Size)
				insDiskMap[disk.InstanceId] = append(insDiskMap[disk.InstanceId], diskInfo)
			}
		}
		cm.Sc.Logger.Info("获取 Disk 映射成功", zap.Int("Disk数量", len(diskResp.Disks.Disk)))
	}
	// =========================================================================

	//resp := &ecs.DescribeInstancesResponse{}
	request := ecs.CreateDescribeInstancesRequest()
	request.Scheme = "https" // 推荐使用 https
	request.PageSize = "100"
	resp, err := client.DescribeInstances(request)

	if err != nil {
		cm.Sc.Logger.Error("ecs DescribeInstances错误:",
			zap.Error(err),
			zap.Any("RegionId", alic.RegionId),
			zap.Any("AccessKeyId", alic.AccessKeyId),
			zap.Any("AccessKeySecret", alic.AccessKeySecret),
		)
		return
	}
	cm.Sc.Logger.Info("ecs DescribeInstances 数量",
		zap.Int("ecs", resp.TotalCount),
		zap.Any("RegionId", alic.RegionId),
		//zap.Any("AccessKeyId", alic.AccessKeyId),
		//zap.Any("AccessKeySecret", alic.AccessKeySecret),
	)
	cloudIns := resp.Instances
	//for _, ins := range cloudIns.Instance {
	//	// 做sdk中的ecs结构转换为db中的
	//	dbIns := cm.ConvertEcsCloudAli(ins)
	//	allEcs.Store(dbIns.InstanceId, dbIns)
	//}

	for _, ins := range cloudIns.Instance {
		// 做sdk中的ecs结构转换为db中的
		dbIns := cm.ConvertEcsCloudAli(ins, eniEipMap, insDiskMap, alic.AccountName, realAccountId)

		// 【增加判空逻辑】：只有当 dbIns 不为 nil 的时候，才往 map 里存
		if dbIns != nil {
			allEcs.Store(dbIns.InstanceId, dbIns)
		} else {
			cm.Sc.Logger.Warn("ConvertEcsCloudAli 返回了 nil, 跳过该实例")
		}
	}
	return

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

// ConvertEc2CloudAws 将 AWS EC2 对象转换为本地 DB 模型
func (cm *CronManager) ConvertEc2CloudAws(ins types.Instance, diskMap map[string][]string, typeMap map[string]types.InstanceTypeInfo, accountName string, realAccountId string) *models.ResourceEcs {
	privateIpMap := make(map[string]struct{})
	publicIpMap := make(map[string]struct{})
	var enis []string
	var securityGroupIds []string

	// 1. 提取 IP 和 网卡 (AWS 会把所有关联的公网IP都放在 Association 里)
	for _, eni := range ins.NetworkInterfaces {
		enis = append(enis, aws.ToString(eni.NetworkInterfaceId))
		for _, pIp := range eni.PrivateIpAddresses {
			if pIp.PrivateIpAddress != nil {
				privateIpMap[aws.ToString(pIp.PrivateIpAddress)] = struct{}{}
			}
			// 如果这个内网IP绑定了公网/EIP
			if pIp.Association != nil && pIp.Association.PublicIp != nil {
				publicIpMap[aws.ToString(pIp.Association.PublicIp)] = struct{}{}
			}
		}
	}

	// 2. 提取安全组
	for _, sg := range ins.SecurityGroups {
		securityGroupIds = append(securityGroupIds, aws.ToString(sg.GroupId))
	}

	// 3. 提取标签，并寻找 Name
	var tags []string
	instanceName := aws.ToString(ins.InstanceId) // 默认用ID兜底
	for _, t := range ins.Tags {
		tags = append(tags, fmt.Sprintf("%s=%s", aws.ToString(t.Key), aws.ToString(t.Value)))
		if aws.ToString(t.Key) == "Name" {
			instanceName = aws.ToString(t.Value)
		}
	}

	// 4. 将 Map 转回 Slice 并排序
	var privateIps []string
	for ip := range privateIpMap {
		privateIps = append(privateIps, ip)
	}
	sort.Strings(privateIps)

	var publicIps []string
	for ip := range publicIpMap {
		publicIps = append(publicIps, ip)
	}
	sort.Strings(publicIps)
	sort.Strings(enis)
	sort.Strings(securityGroupIds)
	sort.Strings(tags)

	// 5. 提取云盘 (从传进来的字典查)
	var diskIds []string
	if disks, ok := diskMap[aws.ToString(ins.InstanceId)]; ok {
		diskIds = disks
	}
	sort.Strings(diskIds)

	// 6. 获取 CPU 和 内存 (从传进来的字典查)
	var cpu, memory int
	insTypeStr := string(ins.InstanceType)
	if typeInfo, ok := typeMap[insTypeStr]; ok {
		if typeInfo.VCpuInfo != nil && typeInfo.VCpuInfo.DefaultVCpus != nil {
			cpu = int(aws.ToInt32(typeInfo.VCpuInfo.DefaultVCpus))
		}
		if typeInfo.MemoryInfo != nil && typeInfo.MemoryInfo.SizeInMiB != nil {
			memory = int(aws.ToInt64(typeInfo.MemoryInfo.SizeInMiB))
		}
	}

	rawPlatform := aws.ToString(ins.PlatformDetails)
	osType := "linux"

	// AWS 逻辑：如果 PlatformDetails 包含 Windows 则是 windows，否则基本都是 Linux 变体
	if strings.Contains(strings.ToLower(rawPlatform), "windows") {
		osType = "windows"
	}
	// 6.5 安全的时间复制 (防止直接引用 AWS SDK 内部的指针引发不可预知问题)
	safeTime := func(t *time.Time) *time.Time {
		if t == nil {
			return nil
		}
		// 解引用取值，然后再取新地址，完美实现深拷贝
		newT := *t
		return &newT
	}

	// 7. 组装模型
	dbIns := &models.ResourceEcs{
		ResourceCommon: models.ResourceCommon{

			Vendor:      "aws", // 👉 标记为 AWS
			AccountName: fmt.Sprintf("%s(%s)", accountName, realAccountId),
			Tags:        models.StringArray(tags),
			ZoneId:      aws.ToString(ins.Placement.AvailabilityZone),
		},
		InstanceId:        aws.ToString(ins.InstanceId),
		InstanceName:      instanceName,
		InstanceType:      insTypeStr,
		HostName:          aws.ToString(ins.PrivateDnsName),
		VmType:            "1",
		VpcId:             aws.ToString(ins.VpcId),
		Status:            string(ins.State.Name), // pending | running | stopping | stopped 等
		OSType:            osType,                 // 存入 "linux"
		OSName:            rawPlatform,            // 存入 "Linux/UNIX"
		Cpu:               cpu,
		Memory:            memory,
		ImageId:           aws.ToString(ins.ImageId),
		SecurityGroupIds:  models.StringArray(securityGroupIds),
		PrivateIpAddress:  models.StringArray(privateIps),
		PublicIpAddresses: models.StringArray(publicIps),
		NetworkInterfaces: models.StringArray(enis),
		DiskIds:           models.StringArray(diskIds),
		CreationTime:      safeTime(ins.LaunchTime),
		StartTime:         safeTime(ins.LaunchTime),
	}
	if dbIns.HostName == "" {
		dbIns.HostName = aws.ToString(ins.InstanceId)
	}

	dbIns.Hash = dbIns.GenHash()
	return dbIns
}
func (cm *CronManager) RunSyncOneCloudEc2Aws(ctx context.Context, awsConf *config.AwsCloud, allEcs *sync.Map) {
	// 1. 初始化 AWS Client
	cfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithRegion(awsConf.RegionId),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awsConf.AccessKeyId, awsConf.SecretAccessKey, "")),
	)
	if err != nil {
		cm.Sc.Logger.Error("初始化 AWS EC2 Client 错误:", zap.Error(err))
		return
	}
	client := ec2.NewFromConfig(cfg)

	// 🌟 新增：获取 AWS 12位账号 ID
	stsClient := awsSts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &awsSts.GetCallerIdentityInput{})

	realAccountId := ""
	if err == nil {
		realAccountId = aws.ToString(identity.Account)
		cm.Sc.Logger.Info("成功获取AWS账号ID", zap.String("账号", awsConf.AccountName))
	}

	// ================== 【提前获取 AWS 云盘大小映射】 ==================
	insDiskMap := make(map[string][]string)
	volPaginator := ec2.NewDescribeVolumesPaginator(client, &ec2.DescribeVolumesInput{})
	for volPaginator.HasMorePages() {
		volPage, err := volPaginator.NextPage(ctx)
		if err != nil {
			cm.Sc.Logger.Error("AWS 获取 Disk 列表失败", zap.Any("账号", awsConf.AccountName), zap.Error(err))
			break
		}
		for _, vol := range volPage.Volumes {
			if len(vol.Attachments) > 0 {
				insId := aws.ToString(vol.Attachments[0].InstanceId)
				// 拼接盘ID和大小 (单位: GB)
				diskInfo := fmt.Sprintf("%s(%dG)", aws.ToString(vol.VolumeId), aws.ToInt32(vol.Size))
				insDiskMap[insId] = append(insDiskMap[insId], diskInfo)
			}
		}
	}

	// ================== 【获取实例列表】 ==================
	var rawInstances []types.Instance
	uniqueTypesMap := make(map[string]struct{}) // 记录用到了哪些实例规格

	insPaginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{})
	for insPaginator.HasMorePages() {
		insPage, err := insPaginator.NextPage(ctx)
		if err != nil {
			cm.Sc.Logger.Error("AWS DescribeInstances 错误:", zap.Any("账号", awsConf.AccountName), zap.Error(err))
			return
		}
		// AWS 的实例藏在 Reservations 里
		for _, res := range insPage.Reservations {
			for _, ins := range res.Instances {
				rawInstances = append(rawInstances, ins)
				uniqueTypesMap[string(ins.InstanceType)] = struct{}{}
			}
		}
	}
	cm.Sc.Logger.Info("AWS DescribeInstances 数量", zap.Int("ec2", len(rawInstances)), zap.String("Region", awsConf.RegionId))

	// ================== 【查询规格对应的 CPU 和内存】 ==================
	// 避免查询全量规格，只查当前拥有的规格
	typeInfoMap := make(map[string]types.InstanceTypeInfo)
	if len(uniqueTypesMap) > 0 {
		var typeQueryList []types.InstanceType
		for t := range uniqueTypesMap {
			typeQueryList = append(typeQueryList, types.InstanceType(t))
		}

		typeResp, err := client.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
			InstanceTypes: typeQueryList,
		})
		if err == nil {
			for _, tInfo := range typeResp.InstanceTypes {
				typeInfoMap[string(tInfo.InstanceType)] = tInfo
			}
		} else {
			cm.Sc.Logger.Error("AWS 获取 InstanceTypes 失败，CPU和内存将为 0", zap.Error(err))
		}
	}

	// ================== 【转换为 DB 模型并存入 Map】 ==================
	for _, ins := range rawInstances {
		dbIns := cm.ConvertEc2CloudAws(ins, insDiskMap, typeInfoMap, awsConf.AccountName, realAccountId)
		if ins.State.Name == types.InstanceStateNameTerminated {
			continue
		}
		if dbIns != nil {
			allEcs.Store(dbIns.InstanceId, dbIns)
		} else {
			cm.Sc.Logger.Warn("ConvertEcsCloudAws 返回了 nil")
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

func MockDescribeInstancesResponse(allEcs *sync.Map) {
	// 1. 种子只播一次
	rand.Seed(time.Now().UnixNano())

	randVendor := []string{"ali", "huawei", "tencent", "aws"}
	randVmType := []string{"虚拟机", "物理机", "容器"}
	randVpcId := []string{"vpc-001", "vpc-002", "vpc-003"}
	randEnv := []string{"dev", "stage", "press", "prod"}
	randOs := []string{"windows", "linux", "aix"}
	randOsName := []string{"win10", "centos", "Euler", "debian", "ubuntu"}
	randInstanceType := []string{"ecs.c9i.large", "ecs.c8a.12xlarge", "ecs.c8i.large", "ecs.c8ae.32xlarge"}
	randRegions := []string{"cn-beijing", "cn-shanghai", "cn-qingdao", "cn-guangzhou"}

	randCpus := []int{4, 8, 16, 32}
	randMems := []int{8, 16, 32, 64, 128, 256, 512}

	randTagKeys := []string{"arch", "x86", "idc", "os", "job", "env", "cluster", "type"}
	randTagValues := []string{"linux", "beijing", "debian", "ubuntu"}

	// 2. 修复闭包：每个数组用自己的长度，且移除 -1
	// 直接返回随机元素更简洁
	getStr := func(slice []string) string {
		return slice[rand.Intn(len(slice))]
	}
	getInt := func(slice []int) int {
		return slice[rand.Intn(len(slice))]
	}

	// 随机生成 10 到 30 台机器
	num := rand.Intn(20) + 10

	for i := 0; i < num; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i+1)
		// 注意：这里建议使用指针，符合你同步逻辑中的 v.(*models.ResourceEcs)
		ecsOne := &models.ResourceEcs{}

		ecsOne.InstanceName = fmt.Sprintf("Mock-Host-%v", i)
		ecsOne.HostName = ecsOne.InstanceName
		ecsOne.InstanceId = uuid.New().String()

		// ✅ 修复：全部改用对应数组的真实长度
		ecsOne.Vendor = getStr(randVendor)
		ecsOne.VmType = getStr(randVmType)
		ecsOne.InstanceType = getStr(randInstanceType)
		ecsOne.VpcId = getStr(randVpcId)
		ecsOne.OSType = getStr(randOs)
		ecsOne.OSName = getStr(randOsName)
		ecsOne.ZoneId = getStr(randRegions)
		ecsOne.Cpu = getInt(randCpus)
		ecsOne.Memory = getInt(randMems)
		ecsOne.Status = "running"
		ecsOne.Env = getStr(randEnv)

		// 3. 标签逻辑修复
		var tags []string
		// 随机选 2 个 key 组成标签
		for j := 0; j < 2; j++ {
			key := getStr(randTagKeys)
			val := getStr(randTagValues)
			tags = append(tags, fmt.Sprintf("%s=%s", key, val))
		}
		sort.Strings(tags) // 排序保证 Hash 稳定
		ecsOne.Tags = tags

		ecsOne.PrivateIpAddress = []string{ip}
		ecsOne.PublicIpAddresses = []string{ip}

		// ✅ 4. 先计算 Hash，再 Store
		ecsOne.Hash = ecsOne.GenHash()
		allEcs.Store(ecsOne.InstanceId, ecsOne)
	}
}
