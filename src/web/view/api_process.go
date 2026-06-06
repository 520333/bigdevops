package view

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

func createProcess(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.Process
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增流程请求失败", zap.Any("流程", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqObj)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		sc.Logger.Error("通过token解析到的userName去数据库中找User", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("通过token解析到的userName去数据库中找User失败 %v", err.Error()), c)
		return
	}
	reqObj.UserID = dbUser.ID
	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增流程数据库失败", zap.Any("流程", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
	}
	common.OkWithMessage("创建成功", c)
}

func getProcessList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	// 数据库中拿到所有的menu列表
	objs, err := models.GetProcessAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的流程错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的流程错误：%v", err.Error()), c)
		return
	}

	for _, obj := range objs {
		obj := obj
		obj.FillFrontAllData()
	}

	common.OkWithDetailed(objs, "ok", c)
}

//	func updateProcess(c *gin.Context) {
//		sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
//		var reqObj models.Process
//		err := c.ShouldBindJSON(&reqObj)
//		if err != nil {
//			sc.Logger.Error("解析新增流程请求失败", zap.Any("流程", reqObj), zap.Error(err))
//			common.FailWithMessage(err.Error(), c)
//			return
//		}
//
//		err = validate.Struct(&reqObj)
//		if err != nil {
//			if errors, ok := err.(validator.ValidationErrors); ok {
//				common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
//				return
//			}
//		}
//
//		dbObj, err := models.GetProcessById(int(reqObj.ID))
//		if err != nil {
//			sc.Logger.Error("根据id找流程错误", zap.Any("流程", reqObj), zap.Error(err))
//			common.FailWithMessage(err.Error(), c)
//			return
//		}
//
//		err = reqObj.UpdateOne()
//		if err != nil {
//			sc.Logger.Error("更新流程错误", zap.Any("流程", reqObj), zap.Error(err))
//			common.FailWithMessage(err.Error(), c)
//			return
//		}
//
//		err = reqObj.UpdateFlowNodes(reqObj.FlowNodes)
//		if err != nil {
//			sc.Logger.Error("更新流程审批节点错误", zap.Any("审批节点", dbObj), zap.Error(err))
//			common.FailWithMessage(err.Error(), c)
//			return
//		}
//
//		common.OkWithMessage("更新成功", c)
//	}
func updateProcess(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.Process
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新流程请求失败", zap.Any("流程", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqObj)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	// 1. 先查出数据库里原始的、完整的 Process（包含 UserID 等重要字段）
	dbProcess, err := models.GetProcessById(int(reqObj.ID))
	if err != nil {
		sc.Logger.Error("根据id找流程错误", zap.Any("流程", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 2. 覆盖前端允许修改的字段（重点！）
	dbProcess.Name = reqObj.Name
	dbProcess.FlowNodes = reqObj.FlowNodes // 把前端传来的新节点列表全量覆盖上去

	// 3. 执行真正的全量级联更新
	err = dbProcess.UpdateWithNodes()
	if err != nil {
		sc.Logger.Error("更新流程及审批节点错误", zap.Any("流程", dbProcess), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}
func setProcessStatus(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqRole models.SetRoleStatusReq
	err := c.ShouldBindJSON(&reqRole)
	if err != nil {
		sc.Logger.Error("解析新增流程请求失败", zap.Any("流程", reqRole), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqRole)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	dbRole, err := models.GetRoleById(reqRole.Id)
	if err != nil {
		sc.Logger.Error("根据id找流程错误", zap.Any("流程", reqRole), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbRole.Status = reqRole.Status

	// 更新
	err = dbRole.UpdateMenus(dbRole.Menus)
	if err != nil {
		sc.Logger.Error("跟新流程和关联菜单错误", zap.Any("流程", reqRole), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("创建成功", c)
}

func deleteProcess(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("删除流程", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbRole, err := models.GetProcessById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找流程错误", zap.Any("流程", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	err = dbRole.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除流程错误", zap.Any("流程", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("删除成功", c)
}
