package cron

import (
	"bigdevops/src/config"
	"bigdevops/src/models"
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/rds" // ⚠️ 必须导入阿里云真实的 RDS 包
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awsRds "github.com/aws/aws-sdk-go-v2/service/rds" // ⚠️ 必须导入 AWS 真实的 RDS 包
	awsRdsTypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	awsSts "github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/gammazero/workerpool"
	"go.uber.org/zap"
)

func (cm *CronManager) RunSyncCloudResourceRds(ctx context.Context) {
	start := time.Now()

	if !cm.GetRdsSynced() {
		cm.Sc.Logger.Info("RDS上次同步还未完成，本次cron跳过")
		return
	}
	cm.SetRdsSynced(false)

	dbUidHashM, err := models.GetResourceRdsUidAndHash()
	if err != nil {
		cm.Sc.Logger.Error("获取本地RDS Hash失败", zap.Error(err))
	}

	allRds := &sync.Map{}
	wp := workerpool.New(5)

	// 1. 提交阿里云同步任务
	for _, alic := range cm.Sc.PublicCloudSyncC.AliCloud {
		if !alic.Enable {
			cm.Sc.Logger.Info("阿里云同步任务已禁用，跳过同步", zap.String("region", alic.RegionId))
			continue
		}
		alic := alic
		wp.Submit(func() {
			cm.RunSyncOneCloudRdsAli(alic, allRds)
		})
	}

	// 2. 提交 AWS 同步任务
	for _, awsc := range cm.Sc.PublicCloudSyncC.AwsCloud {
		if !awsc.Enable {
			cm.Sc.Logger.Info("AWS 账号配置已禁用，跳过同步", zap.String("Region", awsc.RegionId))
			continue
		}
		awsc := awsc
		wp.Submit(func() {
			cm.RunSyncOneCloudRdsAws(ctx, awsc, allRds)
		})
	}

	wp.StopWait() // 等待所有抓取完毕

	// ================== 开始增量比对 ==================
	toAddSet := make([]*models.ResourceRds, 0)
	toModSet := make([]*models.ResourceRds, 0)
	var toDelUids []string
	localUidSet := make(map[string]struct{})
	var toAddNum, toModNum, toDelNum, suAddNum, suModNum, suDelNum int

	allRds.Range(func(k, v interface{}) bool {
		uid := k.(string)
		RdsObj := v.(*models.ResourceRds)
		localUidSet[uid] = struct{}{}

		dbHash, ok := dbUidHashM[uid]
		if !ok {
			toAddSet = append(toAddSet, RdsObj)
			toAddNum++
		} else {
			if dbHash != RdsObj.Hash {
				toModSet = append(toModSet, RdsObj)
				toModNum++
			}
		}
		return true
	})

	for uid := range dbUidHashM {
		if _, ok := localUidSet[uid]; !ok {
			toDelUids = append(toDelUids, uid)
			toDelNum++
		}
	}

	// ================== 执行 DB 操作 ==================
	for _, obj := range toAddSet {
		if err := obj.CreateOne(); err == nil {
			suAddNum++
		} else {
			cm.Sc.Logger.Error("新增RDS错误", zap.Error(err), zap.String("id", obj.DBInstanceId))
		}
	}

	for _, obj := range toModSet {
		if err := obj.UpdateOne(); err == nil {
			suModNum++
		} else {
			cm.Sc.Logger.Error("更新RDS错误", zap.Error(err), zap.String("id", obj.DBInstanceId))
		}
	}

	for _, uid := range toDelUids {
		dbObj, err := models.GetResourceRdsByInstanceId(uid) // 需在models实现该方法
		if err == nil && dbObj != nil {
			if err = dbObj.DeleteOne(); err == nil {
				suDelNum++
			} else {
				cm.Sc.Logger.Error("删除RDS错误", zap.Error(err), zap.String("uid", uid))
			}
		}
	}

	//cm.Sc.Logger.Info("同步RDS结果打印",
	//	zap.Int("公有云总数", len(localUidSet)),
	//	zap.Int("本地数据库", len(dbUidHashM)),
	//	zap.Int("新增尝试", toAddNum), zap.Int("新增成功", suAddNum),
	//	zap.Int("更新尝试", toModNum), zap.Int("更新成功", suModNum),
	//	zap.Int("删除尝试", toDelNum), zap.Int("删除成功", suDelNum),
	//	zap.Float64("耗时(s)", time.Since(start).Seconds()),
	//)
	cm.Sc.Logger.Info("同步RDS结果打印",
		zap.Any("公有云总数", len(localUidSet)),
		zap.Any("本地数据库", len(dbUidHashM)),
		zap.Any("toAddNum", toAddNum),
		zap.Any("toModNum", toModNum),
		zap.Any("toDelNum", toDelNum),
		zap.Any("suAddNum", suAddNum),
		zap.Any("suModNum", suModNum),
		zap.Any("suDelNum", suDelNum),
		zap.Any("timeTook", time.Since(start).Seconds()),
	)
	cm.SetRdsSynced(true)
}

// ---------------- 阿里云解析与抓取 ----------------

func (cm *CronManager) ConvertRdsCloudAli(db rds.DBInstance, account string, realAccount string, host string, port int) *models.ResourceRds {
	dbRds := &models.ResourceRds{
		ResourceCommon: models.ResourceCommon{
			Vendor:      "aliyun",
			AccountName: fmt.Sprintf("%s(%s)", account, realAccount),
			ZoneId:      db.ZoneId,
		},
		DBInstanceId:      db.DBInstanceId,
		Name:              db.DBInstanceDescription,
		Engine:            db.Engine,
		EngineVersion:     db.EngineVersion,
		DBInstanceClass:   db.DBInstanceClass,
		DBInstanceNetType: db.InstanceNetworkType,
		DBInstanceType:    db.DBInstanceType,
		DBInstanceStatus:  db.DBInstanceStatus,
		PayType:           db.PayType,
		VpcId:             db.VpcId,
		Host:              host, // 💡 从参数直接接收
		Port:              port, // 💡 从参数直接接收
	}

	if t, err := time.Parse(time.RFC3339, db.CreateTime); err == nil {
		dbRds.CreationTime = &t
	}
	if db.ExpireTime != "" {
		if t, err := time.Parse(time.RFC3339, db.ExpireTime); err == nil {
			dbRds.ExpiredTime = &t
		}
	}

	dbRds.Hash = dbRds.GenHash()
	return dbRds
}

func (cm *CronManager) RunSyncOneCloudRdsAli(alic *config.AliCloud, allRds *sync.Map) {
	cm.Sc.Logger.Info("RDS 同步阿里云开始", zap.String("region", alic.RegionId))

	// 获取真实 AccountID
	stsClient, _ := sts.NewClientWithAccessKey(alic.RegionId, alic.AccessKeyId, alic.AccessKeySecret)
	stsReq := sts.CreateGetCallerIdentityRequest()
	stsReq.Scheme = "https"
	realAccountId := ""
	if stsResp, err := stsClient.GetCallerIdentity(stsReq); err == nil {
		realAccountId = stsResp.AccountId
	}

	if rdsClient, err := rds.NewClientWithAccessKey(alic.RegionId, alic.AccessKeyId, alic.AccessKeySecret); err == nil {
		req := rds.CreateDescribeDBInstancesRequest()
		req.Scheme = "https"
		req.PageSize = requests.NewInteger(100)

		if resp, err := rdsClient.DescribeDBInstances(req); err == nil {
			for _, item := range resp.Items.DBInstance {

				// 💡 新增逻辑：单独查询该实例的连接地址和端口
				var host string
				var port int

				netReq := rds.CreateDescribeDBInstanceNetInfoRequest()
				netReq.Scheme = "https"
				netReq.DBInstanceId = item.DBInstanceId

				if netResp, err := rdsClient.DescribeDBInstanceNetInfo(netReq); err == nil {
					for _, netInfo := range netResp.DBInstanceNetInfos.DBInstanceNetInfo {
						// 通常CMDB最需要的是内网通信地址 (VPC内网为 Intranet，经典网络为 Private)
						if netInfo.IPType == "Intranet" || netInfo.IPType == "Private" {
							host = netInfo.ConnectionString
							port, _ = strconv.Atoi(netInfo.Port) // 依然需要 strconv 包
							break
						}
					}
					// 兜底逻辑：如果上面的 if 没命中，拿列表里的第一个地址凑合用
					if host == "" && len(netResp.DBInstanceNetInfos.DBInstanceNetInfo) > 0 {
						fallback := netResp.DBInstanceNetInfos.DBInstanceNetInfo[0]
						host = fallback.ConnectionString
						port, _ = strconv.Atoi(fallback.Port)
					}
				} else {
					cm.Sc.Logger.Warn("获取阿里云RDS网络信息失败", zap.String("id", item.DBInstanceId), zap.Error(err))
				}

				// 将解析到的 host 和 port 传入转换函数
				dbRds := cm.ConvertRdsCloudAli(item, alic.AccountName, realAccountId, host, port)
				allRds.Store(dbRds.DBInstanceId, dbRds)
			}
		} else {
			cm.Sc.Logger.Error("查询阿里云 RDS 失败", zap.Error(err), zap.String("region", alic.RegionId))
		}
	}
}

// ---------------- AWS 解析与抓取 ----------------

func (cm *CronManager) ConvertRdsCloudAws(db awsRdsTypes.DBInstance, account string, realAccount string) *models.ResourceRds {
	// 💡 提取 AWS 的名称逻辑
	instanceId := aws.ToString(db.DBInstanceIdentifier)
	instanceName := instanceId // 默认用 ID 当名称兜底

	// 遍历 AWS 的 Tags，寻找 Key 为 "Name" 的标签
	for _, tag := range db.TagList {
		if aws.ToString(tag.Key) == "Name" || aws.ToString(tag.Key) == "name" {
			instanceName = aws.ToString(tag.Value)
			break
		}
	}
	dbRds := &models.ResourceRds{
		ResourceCommon: models.ResourceCommon{
			Vendor:      "aws",
			AccountName: fmt.Sprintf("%s(%s)", account, realAccount),
			ZoneId:      aws.ToString(db.AvailabilityZone),
		},
		DBInstanceId:     aws.ToString(db.DBInstanceIdentifier), // AWS 通常用 Identifier 作为唯一 ID
		Name:             instanceName,
		Engine:           aws.ToString(db.Engine),
		EngineVersion:    aws.ToString(db.EngineVersion),
		DBInstanceClass:  aws.ToString(db.DBInstanceClass),
		DBInstanceStatus: aws.ToString(db.DBInstanceStatus), // available, backing-up 等
		CreationTime:     db.InstanceCreateTime,
	}

	// 提取 VPC ID (AWS 的 RDS VPC 信息嵌在 DBSubnetGroup 里)
	if db.DBSubnetGroup != nil {
		dbRds.VpcId = aws.ToString(db.DBSubnetGroup.VpcId)
	}
	// 💡 提取 AWS 的连接地址和端口
	if db.Endpoint != nil {
		dbRds.Host = aws.ToString(db.Endpoint.Address)
		// ✅ 修复：先用 aws.ToInt32 安全解引用指针，再转为标准 int
		dbRds.Port = int(aws.ToInt32(db.Endpoint.Port))
	}

	// AWS 没有明确的 PayType 字段直接在 Instance 上返回，通常需要查计费 API，这里可留空或默认按量
	dbRds.PayType = "Postpaid"

	dbRds.Hash = dbRds.GenHash()
	return dbRds
}

func (cm *CronManager) RunSyncOneCloudRdsAws(ctx context.Context, awsConf *config.AwsCloud, allRds *sync.Map) {
	cm.Sc.Logger.Info("RDS 同步AWS开始", zap.String("region", awsConf.RegionId))

	cfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithRegion(awsConf.RegionId),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awsConf.AccessKeyId, awsConf.SecretAccessKey, "")),
	)
	if err != nil {
		cm.Sc.Logger.Error("AWS Config失败", zap.Error(err))
		return
	}

	stsClient := awsSts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &awsSts.GetCallerIdentityInput{})
	realAccountId := ""
	if err == nil {
		realAccountId = aws.ToString(identity.Account)
	}

	// 真正的 AWS RDS 查询逻辑
	client := awsRds.NewFromConfig(cfg)
	paginator := awsRds.NewDescribeDBInstancesPaginator(client, &awsRds.DescribeDBInstancesInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			cm.Sc.Logger.Error("AWS RDS 查询失败", zap.Error(err))
			break
		}
		for _, dbInst := range page.DBInstances {
			dbRds := cm.ConvertRdsCloudAws(dbInst, awsConf.AccountName, realAccountId)
			allRds.Store(dbRds.DBInstanceId, dbRds)
		}
	}
}
