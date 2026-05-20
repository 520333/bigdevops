package view

import (
	"bigdevops/src/web/middleware"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

func ConfigRouter(r *gin.Engine) {
	base := r.Group("/")
	base.GET("/ping", ping)
	base.GET("/now", getNowTs)
	base.GET("/long", longRequest)
	base.POST("/login", UserLogin)
	base.GET("/logout", UserLogout)

	// 以下开始需要认证
	afterLoginApiGroup := r.Group("/api")
	afterLoginApiGroup.Use(middleware.JWTAuthMiddleWare()).Use(middleware.CasBinRbacMiddleware())
	{
		afterLoginApiGroup.GET("/getUserInfo", getUserAfterLogin)
		afterLoginApiGroup.GET("/getPermCode", getPermCode)
	}
	systemApiGroup := afterLoginApiGroup.Group("/system")
	{
		// 菜单路由
		systemApiGroup.GET("/getMenuList", getMenuList)
		systemApiGroup.GET("/getMenuListAll", getMenuListAll)
		systemApiGroup.POST("/createMenu", createMenu)
		systemApiGroup.POST("/updateMenu", updateMenu)
		systemApiGroup.DELETE("/deleteMenu/:id", deleteMenu)

		// 角色路由
		systemApiGroup.GET("/getRoleListAll", getRoleListAll)
		systemApiGroup.POST("/createRole", createRole)
		systemApiGroup.POST("/updateRole", updateRole)
		systemApiGroup.POST("/setRoleStatus", setRoleStatus)
		systemApiGroup.DELETE("/deleteRole/:id", deleteRole)

		// 账号路由
		systemApiGroup.POST("/createAccount", createAccount)
		systemApiGroup.POST("/updateAccount", updateAccount)
		systemApiGroup.POST("/accountExist", accountExist)
		systemApiGroup.DELETE("/deleteAccount/:id", deleteAccount)
		systemApiGroup.GET("/getAccountList", getAccountList)
		systemApiGroup.POST("/changePassword", changePassword)

		// 菜单路由
		systemApiGroup.GET("/getApiList", getApiList)
		systemApiGroup.GET("/getApiListAll", getApiListAll)
		systemApiGroup.POST("/createApi", createApi)
		systemApiGroup.POST("/updateApi", updateApi)
		systemApiGroup.DELETE("/deleteApi/:id", deleteApi)

	}

	sTreeApiGroup := afterLoginApiGroup.Group("/stree")
	{
		sTreeApiGroup.GET("/getStreeNodeList", getStreeNodeList)
		sTreeApiGroup.GET("/getTopStreeNodes", getTopStreeNodes)
		sTreeApiGroup.POST("/createStreeNode", createStreeNode)
		sTreeApiGroup.POST("/updateStreeNode", updateStreeNode)
		sTreeApiGroup.DELETE("/deleteStreeNode/:id", deleteStreeNode)
		sTreeApiGroup.GET("/getChildrenStreeNodes/:pid", getChildrenStreeNodes)

		sTreeApiGroup.GET("/getResourceEcsUnbindList", getResourceEcsUnbindList)
		sTreeApiGroup.POST("/bindEcsToStreeNode", bindEcsToStreeNode)
		sTreeApiGroup.POST("/unBindEcsToStreeNode", unBindEcsToStreeNode)
		sTreeApiGroup.GET("/fetchResourceByNode", fetchResourceByNode)
	}

}

func getNowTs(c *gin.Context) {
	c.String(200, time.Now().Format("2006-01-02 15:04:05"))
}

func longRequest(c *gin.Context) {
	fmt.Printf("longRequest请求开始，休息6秒 %v\n", c)
	time.Sleep(5 * time.Second)
	c.String(200, "longRequest请求结束")
}
