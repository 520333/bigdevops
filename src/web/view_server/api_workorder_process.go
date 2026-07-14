package view_server

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
	var reqObj models.WorkOrderProcess
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

	// 1. 获取分页参数和查询参数
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	name := c.Query("name") // 获取前端传来的流程名称
	searchCreateUserName := c.Query("createUserName")
	offset := (currentPage - 1) * pageSize

	// 2. 使用带名称过滤的统计方法
	total, err := models.GetProcessCountByNameAndCreator(name, searchCreateUserName)
	if err != nil {
		sc.Logger.Error("查询流程总数失败", zap.Error(err))
		common.ReqBadFailWithMessage("查询失败", c)
		return
	}

	if total == 0 {
		common.OkWithDetailed(gin.H{"items": []*models.WorkOrderProcess{}, "total": 0}, "ok", c)
		return
	}

	// 3. 使用带名称过滤的分页方法
	pagedObjs, err := models.GetProcessListByNameAndCreator(name, searchCreateUserName, pageSize, offset)
	if err != nil {
		sc.Logger.Error("分页获取流程数据失败", zap.Error(err))
		common.ReqBadFailWithMessage("查询失败", c)
		return
	}

	for _, obj := range pagedObjs {
		obj.FillFrontAllData()
	}

	common.OkWithDetailed(gin.H{"items": pagedObjs, "total": total}, "ok", c)
}

func updateProcess(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.WorkOrderProcess
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
	dbTemplate, err := models.GetWorkOrderTemplateByProcessId(intVar)

	// 1. 如果有 err 并且不是“未找到记录”的错误，说明数据库崩了
	if err != nil && err.Error() != "WorkOrderTemplate不存在" { // 这里的字符串取决于你 Get 方法里的定义
		sc.Logger.Error("检查流程关联模板时发生数据库错误", zap.Error(err))
		common.FailWithMessage("检查模板关联失败", c)
		return
	}

	// 2. 如果成功查到了模板，说明被占用了，明确拒绝并返回自定义提示（绝对不能用 err.Error()）
	if dbTemplate != nil && dbTemplate.ID > 0 {
		errMsg := fmt.Sprintf("该审批流程已被工单模板【%s】绑定，禁止直接删除！", dbTemplate.Name)
		sc.Logger.Warn(errMsg, zap.Any("流程ID", id))
		common.FailWithMessage(errMsg, c)
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
