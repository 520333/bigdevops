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

type CommonSelectResponse struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// @Summary      获取作业脚本列表
// @Description  获取作业脚本列表 接口
// @Tags         jobexec-script
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取作业脚本列表 响应结果"
// @Router       /jobexec/getJobExecScriptList [get]
// @Security     Bearer
func getJobExecScriptList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")
	searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchName := c.DefaultQuery("name", "")

	searchCreateUserName := c.DefaultQuery("createUserName", "")
	// 🚀 1. 接收前端传来的 lang 参数
	searchLang := c.DefaultQuery("lang", "")

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	// 数据库中拿到所有的JobExecScript列表
	objs, err := models.GetJobScriptAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的脚本模板错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的脚本模板错误：%v", err.Error()), c)
		return
	}
	allIds := []int{}

	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}

		// 模糊匹配名称
		if searchName != "" && !strings.Contains(obj.Name, searchName) {
			continue
		}

		// 🚀 2. 精确匹配脚本类型 (lang)
		if searchLang != "" && obj.Lang != searchLang {
			continue
		}

		// 先填充前端需要的数据（拿到组合好的 CreateUserName）
		obj.FillFrontAllData()

		// 对创建人进行模糊匹配过滤
		if searchCreateUserName != "" && !strings.Contains(obj.CreateUserName, searchCreateUserName) {
			continue
		}

		allIds = append(allIds, int(obj.ID))
	}

	// 如果过滤后没有数据，直接返回空列表
	if len(allIds) == 0 {
		common.OkWithDetailed(gin.H{
			"items": []models.JobScript{},
			"total": 0,
		}, "ok", c)
		return
	}

	// 根据过滤后的 ID 进行分页查询
	pagedObjs, err := models.GetJobScriptByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的脚本模板错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的脚本模板错误：%v", err.Error()), c)
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

// @Summary      获取作业脚本下拉选项
// @Description  获取作业脚本下拉选项 接口
// @Tags         jobexec-script
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取作业脚本下拉选项 响应结果"
// @Router       /jobexec/getJobExecScriptSelect [get]
// @Security     Bearer
func getJobExecScriptSelect(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	// 数据库中拿到所有的JobExecScript列表
	objs, err := models.GetJobScriptAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的脚本模板错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的脚本模板错误：%v", err.Error()), c)
		return
	}
	res := []CommonSelectResponse{}
	for _, obj := range objs {
		obj := obj
		one := CommonSelectResponse{
			Label: obj.Name,
			Value: fmt.Sprintf("%d", obj.ID),
		}
		res = append(res, one)
	}
	common.OkWithDetailed(res, "ok", c)
}

// @Summary      获取指定作业脚本基础信息
// @Description  获取指定作业脚本基础信息 接口
// @Tags         jobexec-script
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取指定作业脚本基础信息 响应结果"
// @Router       /jobexec/getJobExecScriptOne/{id} [get]
// @Security     Bearer
func getJobExecScriptOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetJobScriptById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找脚本模板错误", zap.Any("脚本模板", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithDetailed(dbObj, "ok", c)

}

// @Summary      获取作业脚本源码详情
// @Description  获取作业脚本源码详情 接口
// @Tags         jobexec-script
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取作业脚本源码详情 响应结果"
// @Router       /jobexec/getJobExecScriptDetail/{id} [get]
// @Security     Bearer
func getJobExecScriptDetail(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("脚本模板", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetJobScriptById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找脚本模板错误", zap.Any("脚本模板", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbObj.FillFrontAllData()

	common.OkWithDetailed(dbObj, "ok", c)
}

// @Summary      创建作业脚本
// @Description  创建作业脚本 接口
// @Tags         jobexec-script
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "创建作业脚本 响应结果"
// @Router       /jobexec/createJobExecScript [post]
// @Security     Bearer
func createJobExecScript(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.JobScript
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增脚本模板请求失败", zap.Any("脚本模板", reqObj), zap.Error(err))
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
		sc.Logger.Error("新增脚本模板数据库失败", zap.Any("脚本模板", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
	}
	common.OkWithMessage("创建成功", c)
}

// @Summary      更新作业脚本
// @Description  更新作业脚本 接口
// @Tags         jobexec-script
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "更新作业脚本 响应结果"
// @Router       /jobexec/updateJobExecScript [post]
// @Security     Bearer
func updateJobExecScript(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqObj models.JobScript
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增脚本模板请求失败", zap.Any("脚本模板", reqObj), zap.Error(err))
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

	_, err = models.GetJobScriptById(int(reqObj.ID))
	if err != nil {
		sc.Logger.Error("根据id找脚本模板错误", zap.Any("脚本模板", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新脚本模板错误", zap.Any("脚本模板", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

// @Summary      删除作业脚本
// @Description  删除作业脚本 接口
// @Tags         jobexec-script
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "删除作业脚本 响应结果"
// @Router       /jobexec/deleteJobExecScript/{id} [delete]
// @Security     Bearer
func deleteJobExecScript(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("删除脚本模板", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetJobScriptById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找脚本模板错误", zap.Any("脚本模板", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = dbObj.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除脚本模板错误", zap.Any("脚本模板", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("删除成功", c)
}
