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

// @Summary      获取全量系统角色列表
// @Description  获取全量系统角色列表 接口
// @Tags         system-role
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取全量系统角色列表 响应结果"
// @Router       /system/getRoleListAll [get]
// @Security     Bearer
func getRoleListAll(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	// 数据库中拿到所有的menu列表
	roles, err := models.GetRoleAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的角色错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的角色错误：%v", err.Error()), c)
		return
	}

	for _, role := range roles {
		role := role
		for _, menu := range role.Menus {
			menu.Key = menu.ID
			menu.Value = menu.ID
		}
	}

	common.OkWithDetailed(roles, "ok", c)

}

// @Summary      创建系统角色
// @Description  创建系统角色 接口
// @Tags         system-role
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "创建系统角色 响应结果"
// @Router       /system/createRole [post]
// @Security     Bearer
func createRole(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqRole models.Role
	err := c.ShouldBindJSON(&reqRole)
	if err != nil {
		sc.Logger.Error("解析新增角色请求失败", zap.Any("角色", reqRole), zap.Error(err))
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

	menus := make([]*models.Menu, 0)
	for _, menuId := range reqRole.MenuIds {
		dbMenu, err := models.GetMenuById(menuId)
		if err != nil {
			sc.Logger.Error("根据id找菜单错误", zap.Any("菜单", reqRole), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		menus = append(menus, dbMenu)
	}
	reqRole.Menus = menus

	// ================= 2. 处理 API 关联 (新增缺失的逻辑) =================
	apis := make([]*models.Api, 0)
	for _, apiId := range reqRole.ApiIds {
		dbApi, err := models.GetApiById(apiId)
		if err != nil {
			sc.Logger.Error("根据id找Api错误", zap.Any("api", reqRole), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		apis = append(apis, dbApi)
	}
	reqRole.Apis = apis

	err = reqRole.CreateOne()
	if err != nil {
		sc.Logger.Error("创建角色错误", zap.Any("角色", reqRole), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

// @Summary      更新系统角色
// @Description  更新系统角色 接口
// @Tags         system-role
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "更新系统角色 响应结果"
// @Router       /system/updateRole [post]
// @Security     Bearer
func updateRole(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqRole models.Role
	err := c.ShouldBindJSON(&reqRole)
	if err != nil {
		sc.Logger.Error("解析更新角色请求失败", zap.Any("角色", reqRole), zap.Error(err))
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

	_, err = models.GetRoleById(int(reqRole.ID))
	if err != nil {
		sc.Logger.Error("根据id找角色错误", zap.Any("角色", reqRole), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	menus := make([]*models.Menu, 0)
	menuIdMap := make(map[int]bool)
	for _, menuId := range reqRole.MenuIds {
		dbMenu, err := models.GetMenuById(menuId)
		if err != nil {
			sc.Logger.Error("根据id找菜单错误", zap.Any("菜单", reqRole), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}

		menus = append(menus, dbMenu)
		menuIdMap[int(dbMenu.ID)] = true
	}

	apis := make([]*models.Api, 0)
	apisIdMap := make(map[int]bool)
	for _, ApiId := range reqRole.ApiIds {
		dbApi, err := models.GetApiById(ApiId)
		if err != nil {
			sc.Logger.Error("根据id找菜单错误", zap.Any("菜单", reqRole), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}

		apis = append(apis, dbApi)
		apisIdMap[int(dbApi.ID)] = true
	}

	extraMenus := make([]*models.Menu, 0)
	for _, m := range menus {
		if m.Pid != 0 && !menuIdMap[m.Pid] {
			parent, err := models.GetMenuById(m.Pid)
			if err == nil {
				extraMenus = append(extraMenus, parent)
				menuIdMap[m.Pid] = true // 防止重复添加同一个父 ID
			}
		}
	}
	menus = append(menus, extraMenus...)

	// 更新
	err = reqRole.UpdateMenus(menus)
	if err != nil {
		sc.Logger.Error("更新角色和关联菜单错误", zap.Any("角色", reqRole), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = reqRole.UpdateApis(apis)
	if err != nil {
		sc.Logger.Error("更新角色关联接口错误", zap.Any("apis", reqRole), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	models.CasbinEnforcer.RemoveFilteredPolicy(0, reqRole.RoleValue)

	// 2. 将这次新勾选的 API 组装成 Casbin 需要的格式并批量添加
	var rules [][]string
	for _, api := range apis {
		// Casbin 的标准策略格式: p, 角色名, 路径, 请求方法
		rules = append(rules, []string{reqRole.RoleValue, api.Path, api.Method})
	}

	// 3. 批量写入 Casbin
	if len(rules) > 0 {
		success, err := models.CasbinEnforcer.AddPolicies(rules)
		if err != nil || !success {
			sc.Logger.Error("同步角色权限到Casbin失败", zap.Error(err))
			// 仅打印日志，不阻断正常返回
		}
	}
	// ========================================================
	common.OkWithMessage("更新成功", c)
}

// @Summary      设置角色启用状态
// @Description  设置角色启用状态 接口
// @Tags         system-role
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "设置角色启用状态 响应结果"
// @Router       /system/setRoleStatus [post]
// @Security     Bearer
func setRoleStatus(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqRole models.SetRoleStatusReq
	err := c.ShouldBindJSON(&reqRole)
	if err != nil {
		sc.Logger.Error("解析新增角色请求失败", zap.Any("角色", reqRole), zap.Error(err))
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
		sc.Logger.Error("根据id找角色错误", zap.Any("角色", reqRole), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbRole.Status = reqRole.Status

	// 更新
	err = dbRole.UpdateMenus(dbRole.Menus)
	if err != nil {
		sc.Logger.Error("跟新角色和关联菜单错误", zap.Any("角色", reqRole), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("创建成功", c)
}

// @Summary      删除系统角色
// @Description  删除系统角色 接口
// @Tags         system-role
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "删除系统角色 响应结果"
// @Router       /system/deleteRole/{id} [delete]
// @Security     Bearer
func deleteRole(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("删除角色", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbRole, err := models.GetRoleById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找角色错误", zap.Any("角色", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	err = dbRole.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除角色错误", zap.Any("角色", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("删除成功", c)
}
