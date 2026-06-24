package view

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

func getMenuList(c *gin.Context) {
	// 拿到用户对应的role列表 遍历role列表 找到Menu List 拼接父子结构 返回的是组数 第一层father 第二层 children
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		sc.Logger.Error("通过token解析到的userName去数据库中找User", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("通过token解析到的userName去数据库中找User失败 %v", err.Error()), c)
		return
	}

	fatherMenuMap := make(map[uint]*models.Menu)
	uniqueChildMap := make(map[uint]*models.Menu)
	roles := dbUser.Roles
	for _, role := range roles {
		role := role
		if role.Status == common.COMMON_STATUS_DISABLE {
			sc.Logger.Info("用户的角色禁用 跳过帅选菜单",
				zap.String("用户", dbUser.Username),
				zap.String("角色", role.RoleName),
			)
			continue
		}
		for _, menu := range role.Menus {
			menu := menu
			if menu.Status == common.COMMON_STATUS_DISABLE {
				if role.RoleValue != "super" {

					sc.Logger.Info("菜单禁用 跳过帅选菜单",
						zap.String("用户", dbUser.Username),
						zap.String("菜单", menu.Name),
					)
					continue
				}
			}

			// 拼接前端依赖的字段
			menu.Meta = &models.MenuMeta{}
			menu.Meta.Icon = menu.Icon
			menu.Meta.Title = menu.Title
			//menu.Meta.ShowMenu = common.COMMON_SHOW_MAP[menu.Show]
			showBool := common.COMMON_SHOW_MAP[menu.Show]
			menu.Meta.ShowMenu = showBool
			menu.Meta.HideMenu = !showBool
			if menu.Path == "stree" || menu.Path == "menu" {
				menu.Meta.IgnoreKeepAlive = true
			}
			if menu.Pid == 0 {
				fatherMenuMap[menu.ID] = menu
				continue
			}
			fatherMenu, err := models.GetMenuById(menu.Pid)
			if err != nil {

				sc.Logger.Error("通过ParentMenu找menu错误", zap.Error(err))
				continue
			}

			_, ok := uniqueChildMap[menu.ID]
			if ok {
				continue
			}
			uniqueChildMap[menu.ID] = menu

			//fatherMenu.Meta = &models.MenuMeta{}
			//fatherMenu.Meta.Icon = menu.Icon
			//fatherMenu.Meta.Title = menu.Title
			//fatherMenuShowBool := common.COMMON_SHOW_MAP[menu.Show]
			//fatherMenu.Meta.ShowMenu = fatherMenuShowBool
			//fatherMenu.Meta.HideMenu = !fatherMenuShowBool

			load, ok := fatherMenuMap[fatherMenu.ID]
			if !ok {
				fatherMenu.Children = make([]*models.Menu, 0)
				fatherMenu.Children = append(fatherMenu.Children, menu)
				fatherMenuMap[fatherMenu.ID] = fatherMenu
			} else {
				load.Children = append(load.Children, menu)
			}
		}

	}

	finalMenus := make([]*models.Menu, 0)
	// 最终遍历fatherMenuMap
	for _, m := range fatherMenuMap {
		m := m
		finalMenus = append(finalMenus, m)
	}

	// ================= 👇 新增的排序代码加在这里 👇 =================

	// 1. 对最外层的顶级菜单进行排序 (OrderNo 从小到大)
	if len(finalMenus) > 1 {
		sort.Slice(finalMenus, func(i, j int) bool {
			return finalMenus[i].OrderNo < finalMenus[j].OrderNo
		})
	}

	// 2. 遍历顶级菜单，对它们里面的子菜单 (Children) 进行排序
	for _, father := range finalMenus {
		if len(father.Children) > 1 {
			sort.Slice(father.Children, func(i, j int) bool {
				return father.Children[i].OrderNo < father.Children[j].OrderNo
			})
		}
	}

	// ================= 👆 排序代码结束 👆 =================

	common.OkWithDetailed(finalMenus, "ok", c)

}

//	func getMenuListAll(c *gin.Context) {
//		sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
//		// 数据库中拿到所有的menu列表
//		menus, err := models.GetMenuAll()
//		if err != nil {
//			sc.Logger.Error("去数据库中拿所有的菜单错误", zap.Error(err))
//			common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的菜单错误：%v", err.Error()), c)
//			return
//		}
//
//		fatherMenuMap := make(map[uint]*models.Menu)
//		for _, menu := range menus {
//			menu := menu
//			// 拼接前端依赖的字段
//			menu.Meta = &models.MenuMeta{}
//			menu.Meta.Icon = menu.Icon
//			menu.Meta.Title = menu.Title
//			//menu.Meta.ShowMenu = common.COMMON_SHOW_MAP[menu.Show]
//			showBool := common.COMMON_SHOW_MAP[menu.Show]
//			menu.Meta.ShowMenu = showBool
//			menu.Meta.HideMenu = !showBool
//
//			//if menu.ParentMenu == "" {
//			//	menu.Id = fmt.Sprintf("%v", menu.ID)
//			//	fatherMenuMap[menu.ID] = menu
//			//	continue
//			//} else {
//			//	menu.Id = fmt.Sprintf("%s-%v", menu.ParentMenu, menu.ID)
//			//}
//			fatherMenuId, _ := strconv.Atoi(menu.ParentMenu)
//			fatherMenu, err := models.GetMenuById(fatherMenuId)
//			if err != nil {
//				sc.Logger.Error("通过ParentMenu找menu错误", zap.Error(err))
//				continue
//			}
//
//			load, ok := fatherMenuMap[fatherMenu.ID]
//			if !ok {
//				fatherMenu.Children = make([]*models.Menu, 0)
//				fatherMenu.Children = append(fatherMenu.Children, menu)
//				fatherMenuMap[fatherMenu.ID] = fatherMenu
//			} else {
//				load.Children = append(load.Children, menu)
//			}
//		}
//
//		finalMenus := make([]*models.Menu, 0)
//		// 最终遍历fatherMenuMap
//		for _, m := range fatherMenuMap {
//			m := m
//			finalMenus = append(finalMenus, m)
//		}
//
//		common.OkWithDetailed(finalMenus, "ok", c)
//
// }
func getMenuListAll(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	// 数据库中拿到所有的menu列表
	menus, err := models.GetMenuAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的菜单错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的菜单错误：%v", err.Error()), c)
		return
	}

	for _, menu := range menus {
		menu := menu
		// 拼接前端依赖的字段
		menu.Meta = &models.MenuMeta{}
		menu.Meta.Icon = menu.Icon
		menu.Meta.Title = menu.Title
		//menu.Meta.ShowMenu = common.COMMON_SHOW_MAP[menu.Show]
		showBool := common.COMMON_SHOW_MAP[menu.Show]
		menu.Meta.ShowMenu = showBool
		menu.Meta.HideMenu = !showBool
		menu.Key = menu.ID
		menu.Value = menu.ID
	}

	common.OkWithDetailed(menus, "ok", c)

}

func updateMenu(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqMenu models.Menu
	err := c.ShouldBindJSON(&reqMenu)
	if err != nil {
		sc.Logger.Error("解析更新菜单请求失败", zap.Any("菜单", reqMenu), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	sc.Logger.Info("更新菜单请求", zap.Any("菜单", reqMenu.Name))
	sc.Logger.Info("更新菜单请求", zap.Any("菜单", reqMenu))
	err = validate.Struct(&reqMenu)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}
	_, err = models.GetMenuById(int(reqMenu.ID))
	if err != nil {
		sc.Logger.Error("根据id找menu错误", zap.Any("菜单", reqMenu), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	err = reqMenu.UpdateOne()
	if err != nil {
		sc.Logger.Error("根据id更新menu错误", zap.Any("菜单", reqMenu), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("更新成功", c)
}

func createMenu(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqMenu models.Menu
	err := c.ShouldBindJSON(&reqMenu)
	if err != nil {
		sc.Logger.Error("解析新增菜单请求失败", zap.Any("菜单", reqMenu), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqMenu)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	err = reqMenu.CreateOne()
	if err != nil {
		sc.Logger.Error("创建menu错误", zap.Any("菜单", reqMenu), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("创建成功", c)
}

func deleteMenu(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("删除菜单", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbMenu, err := models.GetMenuById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找菜单错误", zap.Any("菜单", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	err = dbMenu.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除菜单错误", zap.Any("菜单", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("删除成功", c)
}
