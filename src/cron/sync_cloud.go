package cron

import (
	"bigdevops/src/models"
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
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

func (cm *CronManager) RunSyncCloudResource(ctx context.Context) {
	cm.Sc.Logger.Info("同步公有云资源中....")
	//go cm.RunSyncCloudResourceEcs(ctx)
	//go cm.RunSyncCloudResourceElb(ctx)
	//go cm.RunSyncCloudResourceRds(ctx)
	//go cm.RunSyncCloudResourceDns(ctx)

	// 1. 使用 WaitGroup 等待所有的“底层基础设施”同步完
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		cm.RunSyncCloudResourceEcs(ctx)
	}()
	go func() {
		defer wg.Done()
		cm.RunSyncCloudResourceElb(ctx)
	}()
	go func() {
		defer wg.Done()
		cm.RunSyncCloudResourceRds(ctx)
	}()

	// 阻塞等待：不等到这三个底层资源（ECS、ELB、RDS）落盘，绝不往下走
	wg.Wait()

	cm.Sc.Logger.Info("底层基础设施 (ECS/ELB/RDS) 同步完毕，开始同步上层应用层资源 (DNS)...")

	// 2. 此时数据库里已经有最新的 ECS 和 ELB IP 了，再跑 DNS，绝对能 100% 匹配上！
	cm.RunSyncCloudResourceDns(ctx)
}

func MockDescribeInstancesResponse(allEcs *sync.Map) {
	// 1. 种子只播一次
	rand.Seed(time.Now().UnixNano())

	randVendor := []string{"ali", "huawei", "tencent", "aws"}
	randVmType := []int{1, 2, 3}
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
		ecsOne.VmType = getInt(randVmType)
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
