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

	noAuth := r.Group("/noAuth")
	{
		noAuth.GET("/downloadPrometheusMainConfigYaml", downloadPrometheusMainConfigYaml) //给prometheus使用的
		noAuth.GET("/downloadAlertManagerMainConfigYaml", downloadAlertManagerMainConfigYaml)
		noAuth.GET("/getLeafStreeNodeBindIps", getLeafStreeNodeBindIps)
	}

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
		systemApiGroup.GET("/getAllUserAndRoles", getAllUserAndRoles)

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
		sTreeApiGroup.GET("/getStreeNodeSelect", getStreeNodeSelect)
		sTreeApiGroup.GET("/getTopStreeNodes", getTopStreeNodes)
		sTreeApiGroup.POST("/createStreeNode", createStreeNode)
		sTreeApiGroup.POST("/updateStreeNode", updateStreeNode)
		sTreeApiGroup.DELETE("/deleteStreeNode/:id", deleteStreeNode)
		sTreeApiGroup.GET("/getChildrenStreeNodes/:pid", getChildrenStreeNodes)
		sTreeApiGroup.GET("/getLeafStreeNodes", getLeafStreeNodes)

		// ECS
		sTreeApiGroup.GET("/getResourceEcsUnbindList", getResourceEcsUnbindList)

		sTreeApiGroup.POST("/bindEcsToStreeNode", bindEcsToStreeNode)
		sTreeApiGroup.POST("/unBindEcsToStreeNode", unBindEcsToStreeNode)
		sTreeApiGroup.GET("getStreeNodeEcsList/:id", getStreeNodeEcsList)
		sTreeApiGroup.GET("/getResourceEcsList", getResourceEcsList)
		// ELB
		sTreeApiGroup.GET("/getResourceElbUnbindList", getResourceElbUnbindList)
		sTreeApiGroup.POST("/bindElbToStreeNode", bindElbToStreeNode)
		sTreeApiGroup.POST("/unBindElbToStreeNode", unBindElbToStreeNode)

		// RDS
		sTreeApiGroup.GET("/getResourceRdsUnbindList", getResourceRdsUnbindList)
		sTreeApiGroup.POST("/bindRdsToStreeNode", bindRdsToStreeNode)
		sTreeApiGroup.POST("/unBindRdsToStreeNode", unBindRdsToStreeNode)

		sTreeApiGroup.GET("/fetchResourceByNode", fetchResourceByNode)
	}

	workOrDerApiGroup := afterLoginApiGroup.Group("/workorder")
	{
		workOrDerApiGroup.GET("/getProcessList", getProcessList)
		workOrDerApiGroup.POST("/createProcess", createProcess)
		workOrDerApiGroup.POST("/updateProcess", updateProcess)
		workOrDerApiGroup.DELETE("/deleteProcess/:id", deleteProcess)

		workOrDerApiGroup.GET("/getFormDesignList", getFormDesignList)
		workOrDerApiGroup.POST("/createFormDesign", createFormDesign)
		workOrDerApiGroup.POST("/updateFormDesign", updateFormDesign)
		workOrDerApiGroup.DELETE("/deleteFormDesign/:id", deleteFormDesign)

		workOrDerApiGroup.GET("/getWorkOrderTemplateList", getWorkOrderTemplateList)
		workOrDerApiGroup.POST("/createWorkOrderTemplate", createWorkOrderTemplate)
		workOrDerApiGroup.POST("/updateWorkOrderTemplate", updateWorkOrderTemplate)
		workOrDerApiGroup.DELETE("/deleteWorkOrderTemplate/:id", deleteWorkOrderTemplate)
		workOrDerApiGroup.GET("/getWorkOrderTemplateDetail/:id", getWorkOrderTemplateDetail)

		workOrDerApiGroup.GET("/getWorkOrderInstanceList", getWorkOrderInstanceList)
		workOrDerApiGroup.POST("/createWorkOrderInstance", createWorkOrderInstance)
		workOrDerApiGroup.POST("/updateWorkOrderInstance", updateWorkOrderInstance)
		workOrDerApiGroup.DELETE("/deleteWorkOrderInstance/:id", deleteWorkOrderInstance)
		workOrDerApiGroup.POST("/approvalWorkOrderInstance/:id", approvalWorkOrderInstance)
		workOrDerApiGroup.POST("/actionWorkOrderInstance/:id", actionWorkOrderInstance)
		workOrDerApiGroup.GET("/getWorkOrderInstanceDetail/:id", getWorkOrderInstanceDetail)
		workOrDerApiGroup.POST("/commentWorkOrderInstance/:id", commentWorkOrderInstance)

	}

	jobExecApiGroup := afterLoginApiGroup.Group("/jobexec")
	{
		jobExecApiGroup.GET("/getJobExecScriptList", getJobExecScriptList)
		jobExecApiGroup.GET("/getJobExecScriptSelect", getJobExecScriptSelect)
		jobExecApiGroup.GET("/getJobExecScriptOne/:id", getJobExecScriptOne)
		jobExecApiGroup.POST("/createJobExecScript", createJobExecScript)
		jobExecApiGroup.POST("/updateJobExecScript", updateJobExecScript)
		jobExecApiGroup.DELETE("/deleteJobExecScript/:id", deleteJobExecScript)
		jobExecApiGroup.GET("/getJobExecScriptDetail/:id", getJobExecScriptDetail)

		jobExecApiGroup.GET("/getJobExecTaskList", getJobExecTaskList)
		jobExecApiGroup.POST("/createJobExecTask", createJobExecTask)
		jobExecApiGroup.POST("/updateJobExecTask", updateJobExecTask)
		jobExecApiGroup.DELETE("/deleteJobExecTask/:id", deleteJobExecTask)
		jobExecApiGroup.GET("/getJobExecTaskOne/:id", getJobExecTaskOne)
		jobExecApiGroup.POST("/actionJobExecTaskOne/:id", actionJobExecTaskOne)
		jobExecApiGroup.GET("/getJobExecResultByJobId", getJobExecResultByJobId)
	}

	monitorApiGroup := afterLoginApiGroup.Group("/monitor")
	{
		monitorApiGroup.GET("/getMonitorScrapePoolList", getMonitorScrapePoolList)
		monitorApiGroup.POST("/createMonitorScrapePool", createMonitorScrapePool)
		monitorApiGroup.POST("/updateMonitorScrapePool", updateMonitorScrapePool)
		monitorApiGroup.DELETE("/deleteMonitorScrapePool/:id", deleteMonitorScrapePool)
		monitorApiGroup.GET("/getMonitorPrometheusYamlOne", getMonitorPrometheusYamlOne)

		monitorApiGroup.GET("/getMonitorScrapeJobList", getMonitorScrapeJobList)
		monitorApiGroup.POST("/createMonitorScrapeJob", createMonitorScrapeJob)
		monitorApiGroup.POST("/updateMonitorScrapeJob", updateMonitorScrapeJob)
		monitorApiGroup.DELETE("/deleteMonitorScrapeJob/:id", deleteMonitorScrapeJob)
		monitorApiGroup.GET("/getMonitorScrapeJobOne", getMonitorScrapeJobOne)
		monitorApiGroup.POST("/setScrapeJobStatus", setScrapeJobStatus)

		monitorApiGroup.GET("/getMonitorAlertManagerYamlOne", getMonitorAlertManagerYamlOne)
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
