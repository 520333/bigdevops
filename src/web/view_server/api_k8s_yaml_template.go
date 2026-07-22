package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	yaml3 "gopkg.in/yaml.v3"
)

// getK8sYamlTemplateList 获取K8s YAML模板列表
// @Summary      获取K8s YAML模板列表
// @Description  分页查询K8s YAML模板列表
// @Tags         k8s YAML管理模块
// @Accept       json
// @Produce      json
// @Param        page            query     int     false  "页码" default(1)
// @Param        pageSize        query     int     false  "每页数量" default(10)
// @Param        name            query     string  false  "模板名称"
// @Param        createUserName  query     string  false  "创建人名称"
// @Success      200             {object}  map[string]interface{} "成功响应"
// @Failure      400             {object}  map[string]interface{} "请求错误"
// @Failure      500             {object}  map[string]interface{} "服务器错误"
// @Security     Bearer
// @Router       /k8s/getK8sYamlTemplateList [get]
func getK8sYamlTemplateList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")

	searchUserIDInt, _ := strconv.Atoi(searchUserID)
	searchTitle := c.DefaultQuery("name", "")
	searchCreateUserName := c.DefaultQuery("createUserName", "")

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}
	objs, err := models.GetK8sYamlTemplateAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的k8s集群yaml执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的k8s集群yaml执行错误：%v", err.Error()), c)
		return
	}
	allIds := []int{}
	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}

		if searchTitle != "" && !strings.Contains(obj.Name, searchTitle) {
			continue
		}

		obj.FillFrontAllData()

		if searchCreateUserName != "" && !strings.Contains(obj.CreateUserName, searchCreateUserName) {
			continue
		}

		allIds = append(allIds, int(obj.ID))
	}

	if len(allIds) == 0 {
		common.OkWithDetailed(gin.H{
			"items": []models.K8sYamlTemplate{},
			"total": 0,
		}, "ok", c)
		return
	}

	pagedObjs, err := models.GetK8sYamlTemplateByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的k8s集群yaml执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的k8s集群yaml执行错误：%v", err.Error()), c)
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

// createK8sYamlTemplate 创建K8s YAML模板
// @Summary      创建K8s YAML模板
// @Description  创建新的K8s YAML模板
// @Tags         k8s YAML管理模块
// @Accept       json
// @Produce      json
// @Param        data  body      models.K8sYamlTemplate  true  "YAML模板数据"
// @Success      200   {object}  map[string]interface{} "成功响应"
// @Failure      400   {object}  map[string]interface{} "请求错误"
// @Failure      500   {object}  map[string]interface{} "服务器错误"
// @Security     Bearer
// @Router       /k8s/createK8sYamlTemplate [post]
func createK8sYamlTemplate(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj models.K8sYamlTemplate
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增k8s集群yaml模板执行请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 获取当前用户ID
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)

	dbUser, err := models.GetUserByUsername(userName)
	if dbUser != nil {
		reqObj.UserID = dbUser.ID
	}

	var out interface{}
	err = yaml3.Unmarshal([]byte(reqObj.Content), &out)
	if err != nil {
		sc.Logger.Error("yaml文件格式有问题", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 存入数据库
	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增k8s集群yaml执行数据库失败", zap.Error(err))
		common.FailWithMessage("存入数据库失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

// updateK8sYamlTemplate 更新K8s YAML模板
// @Summary      更新K8s YAML模板
// @Description  更新现有的K8s YAML模板
// @Tags         k8s YAML管理模块
// @Accept       json
// @Produce      json
// @Param        data  body      models.K8sYamlTemplate  true  "YAML模板数据"
// @Success      200   {object}  map[string]interface{} "成功响应"
// @Failure      400   {object}  map[string]interface{} "请求错误"
// @Failure      500   {object}  map[string]interface{} "服务器错误"
// @Security     Bearer
// @Router       /k8s/updateK8sYamlTemplate [post]
func updateK8sYamlTemplate(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj models.K8sYamlTemplate
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新k8s集群yaml请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 检查是否存在
	dbOld, err := models.GetK8sYamlTemplateById(int(reqObj.ID))
	if err != nil {
		common.FailWithMessage("k8s集群yaml模板不存在", c)
		return
	}

	var out interface{}
	err = yaml3.Unmarshal([]byte(reqObj.Content), &out)
	if err != nil {
		sc.Logger.Error("yaml文件格式有问题", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	reqObj.UserID = dbOld.UserID

	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新k8s集群yaml执行错误", zap.Error(err))
		common.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

// deleteK8sYamlTemplate 删除K8s YAML模板
// @Summary      删除K8s YAML模板
// @Description  根据ID删除K8s YAML模板
// @Tags         k8s YAML管理模块
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "模板ID"
// @Success      200  {object}  map[string]interface{} "成功响应"
// @Failure      400  {object}  map[string]interface{} "请求错误"
// @Failure      500  {object}  map[string]interface{} "服务器错误"
// @Security     Bearer
// @Router       /k8s/deleteK8sYamlTemplate/{id} [delete]
func deleteK8sYamlTemplate(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetK8sYamlTemplateById(intVar)
	if err != nil {
		common.FailWithMessage("k8s集群yaml模板不存在", c)
		return
	}
	dbTasks, _ := models.GetK8sYamlTaskByTemplateId(uint(intVar))
	if dbTasks != nil && len(dbTasks) > 0 {
		sc.Logger.Warn("该yaml任务已经绑定了模板，禁止直接删除！", zap.Any("", id))
		common.FailWithMessage("该yaml任务已经绑定了模板，禁止直接删除！", c)
		return
	}

	err = dbObj.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除k8s集群yaml执行错误", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}
