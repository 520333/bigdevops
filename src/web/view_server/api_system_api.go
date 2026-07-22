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

// @Summary      获取系统API权限接口列表
// @Description  获取系统API权限接口列表 接口
// @Tags         system-api
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取系统API权限接口列表 响应结果"
// @Router       /system/getApiList [get]
// @Security     Bearer
func getApiList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	apis, err := models.GetApiAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的Api接口错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的Api接口错误：%v", err.Error()), c)
		return
	}

	fatherApiMap := make(map[uint]*models.Api)
	for _, api := range apis {
		api := api
		api.Key = api.ID
		api.Value = api.ID
		if api.Pid == 0 {
			fatherApiMap[api.ID] = api
			continue
		}
		fatherApi, err := models.GetApiById(api.Pid)
		if err != nil {

			sc.Logger.Error("通过Pid找api错误", zap.Error(err))
			continue
		}
		fatherApi.Key = fatherApi.ID
		fatherApi.Value = fatherApi.ID

		load, ok := fatherApiMap[fatherApi.ID]
		if !ok {
			fatherApi.Children = make([]*models.Api, 0)
			fatherApi.Children = append(fatherApi.Children, api)
			fatherApiMap[fatherApi.ID] = fatherApi
		} else {
			load.Children = append(load.Children, api)
		}
	}

	finalApis := make([]*models.Api, 0)
	// 最终遍历fatherApiMap
	for _, m := range fatherApiMap {
		m := m
		finalApis = append(finalApis, m)
	}

	common.OkWithDetailed(finalApis, "ok", c)

}

// @Summary      获取全量系统API接口列表
// @Description  获取全量系统API接口列表 接口
// @Tags         system-api
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取全量系统API接口列表 响应结果"
// @Router       /system/getApiListAll [get]
// @Security     Bearer
func getApiListAll(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	apis, err := models.GetApiAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的Api接口错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的Api接口错误：%v", err.Error()), c)
		return
	}

	fatherApiMap := make(map[uint]*models.Api)
	for _, api := range apis {
		api := api
		api.Key = api.ID
		api.Value = api.ID
	}

	common.OkWithDetailed(fatherApiMap, "ok", c)
}

// @Summary      创建API接口定义
// @Description  创建API接口定义 接口
// @Tags         system-api
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "创建API接口定义 响应结果"
// @Router       /system/createApi [post]
// @Security     Bearer
func createApi(c *gin.Context) {
	// 校验Api字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqApi models.Api
	err := c.ShouldBindJSON(&reqApi)
	if err != nil {
		sc.Logger.Error("解析新增api请求失败", zap.Any("api", reqApi), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqApi)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	err = reqApi.CreateOne()
	success, err := models.CasbinEnforcer.AddPolicy("super", reqApi.Path, reqApi.Method)
	if err != nil || !success {
		sc.Logger.Error("同步Casbin策略失败", zap.Error(err))
		// 注意：这里仅仅打印日志，不让前端报错，因为 API 已经存入字典了
	}
	if err != nil {
		sc.Logger.Error("创建Api错误", zap.Any("api", reqApi), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("创建成功", c)
}

// @Summary      更新API接口定义
// @Description  更新API接口定义 接口
// @Tags         system-api
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "更新API接口定义 响应结果"
// @Router       /system/updateApi [post]
// @Security     Bearer
func updateApi(c *gin.Context) {
	// 校验Api字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqApi models.Api
	err := c.ShouldBindJSON(&reqApi)
	if err != nil {
		sc.Logger.Error("解析更新api请求失败", zap.Any("api", reqApi), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	err = validate.Struct(&reqApi)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	// 获取旧的 Api 数据
	// 1. 获取旧的 Api 数据，保留引用为 oldApi
	oldApi, err := models.GetApiById(int(reqApi.ID))
	if err != nil {
		sc.Logger.Error("根据id找Api错误", zap.Any("api", reqApi), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// ================= 3. 拦截违规的 类型转换/层级修改 =================
	// 场景 A：试图把【父级】变成【子级】(硬拦截)
	if oldApi.Pid == 0 && reqApi.Pid != 0 {
		common.FailWithMessage("类型已锁定，不允许将父级节点修改为子级！", c)
		return
	}

	// 场景 B：试图把【子级】变成【父级】(硬拦截)
	if oldApi.Pid != 0 && reqApi.Pid == 0 {
		common.FailWithMessage("类型已锁定，不允许将子级节点修改为父级！", c)
		return
	}
	// =======================================================
	err = reqApi.UpdateOne()
	if err != nil {
		sc.Logger.Error("根据id更新Api错误", zap.Any("api", reqApi), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 3. 同步更新 Casbin 策略
	// 如果路径或方法发生了改变，才需要去更新 Casbin
	if oldApi.Path != reqApi.Path || oldApi.Method != reqApi.Method {
		oldPolicy := []string{"super", oldApi.Path, oldApi.Method}
		newPolicy := []string{"super", reqApi.Path, reqApi.Method}

		// 使用标准的 UpdatePolicy 替换规则
		success, err := models.CasbinEnforcer.UpdatePolicy(oldPolicy, newPolicy)
		if err != nil || !success {
			sc.Logger.Error("同步更新Casbin策略失败", zap.Error(err))
			// 仅打印日志
		}
	}
	common.OkWithMessage("更新成功", c)
}

// @Summary      删除API接口定义
// @Description  删除API接口定义 接口
// @Tags         system-api
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "删除API接口定义 响应结果"
// @Router       /system/deleteApi/{id} [delete]
// @Security     Bearer
func deleteApi(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("删除api权限", zap.Any("id", id))

	intVar, _ := strconv.Atoi(id)
	dbApi, err := models.GetApiById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找api错误", zap.Any("api", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = dbApi.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除api错误", zap.Any("api", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	success, err := models.CasbinEnforcer.RemoveFilteredPolicy(1, dbApi.Path, dbApi.Method)
	if err != nil || !success {
		sc.Logger.Warn("清理Casbin策略失败或规则不存在", zap.Error(err))
		// 同样，这里仅打印日志，不影响主流程
	}
	common.OkWithMessage("删除成功", c)
}
