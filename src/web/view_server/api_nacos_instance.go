package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/models"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// getNacosInstanceList 获取 Nacos 实例列表
func getNacosInstanceList(c *gin.Context) {
	var param models.NacosInstanceQueryParam
	_ = c.ShouldBindQuery(&param)

	list, total, err := models.GetNacosInstanceList(&param)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("查询 Nacos 实例失败: %v", err), c)
		return
	}

	common.OkWithDetailed(gin.H{
		"items": list,
		"total": total,
	}, "获取 Nacos 实例列表成功", c)
}

// getNacosInstanceDetail 获取单个 Nacos 实例详情 (参考 Jenkins 实例实现，回显真实密码供编辑查看与修改)
func getNacosInstanceDetail(c *gin.Context) {
	idStr := c.Query("id")
	id, _ := strconv.Atoi(idStr)
	if id <= 0 {
		common.ReqBadFailWithMessage("缺少有效实例 ID", c)
		return
	}

	dbObj, err := models.GetNacosInstanceById(uint(id))
	if err != nil || dbObj == nil {
		common.ReqBadFailWithMessage("实例不存在", c)
		return
	}

	common.OkWithDetailed(gin.H{
		"id":              dbObj.ID,
		"name":            dbObj.Name,
		"envKey":          dbObj.EnvKey,
		"serverAddr":      dbObj.ServerAddr,
		"port":            dbObj.Port,
		"namespaceId":     dbObj.NamespaceId,
		"username":        dbObj.Username,
		"password":        dbObj.Password, // 仅在编辑单条实例时回显真实密码，供管理员查看和修改
		"status":          dbObj.Status,
		"lastProbSuccess": dbObj.LastProbSuccess,
		"lastTestAt":      dbObj.LastTestAt,
		"remark":          dbObj.Remark,
	}, "获取成功", c)
}

// createNacosInstance 创建 Nacos 实例
func createNacosInstance(c *gin.Context) {
	var obj models.NacosInstance
	if err := c.ShouldBindJSON(&obj); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("参数错误: %v", err), c)
		return
	}

	if obj.Name == "" || obj.ServerAddr == "" {
		common.ReqBadFailWithMessage("实例名称和服务地址不可为空", c)
		return
	}

	if obj.Port == 0 {
		obj.Port = 8848
	}

	if obj.Password == "" && obj.ReqPassword != "" {
		obj.Password = obj.ReqPassword
	}

	nowStr := time.Now().Format("2006-01-02 15:04:05")
	obj.Status = "untested"
	obj.LastTestAt = nowStr

	if err := obj.CreateOne(); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("创建 Nacos 实例失败: %v", err), c)
		return
	}

	// 异步尝试测试连通性并回写状态
	go testAndUpdateNacosStatus(obj.ID, &common.NacosClientOptions{
		ServerAddr:  obj.ServerAddr,
		Port:        obj.Port,
		NamespaceId: obj.NamespaceId,
		Username:    obj.Username,
		Password:    obj.Password,
	})

	common.OkWithDetailed(obj, "Nacos 实例创建成功！", c)
}

// updateNacosInstance 更新 Nacos 实例
func updateNacosInstance(c *gin.Context) {
	var obj models.NacosInstance
	if err := c.ShouldBindJSON(&obj); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("参数错误: %v", err), c)
		return
	}

	if obj.ID == 0 {
		idStr := c.Query("id")
		if id, _ := strconv.Atoi(idStr); id > 0 {
			obj.ID = uint(id)
		}
	}

	if obj.ID == 0 {
		common.ReqBadFailWithMessage("缺少有效实例 ID", c)
		return
	}

	if obj.Port == 0 {
		obj.Port = 8848
	}

	// 参考 Jenkins 实例安全保护：若未修改密码（保持掩码 ****** 或为空），则保留数据库中原有密码
	if obj.ReqPassword != "" && obj.ReqPassword != "******" {
		obj.Password = obj.ReqPassword
	} else if obj.Password == "******" || obj.Password == "" {
		oldInst, err := models.GetNacosInstanceById(obj.ID)
		if err == nil && oldInst != nil {
			obj.Password = oldInst.Password
		}
	}

	if err := obj.UpdateOne(); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("更新失败: %v", err), c)
		return
	}

	// 异步更新连通性状态
	go testAndUpdateNacosStatus(obj.ID, &common.NacosClientOptions{
		ServerAddr:  obj.ServerAddr,
		Port:        obj.Port,
		NamespaceId: obj.NamespaceId,
		Username:    obj.Username,
		Password:    obj.Password,
	})

	common.OkWithDetailed(obj, "Nacos 实例更新成功！", c)
}

// deleteNacosInstance 删除 Nacos 实例
func deleteNacosInstance(c *gin.Context) {
	idStr := c.Query("id")
	id, _ := strconv.Atoi(idStr)
	if id == 0 {
		common.ReqBadFailWithMessage("缺少有效实例 ID", c)
		return
	}

	if err := models.DeleteNacosInstance(uint(id)); err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("删除失败: %v", err), c)
		return
	}

	common.OkWithMessage("Nacos 实例删除成功！", c)
}

// testNacosInstanceConnection 在线一键测试实例连通性
func testNacosInstanceConnection(c *gin.Context) {
	var req struct {
		ID uint `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage("缺少实例 ID", c)
		return
	}

	item, err := models.GetNacosInstanceById(req.ID)
	if err != nil || item == nil {
		common.ReqBadFailWithMessage("实例不存在", c)
		return
	}

	opt := &common.NacosClientOptions{
		ServerAddr:  item.ServerAddr,
		Port:        item.Port,
		NamespaceId: item.NamespaceId,
		Username:    item.Username,
		Password:    item.Password,
	}

	nowStr := time.Now().Format("2006-01-02 15:04:05")
	if testErr := common.TestNacosConnection(opt); testErr != nil {
		item.Status = "offline"
		item.LastTestAt = nowStr
		_ = models.Db.Model(item).Updates(map[string]interface{}{
			"status":       "offline",
			"last_test_at": nowStr,
		})
		common.ReqBadFailWithMessage(fmt.Sprintf("连通失败: %v", testErr), c)
		return
	}

	_ = models.Db.Model(item).Updates(map[string]interface{}{
		"status":       "online",
		"last_test_at": nowStr,
	})

	common.OkWithMessage(fmt.Sprintf("连通成功！与 Nacos [%s:%d] 握手正常", item.ServerAddr, item.Port), c)
}

// 辅助函数：后台静默测试并更新状态
func testAndUpdateNacosStatus(id uint, opt *common.NacosClientOptions) {
	status := "offline"
	if err := common.TestNacosConnection(opt); err == nil {
		status = "online"
	}
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	models.Db.Model(&models.NacosInstance{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       status,
		"last_test_at": nowStr,
	})
}
