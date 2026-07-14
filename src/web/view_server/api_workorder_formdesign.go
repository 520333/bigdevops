package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

func getFormDesignList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")
	searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchName := c.DefaultQuery("name", "")
	searchCreateUserName := c.DefaultQuery("createUserName", "") // 👈 1. 接收前端传来的创建人参数

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	// 数据库中拿到所有的formDesign列表
	objs, err := models.GetFormDesignAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的表单设计错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的表单设计错误：%v", err.Error()), c)
		return
	}
	allIds := []int{}

	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}

		if searchName != "" && !strings.Contains(obj.Name, searchName) {
			continue
		}

		// 👈 2. 先填充前端需要的数据（拿到组合好的 CreateUserName）
		obj.FillFrontAllData()

		// 👈 3. 对创建人进行模糊匹配过滤
		if searchCreateUserName != "" && !strings.Contains(obj.CreateUserName, searchCreateUserName) {
			continue
		}

		allIds = append(allIds, int(obj.ID))
	}

	// 如果过滤后没有数据，直接返回空列表
	if len(allIds) == 0 {
		common.OkWithDetailed(gin.H{
			"items": []models.WorkOrderFormDesign{},
			"total": 0,
		}, "ok", c)
		return
	}

	// 根据过滤后的 ID 进行分页查询
	pagedObjs, err := models.GetFormDesignByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的表单设计错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的表单设计错误：%v", err.Error()), c)
		return
	}

	for _, obj := range pagedObjs {
		obj.FillFrontAllData()
	}

	common.OkWithDetailed(gin.H{
		"items": pagedObjs,
		"total": len(allIds),
	}, "ok", c)
}

func createFormDesign(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.WorkOrderFormDesign
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增表单设计请求失败", zap.Any("表单设计", reqObj), zap.Error(err))
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
		sc.Logger.Error("新增表单设计数据库失败", zap.Any("表单设计", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
	}
	common.OkWithMessage("创建成功", c)
}

func updateFormDesign(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.WorkOrderFormDesign
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增表单设计请求失败", zap.Any("表单设计", reqObj), zap.Error(err))
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

	_, err = models.GetFormDesignById(int(reqObj.ID))
	if err != nil {
		sc.Logger.Error("根据id找表单设计错误", zap.Any("表单设计", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新表单设计错误", zap.Any("表单设计", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

func deleteFormDesign(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("删除表单设计", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbRole, err := models.GetFormDesignById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找表单设计错误", zap.Any("表单设计", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbTemplate, err := models.GetWorkOrderTemplateByFormDesignId(intVar)

	// 1. 如果有 err 并且不是“未找到记录”的错误，说明数据库查询出错了
	if err != nil && err.Error() != "WorkOrderTemplate不存在" { // 这里的字符串取决于你 Get 方法里的定义
		sc.Logger.Error("检查表单关联模板时发生数据库错误", zap.Error(err))
		common.FailWithMessage("检查模板关联失败", c)
		return
	}

	// 2. 如果成功查到了模板，说明被占用了，明确拒绝并返回自定义提示
	if dbTemplate != nil && dbTemplate.ID > 0 {
		errMsg := fmt.Sprintf("该表单设计已被工单模板【%s】绑定，禁止直接删除！", dbTemplate.Name)
		sc.Logger.Warn(errMsg, zap.Any("表单ID", id))
		common.FailWithMessage(errMsg, c)
		return
	}
	err = dbRole.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除表单设计错误", zap.Any("表单设计", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("删除成功", c)
}
