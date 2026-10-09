package rpc

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"bigdevops/src/pbms"
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type InfoReportServer struct {
	pbms.InfoReporterServer
	SC *config.ServerConfig
}

func (s *InfoReportServer) AgentInfoReport(ctx context.Context, in *pbms.AgentInfoReportRequest) (resp *pbms.AgentInfoReportResponse, err error) {
	s.SC.Logger.Info("收到信息上报请求",
		zap.String("hostname", in.GetHostname()),
		zap.String("ip", in.GetIp()),
		zap.String("osType", in.GetOsType()),
		zap.String("osName", in.GetOsName()),
		zap.String("sn", in.GetSn()),
		zap.Int32("cpu", in.GetCpu()),
		zap.Int32("mem", in.GetMem()),
		zap.Int32("disk", in.GetDisk()),
	)

	resp = &pbms.AgentInfoReportResponse{}
	instanceId := in.GetSn() // 这是 Agent 底层的真实 UUID
	agentIp := in.GetIp()    // 这是 Agent 获取的真实内网 IP
	if instanceId == "" {
		resp.Status = "failed"
		resp.Msg = "缺乏SN号"
		return resp, nil
	}

	// 1. 查询数据库，明确它是新机器还是老机器
	dbEcs, err := models.GetResourceEcsBySnOrIP(instanceId, agentIp)
	isNewRecord := false

	if err != nil {
		if err.Error() == common.ERR_ECS_NOT_FOUND || err.Error() == "record not found" {
			isNewRecord = true
			dbEcs = &models.ResourceEcs{} // 新机器，准备一个空模板
		} else {
			// 真正的数据库故障
			s.SC.Logger.Error("查询数据库实例信息出错", zap.Error(err), zap.String("sn", instanceId))
			resp.Status = "failed"
			resp.Msg = fmt.Sprintf("查询数据库实例信息出错：%s", err.Error())
			return resp, nil
		}
	}

	s.SC.Logger.Info("查询数据库实例信息结果",
		zap.String("hostname", in.GetHostname()),
		zap.Bool("是否为新设备", isNewRecord))

	// 2. 核心数据组装与多源资产融合策略
	now := time.Now()

	// 🚀 特殊安全保护：如果该机器原本是云厂商同步的主机 (Vendor 存在且不是 "self"，如 aws, aliyun, tencent 等)
	if !isNewRecord && dbEcs.Vendor != "" && dbEcs.Vendor != "self" {
		s.SC.Logger.Info("检测到已存在云厂商主机资产，执行安全增量合并(保护云厂商元数据不被覆盖)",
			zap.String("vendor", dbEcs.Vendor),
			zap.String("cloudInstanceId", dbEcs.InstanceId),
			zap.String("ip", agentIp),
		)

		// 仅更新 Agent 采集的动态运行状态、心跳与实时硬件指标，严格保护云平台的 Vendor、InstanceId、VmType 等元数据
		updateFields := map[string]interface{}{
			"last_heartbeat_time": &now,
			"status":              "Running",
			"cpu":                 int(in.GetCpu()),
			"memory":              int(in.GetMem()),
			"os_type":             in.GetOsType(),
			"os_name":             in.GetOsName(),
		}
		if in.GetDisk() > 0 {
			updateFields["disk_ids"] = models.StringArray{fmt.Sprintf("%d", in.GetDisk())}
		}

		// 依据数据库自增主键 ID 安全更新，彻底杜绝 instance_id 差异导致的错位
		err = models.Db.Model(&models.ResourceEcs{}).Where("id = ?", dbEcs.ID).Updates(updateFields).Error
		if err != nil {
			s.SC.Logger.Error("更新云厂商机器心跳与指标失败", zap.Error(err), zap.Uint("id", dbEcs.ID))
			resp.Status = "failed"
			resp.Msg = fmt.Sprintf("更新记录失败: %s", err.Error())
			return resp, nil
		}

		resp.Status = "success"
		resp.Msg = fmt.Sprintf("云厂商资产(%s)心跳与监控指标合并成功", dbEcs.Vendor)
		return resp, nil
	}

	// 普通自建设备 (self) 或全新设备的数据组装
	dbEcs.InstanceId = instanceId
	dbEcs.InstanceName = in.GetHostname()
	dbEcs.HostName = in.GetHostname()
	dbEcs.OSType = in.GetOsType()
	dbEcs.OSName = in.GetOsName()
	dbEcs.Env = in.GetEnv()
	dbEcs.VmType = 2
	dbEcs.Vendor = "self"
	dbEcs.Cpu = int(in.GetCpu())
	dbEcs.Memory = int(in.GetMem())
	dbEcs.LastHeartbeatTime = &now
	dbEcs.Status = "Running"

	// 正确转换 int32 为 string
	diskStr := fmt.Sprintf("%d", in.GetDisk())

	// 初始化切片
	dbEcs.DiskIds = models.StringArray{diskStr}
	dbEcs.PrivateIpAddress = models.StringArray{in.GetIp()}

	if isNewRecord {
		dbEcs.SecurityGroupIds = models.StringArray{}
		dbEcs.PublicIpAddresses = models.StringArray{}
		dbEcs.NetworkInterfaces = models.StringArray{}
	}

	// 生成最新哈希值
	dbEcs.Hash = dbEcs.GenHash()

	// 3. 执行入库：新增 or 更新
	if isNewRecord {
		err = dbEcs.CreateOne()
		if err != nil {
			s.SC.Logger.Error("新增实例入库失败", zap.Error(err))
			resp.Status = "failed"
			resp.Msg = fmt.Sprintf("新增记录失败: %s", err.Error())
			return resp, nil
		}
		resp.Status = "success"
		resp.Msg = "新增机器注册成功"
	} else {
		err = dbEcs.UpdateOne()
		if err != nil {
			s.SC.Logger.Error("更新实例信息失败", zap.Error(err))
			resp.Status = "failed"
			resp.Msg = fmt.Sprintf("更新记录失败: %s", err.Error())
			return resp, nil
		}
		resp.Status = "success"
		resp.Msg = "机器信息合并/更新成功"
	}

	return resp, nil
}
