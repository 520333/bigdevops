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
	base.POST("/login", middleware.AuditLogMiddleWare(), UserLogin)
	base.GET("/logout", middleware.AuditLogMiddleWare(), UserLogout)
	base.GET("/auth/oidc/login", GetOidcLoginUrl) // sso单点登录
	base.POST("/auth/oidc/callback", OidcCallback)
	base.GET("/auth/dingtalk/login", GetDingTalkLoginUrl) // 钉钉单点登录
	base.POST("/auth/dingtalk/callback", DingTalkCallback)

	noAuth := r.Group("/noAuth")
	{
		noAuth.GET("/downloadPrometheusMainConfigYaml", downloadPrometheusMainConfigYaml) //prometheus主配置文件
		noAuth.GET("/downloadPrometheusAlertRuleMainConfigYaml", downloadPrometheusAlertRuleMainConfigYaml)
		noAuth.GET("/downloadPrometheusRecordRuleMainConfigYaml", downloadPrometheusRecordRuleMainConfigYaml)
		noAuth.GET("/downloadAlertManagerMainConfigYaml", downloadAlertManagerMainConfigYaml) //alertManager主配置文件
		noAuth.GET("/getLeafStreeNodeBindIps", getLeafStreeNodeBindIps)
		noAuth.GET("/getDnsBlackboxTargets", getDnsBlackboxTargets)
		noAuth.GET("/getMonitorOndutyGroupFuturePlan/:id", getMonitorOndutyGroupFuturePlan)
		noAuth.GET("/platform/telemetry", GetPlatformTelemetry) // 登录大盘公开遥测数据
	}

	// 以下开始需要认证
	afterLoginApiGroup := r.Group("/api")
	afterLoginApiGroup.
		Use(middleware.JWTAuthMiddleWare()).    // jwt中间件
		Use(middleware.UserStatusMiddleware()). // 用户状态中间件
		Use(middleware.CasBinRbacMiddleware()). // rbac	中间件
		Use(middleware.AuditLogMiddleWare())    // 审计日志中间件

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
		systemApiGroup.POST("/updateUserInfo", updateUserInfo)
		systemApiGroup.POST("/uploadAvatar", uploadAvatar)
		systemApiGroup.GET("/getAllUserAndRoles", getAllUserAndRoles)
		systemApiGroup.GET("/getOnlineUserList", getOnlineUserList)
		systemApiGroup.POST("/kickoutUser", kickoutUser)

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

		// 审计日志路由
		systemApiGroup.GET("/getAuditLogList", getAuditLogList)
		systemApiGroup.GET("/getLoginLogList", getLoginLogList)
	}

	artifactoryApiGroup := afterLoginApiGroup.Group("/artifactory")
	{
		artifactoryApiGroup.GET("/repos", getArtifactoryRepositories)
		artifactoryApiGroup.GET("/info", getArtifactoryFileInfo)
		artifactoryApiGroup.GET("/tree", getArtifactoryFileTree)
		artifactoryApiGroup.GET("/content", getArtifactoryFileContent)
		artifactoryApiGroup.POST("/save", saveArtifactoryFileContent)
		artifactoryApiGroup.POST("/upload", uploadArtifactoryFile)
		artifactoryApiGroup.POST("/delete", deleteArtifactoryFile)
		artifactoryApiGroup.GET("/download", downloadArtifactoryFile)
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

		// DNS
		sTreeApiGroup.GET("/getResourceDnsUnbindList", getResourceDnsUnbindList)
		sTreeApiGroup.POST("/bindDnsToStreeNode", bindDnsToStreeNode)
		sTreeApiGroup.POST("/unBindDnsToStreeNode", unBindDnsToStreeNode)

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
		workOrDerApiGroup.GET("/getNotificationList", getWorkOrderNotificationList)
		workOrDerApiGroup.POST("/markNotifyRead", markWorkOrderNotifyRead)
		workOrDerApiGroup.POST("/clearNotifyTab", clearWorkOrderNotifyTab)
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
		K8sGroup.GET("/getK8sNamespaceList", getK8sNamespaceList)
		K8sGroup.GET("/getK8sPodList", getK8sPodList)
		K8sGroup.GET("/getK8sPodYaml", getK8sPodYaml)
		K8sGroup.POST("/createK8sPod", createK8sPod)
		K8sGroup.POST("/updateK8sPod", updateK8sPod)
		K8sGroup.POST("/deleteK8sPod", deleteK8sPod)
		K8sGroup.POST("/deleteK8sPodBatch", deleteK8sPodBatch)
		K8sGroup.GET("/getK8sPodLogs", getK8sPodLogs)
		K8sGroup.GET("/wsK8sPodExec", wsK8sPodExec)
		K8sGroup.GET("/wsK8sPodWatch", wsK8sPodWatch)
		K8sGroup.GET("/wsK8sPodLogs", wsK8sPodLogs)
		K8sGroup.GET("/downloadK8sPodFile", downloadK8sPodFile)
		K8sGroup.GET("/getK8sPodFileList", getK8sPodFileList)
		K8sGroup.POST("/uploadK8sPodFile", uploadK8sPodFile)
		K8sGroup.POST("/deleteK8sPodFile", deleteK8sPodFile)
		K8sGroup.GET("/readK8sPodFileContent", readK8sPodFileContent)
		K8sGroup.POST("/saveK8sPodFileContent", saveK8sPodFileContent)

		// 🚀 K8s Deployment 控制器管理
		K8sGroup.GET("/getK8sDeploymentList", getK8sDeploymentList)
		K8sGroup.GET("/getK8sDeploymentYaml", getK8sDeploymentYaml)
		K8sGroup.POST("/createK8sDeployment", createK8sDeployment)
		K8sGroup.POST("/updateK8sDeployment", updateK8sDeployment)
		K8sGroup.POST("/scaleK8sDeployment", scaleK8sDeployment)
		K8sGroup.POST("/restartK8sDeployment", restartK8sDeployment)
		K8sGroup.POST("/deleteK8sDeployment", deleteK8sDeployment)
		K8sGroup.POST("/deleteK8sDeploymentBatch", deleteK8sDeploymentBatch)
		K8sGroup.GET("/wsK8sDeploymentWatch", wsK8sDeploymentWatch)

		// 🚀 K8s StatefulSet 控制器管理
		K8sGroup.GET("/getK8sStatefulSetList", getK8sStatefulSetList)
		K8sGroup.GET("/getK8sStatefulSetYaml", getK8sStatefulSetYaml)
		K8sGroup.POST("/createK8sStatefulSet", createK8sStatefulSet)
		K8sGroup.POST("/updateK8sStatefulSet", updateK8sStatefulSet)
		K8sGroup.POST("/scaleK8sStatefulSet", scaleK8sStatefulSet)
		K8sGroup.POST("/restartK8sStatefulSet", restartK8sStatefulSet)
		K8sGroup.POST("/deleteK8sStatefulSet", deleteK8sStatefulSet)
		K8sGroup.POST("/deleteK8sStatefulSetBatch", deleteK8sStatefulSetBatch)

		// 🚀 K8s DaemonSet 控制器管理
		K8sGroup.GET("/getK8sDaemonSetList", getK8sDaemonSetList)
		K8sGroup.GET("/getK8sDaemonSetYaml", getK8sDaemonSetYaml)
		K8sGroup.POST("/createK8sDaemonSet", createK8sDaemonSet)
		K8sGroup.POST("/updateK8sDaemonSet", updateK8sDaemonSet)
		K8sGroup.POST("/restartK8sDaemonSet", restartK8sDaemonSet)
		K8sGroup.POST("/deleteK8sDaemonSet", deleteK8sDaemonSet)
		K8sGroup.POST("/deleteK8sDaemonSetBatch", deleteK8sDaemonSetBatch)

		// 🚀 K8s ConfigMap 配置字典
		K8sGroup.GET("/getK8sConfigMapList", getK8sConfigMapList)
		K8sGroup.GET("/getK8sConfigMapYaml", getK8sConfigMapYaml)
		K8sGroup.POST("/createK8sConfigMap", createK8sConfigMap)
		K8sGroup.POST("/updateK8sConfigMap", updateK8sConfigMap)
		K8sGroup.POST("/deleteK8sConfigMap", deleteK8sConfigMap)
		K8sGroup.POST("/deleteK8sConfigMapBatch", deleteK8sConfigMapBatch)

		// 🚀 K8s Secret 密钥凭据
		K8sGroup.GET("/getK8sSecretList", getK8sSecretList)
		K8sGroup.GET("/getK8sSecretYaml", getK8sSecretYaml)
		K8sGroup.POST("/createK8sSecret", createK8sSecret)
		K8sGroup.POST("/updateK8sSecret", updateK8sSecret)
		K8sGroup.POST("/deleteK8sSecret", deleteK8sSecret)
		K8sGroup.POST("/deleteK8sSecretBatch", deleteK8sSecretBatch)

		// 🚀 K8s Service 服务
		K8sGroup.GET("/getK8sServiceList", getK8sServiceList)
		K8sGroup.GET("/getK8sServiceYaml", getK8sServiceYaml)
		K8sGroup.POST("/createK8sService", createK8sService)
		K8sGroup.POST("/updateK8sService", updateK8sService)
		K8sGroup.POST("/deleteK8sService", deleteK8sService)
		K8sGroup.POST("/deleteK8sServiceBatch", deleteK8sServiceBatch)

		// 🚀 K8s Ingress 路由
		K8sGroup.GET("/getK8sIngressList", getK8sIngressList)
		K8sGroup.GET("/getK8sIngressYaml", getK8sIngressYaml)
		K8sGroup.POST("/createK8sIngress", createK8sIngress)
		K8sGroup.POST("/updateK8sIngress", updateK8sIngress)
		K8sGroup.POST("/deleteK8sIngress", deleteK8sIngress)
		K8sGroup.POST("/deleteK8sIngressBatch", deleteK8sIngressBatch)

		// 🚀 K8s YAML 模板管理
		K8sGroup.GET("/getK8sYamlTemplateList", getK8sYamlTemplateList)
		K8sGroup.POST("/createK8sYamlTemplate", createK8sYamlTemplate)
		K8sGroup.POST("/updateK8sYamlTemplate", updateK8sYamlTemplate)
		K8sGroup.DELETE("/deleteK8sYamlTemplate/:id", deleteK8sYamlTemplate)

		// 🚀 K8s YAML 任务发布管理
		K8sGroup.GET("/getK8sYamlTaskList", getK8sYamlTaskList)
		K8sGroup.POST("/createK8sYamlTask", createK8sYamlTask)
		K8sGroup.POST("/updateK8sYamlTask", updateK8sYamlTask)
		K8sGroup.DELETE("/deleteK8sYamlTask/:id", deleteK8sYamlTask)
		K8sGroup.POST("/applyK8sYamlTaskOne/:id", applyK8sYamlTaskOne)
		K8sGroup.GET("/getK8sYamlTaskLogList", getK8sYamlTaskLogList)

		// 🚀 K8s 项目、应用与实例管理
		K8sGroup.GET("/getK8sProjectList", getK8sProjectList)
		K8sGroup.GET("/getK8sProjectOne/:id", getK8sProjectOne)
		K8sGroup.POST("/createK8sProject", createK8sProject)
		K8sGroup.POST("/updateK8sProject", updateK8sProject)
		K8sGroup.DELETE("/deleteK8sProject/:id", deleteK8sProject)

		K8sGroup.GET("/getK8sAppList", getK8sAppList)
		K8sGroup.GET("/getK8sAppOne/:id", getK8sAppOne)
		K8sGroup.POST("/createK8sApp", createK8sApp)
		K8sGroup.POST("/updateK8sApp", updateK8sApp)
		K8sGroup.DELETE("/deleteK8sApp/:id", deleteK8sApp)

		K8sGroup.GET("/getK8sInstanceList", getK8sInstanceList)
		K8sGroup.GET("/getK8sInstanceOne/:id", getK8sInstanceOne)
		K8sGroup.POST("/createK8sInstance", createK8sInstance)
		K8sGroup.POST("/updateK8sInstance", updateK8sInstance)
		K8sGroup.DELETE("/deleteK8sInstance/:id", deleteK8sInstance)
		K8sGroup.POST("/deployK8sInstance/:id", deployK8sInstance)
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

	JenkinsGroup := afterLoginApiGroup.Group("/cicd")
	{
		// 实例管理路由
		JenkinsGroup.GET("/getJenkinsInstanceList", getJenkinsInstanceList)
		JenkinsGroup.POST("/createJenkinsInstance", createJenkinsInstance)
		JenkinsGroup.POST("/updateJenkinsInstance", updateJenkinsInstance)
		JenkinsGroup.DELETE("/deleteJenkinsInstance", deleteJenkinsInstance)

		// Job 管理路由
		JenkinsGroup.GET("/getJenkinsJobList", getJenkinsJobList)
		JenkinsGroup.POST("/createJenkinsJob", createJenkinsJob)
		JenkinsGroup.POST("/updateJenkinsJob", updateJenkinsJob)
		JenkinsGroup.DELETE("/deleteJenkinsJob", deleteJenkinsJob)
		JenkinsGroup.POST("/triggerJenkinsBuild", triggerJenkinsBuild)
		JenkinsGroup.POST("/stopJenkinsBuild", stopJenkinsBuild)
		JenkinsGroup.GET("/getJenkinsBuildLogs", getJenkinsBuildLogs)
		JenkinsGroup.GET("/getJenkinsJobRemotePipeline", getJenkinsJobRemotePipeline)
		JenkinsGroup.GET("/getJenkinsJobStageView", getJenkinsJobStageView)
		JenkinsGroup.POST("/toggleJenkinsJobDeleteLock", toggleJenkinsJobDeleteLock)

		// Pipeline 模版与 Stage 配置路由
		JenkinsGroup.GET("/getJenkinsPipelineList", getJenkinsPipelineList)
		JenkinsGroup.POST("/createJenkinsPipeline", createJenkinsPipeline)
		JenkinsGroup.POST("/updateJenkinsPipeline", updateJenkinsPipeline)
		JenkinsGroup.DELETE("/deleteJenkinsPipeline", deleteJenkinsPipeline)
		JenkinsGroup.POST("/validateJenkinsPipeline", validateJenkinsPipeline)

		// 独立 Stage 模块路由
		JenkinsGroup.GET("/getJenkinsStageList", getJenkinsStageList)
		JenkinsGroup.POST("/createJenkinsStage", createJenkinsStage)
		JenkinsGroup.POST("/updateJenkinsStage", updateJenkinsStage)
		JenkinsGroup.DELETE("/deleteJenkinsStage", deleteJenkinsStage)

		// 独立 环境变量 路由
		JenkinsGroup.GET("/getJenkinsEnvList", getJenkinsEnvList)
		JenkinsGroup.POST("/createJenkinsEnv", createJenkinsEnv)
		JenkinsGroup.POST("/updateJenkinsEnv", updateJenkinsEnv)
		JenkinsGroup.DELETE("/deleteJenkinsEnv", deleteJenkinsEnv)

		// 独立 构建参数 路由
		JenkinsGroup.GET("/getJenkinsParamList", getJenkinsParamList)
		JenkinsGroup.POST("/createJenkinsParam", createJenkinsParam)
		JenkinsGroup.POST("/updateJenkinsParam", updateJenkinsParam)
		JenkinsGroup.DELETE("/deleteJenkinsParam", deleteJenkinsParam)
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
