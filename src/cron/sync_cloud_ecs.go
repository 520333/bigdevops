package cron

import (
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsSts "github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gammazero/workerpool"
	"go.uber.org/zap"
)

func (cm *CronManager) RunSyncCloudResourceEcs(ctx context.Context) {
	start := time.Now()
	// 首先应该判断上次的结果
	if !cm.GetEcsSynced() {
		cm.Sc.Logger.Info("ECS上次同步还未完成，本次cron跳过")
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
	cm.Sc.Logger.Info("同步ECS结果打印",
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
		VmType:       1,
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
	cm.Sc.Logger.Info("ECS 同步阿里云开始",
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
		cm.Sc.Logger.Error("ECS DescribeInstances错误:",
			zap.Error(err),
			zap.Any("RegionId", alic.RegionId),
			zap.Any("AccessKeyId", alic.AccessKeyId),
			zap.Any("AccessKeySecret", alic.AccessKeySecret),
		)
		return
	}
	cm.Sc.Logger.Info("ECS DescribeInstances 数量",
		zap.Int("ECS", resp.TotalCount),
		zap.Any("RegionId", alic.RegionId),
		zap.String("accountId", realAccountId),
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
		VmType:            1,
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
	cm.Sc.Logger.Info("EC2 同步AWS开始",
		zap.Any("地区", awsConf.RegionId),
		zap.Any("账号", awsConf.AccountName),
	)
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
			cm.Sc.Logger.Error("EC2 DescribeInstances 错误:", zap.Any("账号", awsConf.AccountName), zap.Error(err))
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
	cm.Sc.Logger.Info("EC2 DescribeInstances 数量", zap.Int("EC2", len(rawInstances)), zap.String("Region", awsConf.RegionId), zap.String("account", awsConf.AccountName))

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
