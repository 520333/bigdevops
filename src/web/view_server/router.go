package view_server

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
		noAuth.GET("/downloadPrometheusMainConfigYaml", downloadPrometheusMainConfigYaml) //prometheus主配置文件
		noAuth.GET("/downloadPrometheusAlertRuleMainConfigYaml", downloadPrometheusAlertRuleMainConfigYaml)
		noAuth.GET("/downloadPrometheusRecordRuleMainConfigYaml", downloadPrometheusRecordRuleMainConfigYaml)
		noAuth.GET("/downloadAlertManagerMainConfigYaml", downloadAlertManagerMainConfigYaml) //alertManager主配置文件
		noAuth.GET("/getLeafStreeNodeBindIps", getLeafStreeNodeBindIps)
		noAuth.GET("/getMonitorOndutyGroupFuturePlan/:id", getMonitorOndutyGroupFuturePlan)
	}

	// 以下开始需要认证
	afterLoginApiGroup := r.Group("/api")
	afterLoginApiGroup.
		Use(middleware.JWTAuthMiddleWare()).    // jwt中间件
		Use(middleware.UserStatusMiddleware()). // 用户状态中间件
		Use(middleware.CasBinRbacMiddleware())  // rbac	中间件

	{
		afterLoginApiGroup.GET("/getUserInfo", getUserAfterLogin)
		afterLoginApiGroup.GET("/getPermCode", getPermCode)
	}
	systemApiGroup := afterLoginApiGroup.Group("/system")
	{
		// 账号路由
		systemApiGroup.POST("/createAccount", createAccount)
		systemApiGroup.POST("/updateAccount", updateAccount)
		systemApiGroup.POST("/accountExist", accountExist)
		systemApiGroup.DELETE("/deleteAccount/:id", deleteAccount)
		systemApiGroup.POST("/setAccountStatus", setAccountStatus)
		systemApiGroup.GET("/getAccountList", getAccountList)
		systemApiGroup.POST("/changePassword", changePassword)
		systemApiGroup.GET("/getAllUserAndRoles", getAllUserAndRoles)

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
		systemApiGroup.DELETE("/deleteRole/:id", deleteRole)
		systemApiGroup.POST("/setRoleStatus", setRoleStatus)

		// api 路由
		systemApiGroup.GET("/getApiList", getApiList)
		systemApiGroup.GET("/getApiListAll", getApiListAll)
		systemApiGroup.POST("/createApi", createApi)
		systemApiGroup.POST("/updateApi", updateApi)
		systemApiGroup.DELETE("/deleteApi/:id", deleteApi)

		// system settings 路由
		systemApiGroup.GET("/setting/get", GetSystemSetting)
		systemApiGroup.PUT("/setting/update", UpdateSystemSetting)
	}

	sTreeApiGroup := afterLoginApiGroup.Group("/stree")
	{
		sTreeApiGroup.GET("/getStreeNodeList", getStreeNodeList)
		sTreeApiGroup.GET("/getStreeNodeSelect", getStreeNodeSelect)
		sTreeApiGroup.GET("/getTopStreeNodes", getTopStreeNodes)
		sTreeApiGroup.GET("/getChildrenStreeNodes/:pid", getChildrenStreeNodes)
		sTreeApiGroup.POST("/createStreeNode", createStreeNode)
		sTreeApiGroup.POST("/updateStreeNode", updateStreeNode)
		sTreeApiGroup.DELETE("/deleteStreeNode/:id", deleteStreeNode)
		sTreeApiGroup.GET("/getLeafStreeNodes", getLeafStreeNodes)
		sTreeApiGroup.GET("/fetchResourceByNode", fetchResourceByNode)
		// ECS
		sTreeApiGroup.POST("/bindEcsToStreeNode", bindEcsToStreeNode)
		sTreeApiGroup.POST("/unBindEcsToStreeNode", unBindEcsToStreeNode)
		sTreeApiGroup.GET("getStreeNodeEcsList/:id", getStreeNodeEcsList)
		sTreeApiGroup.GET("/getResourceEcsUnbindList", getResourceEcsUnbindList)
		sTreeApiGroup.GET("/getResourceEcsList", getResourceEcsList)
		// ELB
		sTreeApiGroup.GET("/getResourceElbUnbindList", getResourceElbUnbindList)
		sTreeApiGroup.POST("/bindElbToStreeNode", bindElbToStreeNode)
		sTreeApiGroup.POST("/unBindElbToStreeNode", unBindElbToStreeNode)

		// RDS
		sTreeApiGroup.GET("/getResourceRdsUnbindList", getResourceRdsUnbindList)
		sTreeApiGroup.POST("/bindRdsToStreeNode", bindRdsToStreeNode)
		sTreeApiGroup.POST("/unBindRdsToStreeNode", unBindRdsToStreeNode)

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
		jobExecApiGroup.POST("/createJobExecScript", createJobExecScript)
		jobExecApiGroup.POST("/updateJobExecScript", updateJobExecScript)
		jobExecApiGroup.DELETE("/deleteJobExecScript/:id", deleteJobExecScript)
		jobExecApiGroup.GET("/getJobExecScriptSelect", getJobExecScriptSelect)
		jobExecApiGroup.GET("/getJobExecScriptOne/:id", getJobExecScriptOne)
		jobExecApiGroup.GET("/getJobExecScriptDetail/:id", getJobExecScriptDetail)

		jobExecApiGroup.GET("/getJobExecTaskList", getJobExecTaskList)
		jobExecApiGroup.GET("/getJobExecTaskOne/:id", getJobExecTaskOne)
		jobExecApiGroup.POST("/createJobExecTask", createJobExecTask)
		jobExecApiGroup.POST("/updateJobExecTask", updateJobExecTask)
		jobExecApiGroup.DELETE("/deleteJobExecTask/:id", deleteJobExecTask)
		jobExecApiGroup.POST("/actionJobExecTaskOne/:id", actionJobExecTaskOne)
		jobExecApiGroup.GET("/getJobExecResultByJobId", getJobExecResultByJobId)
	}

	monitorApiGroup := afterLoginApiGroup.Group("/monitor")
	{
		// prometheus 集群
		monitorApiGroup.GET("/getMonitorPromScrapePoolList", getMonitorPromScrapePoolList)
		monitorApiGroup.POST("/createMonitorPromScrapePool", createMonitorPromScrapePool)
		monitorApiGroup.POST("/updateMonitorPromScrapePool", updateMonitorPromScrapePool)
		monitorApiGroup.DELETE("/deleteMonitorPromScrapePool/:id", deleteMonitorPromScrapePool)
		monitorApiGroup.GET("/getMonitorPrometheusYamlOne", getMonitorPrometheusYamlOne)
		monitorApiGroup.GET("/getMonitorPrometheusAlertRuleYamlOne", getMonitorPrometheusAlertRuleYamlOne)
		monitorApiGroup.GET("/getMonitorPrometheusRecordRuleYamlOne", getMonitorPrometheusRecordRuleYamlOne)

		// prometheus 采集任务
		monitorApiGroup.GET("/getMonitorPromScrapeJobList", getMonitorPromScrapeJobList)
		monitorApiGroup.GET("/getMonitorPromScrapeJobOne", getMonitorPromScrapeJobOne)
		monitorApiGroup.POST("/createMonitorPromScrapeJob", createMonitorPromScrapeJob)
		monitorApiGroup.POST("/updateMonitorPromScrapeJob", updateMonitorPromScrapeJob)
		monitorApiGroup.DELETE("/deleteMonitorPromScrapeJob/:id", deleteMonitorPromScrapeJob)
		monitorApiGroup.POST("/setMonitorPromScrapeJobStatus", setMonitorPromScrapeJobStatus)

		// prometheus 告警规则
		monitorApiGroup.GET("/getMonitorPromAlertRuleList", getMonitorPromAlertRuleList)
		monitorApiGroup.POST("/createMonitorPromAlertRule", createMonitorPromAlertRule)
		monitorApiGroup.POST("/updateMonitorPromAlertRule", updateMonitorPromAlertRule)
		monitorApiGroup.DELETE("/deleteMonitorPromAlertRule/:id", deleteMonitorPromAlertRule)
		monitorApiGroup.DELETE("/deleteMonitorPromAlertRuleBatch", deleteMonitorPromAlertRuleBatch)
		monitorApiGroup.POST("/setMonitorPromAlertRuleStatus", setMonitorPromAlertRuleStatus)
		monitorApiGroup.POST("/setMonitorPromAlertRuleStatusBatch", setMonitorPromAlertRuleStatusBatch)
		monitorApiGroup.GET("/promqlExprCheck", promqlExprCheck)

		// prometheus 预聚合规则
		monitorApiGroup.GET("/getMonitorPromRecordRuleList", getMonitorPromRecordRuleList)
		monitorApiGroup.POST("/createMonitorPromRecordRule", createMonitorPromRecordRule)
		monitorApiGroup.POST("/updateMonitorPromRecordRule", updateMonitorPromRecordRule)
		monitorApiGroup.DELETE("/deleteMonitorPromRecordRule/:id", deleteMonitorPromRecordRule)
		monitorApiGroup.DELETE("/deleteMonitorPromRecordRuleBatch", deleteMonitorPromRecordRuleBatch)
		monitorApiGroup.POST("/setMonitorPromRecordRuleStatus", setMonitorPromRecordRuleStatus)
		monitorApiGroup.POST("/setMonitorPromRecordRuleStatusBatch", setMonitorPromRecordRuleStatusBatch)
		monitorApiGroup.GET("/recordRulePromqlExprCheck", recordRulePromqlExprCheck)

		// alertManager 集群
		monitorApiGroup.GET("/getMonitorAlertManagerPoolList", getMonitorAlertManagerPoolList)
		monitorApiGroup.POST("/createMonitorAlertManagerPool", createMonitorAlertManagerPool)
		monitorApiGroup.POST("/updateMonitorAlertManagerPool", updateMonitorAlertManagerPool)
		monitorApiGroup.DELETE("/deleteMonitorAlertManagerPool/:id", deleteMonitorAlertManagerPool)
		monitorApiGroup.GET("/getMonitorAlertManagerYamlOne", getMonitorAlertManagerYamlOne)

		// alertManager 发送组
		monitorApiGroup.GET("/getMonitorAlertManagerSendGroupList", getMonitorAlertManagerSendGroupList)
		monitorApiGroup.POST("/createMonitorAlertManagerSendGroup", createMonitorAlertManagerSendGroup)
		monitorApiGroup.POST("/updateMonitorAlertManagerSendGroup", updateMonitorAlertManagerSendGroup)
		monitorApiGroup.DELETE("/deleteMonitorAlertManagerSendGroup/:id", deleteMonitorAlertManagerSendGroup)
		monitorApiGroup.POST("/setAlertManagerSendGroupStatus", setAlertManagerSendGroupStatus)

		// alertManager 告警事件
		monitorApiGroup.GET("/getMonitorAlertManagerEventList", getMonitorAlertManagerEventList)
		monitorApiGroup.POST("/alertManagerEventSilence/:id", alertManagerEventSilence)
		monitorApiGroup.POST("/alertManagerEventUnSilence/:id", alertManagerEventUnSilence)
		monitorApiGroup.POST("/alertManagerEventBatchSilence", alertManagerEventBatchSilence)
		monitorApiGroup.POST("/alertManagerEventBatchUnSilence", alertManagerEventBatchUnSilence)
		monitorApiGroup.POST("/alertManagerEventReLing/:id", alertManagerEventReLing)

		// 值班组
		monitorApiGroup.GET("/getMonitorOndutyGroupList", getMonitorOndutyGroupList)
		monitorApiGroup.GET("/getMonitorOndutyGroupOne/:id", getMonitorOndutyGroupOne)
		monitorApiGroup.POST("/createMonitorOndutyGroup", createMonitorOndutyGroup)
		monitorApiGroup.POST("/updateMonitorOndutyGroup", updateMonitorOndutyGroup)
		monitorApiGroup.DELETE("/deleteMonitorOndutyGroup/:id", deleteMonitorOndutyGroup)
		monitorApiGroup.POST("/setMonitorOndutyStatus", setMonitorOndutyStatus)
		monitorApiGroup.GET("/getMonitorOndutyGroupFuturePlan/:id", getMonitorOndutyGroupFuturePlan)
		monitorApiGroup.POST("/createMonitorOndutyChange", createMonitorOndutyChange)
	}

	K8sGroup := afterLoginApiGroup.Group("/k8s")
	{
		K8sGroup.GET("/getK8sClusterList", getK8sClusterList)
		K8sGroup.GET("/getClusterForSelect", getClusterForSelect)
		K8sGroup.POST("/createK8sCluster", createK8sCluster)
		K8sGroup.POST("/updateK8sCluster", updateK8sCluster)
		K8sGroup.DELETE("/deleteK8sCluster/:id", deleteK8sCluster)
		K8sGroup.DELETE("/deleteK8sClusterBatch", deleteK8sClusterBatch)

		K8sGroup.GET("/getK8sNodeList", getK8sNodeList)
		K8sGroup.POST("/scheduleEnableSwitchK8sNodesOne", scheduleEnableSwitchK8sNodesOne)
		K8sGroup.POST("/labelK8sNodes", labelK8sNodes)
		K8sGroup.POST("/taintK8sNodes", taintK8sNodes)
		K8sGroup.POST("/drainK8sNodes", drainK8sNodes)
		K8sGroup.GET("/getPodListByNodeName", getPodListByNodeName)
	}

	CodeGroup := afterLoginApiGroup.Group("/code")
	{
		CodeGroup.GET("/getCodeGitServerList", getCodeGitServerList)
		CodeGroup.POST("/createCodeGitServer", createCodeGitServer)
		CodeGroup.POST("/updateCodeGitServer", updateCodeGitServer)
		CodeGroup.DELETE("/deleteCodeGitServer/:id", deleteCodeGitServer)
		CodeGroup.POST("/pingCodeGitServer", pingCodeGitServer)

		CodeGroup.GET("/getCodeGitRepoList", getCodeGitRepoList)
		CodeGroup.POST("/createCodeGitRepo", createCodeGitRepo)
		CodeGroup.POST("/updateCodeGitRepo", updateCodeGitRepo)

		CodeGroup.GET("/getGitNamespaces", getGitNamespaces)      // 命名空间管理 (Group / Organization)
		CodeGroup.POST("/createGitNamespace", createGitNamespace) // 创建命名空间
		CodeGroup.POST("/updateGitNamespace", updateGitNamespace) // 更新命名空间

		CodeGroup.GET("/getGitUsers", getGitUsers)      // 远端系统用户列表
		CodeGroup.POST("/createGitUser", createGitUser) // 创建系统用户
		CodeGroup.POST("/updateGitUser", updateGitUser) // 更新系统用户

		CodeGroup.GET("/getRepoMembers", getRepoMembers) // 获取成员
		CodeGroup.POST("/addRepoMember", addRepoMember)  // 添加/更新成员
		CodeGroup.DELETE("/removeRepoMember", removeRepoMember)
		CodeGroup.GET("/getRepoBranches", getRepoBranches)

		CodeGroup.GET("/getMergeRequests", getMergeRequests)
		CodeGroup.POST("/createMergeRequest", createMergeRequest)
		CodeGroup.POST("/mergeMergeRequest", mergeMergeRequest)
		CodeGroup.POST("/closeMergeRequest", closeMergeRequest)

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
