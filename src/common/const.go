package common

const (
	GIN_CTX_CONFIG_LOGGER        = "gin_logger"
	GIN_CTX_CONFIG_CONFIG        = "gin_config"
	GIN_CTX_CONFIG_ALERTRECEIVEQ = "alertReceiveQ"
	GIN_CTX_MONITOR_CACHE        = "monitor_cache"
	GIN_CTX_K8S_CACHE            = "k8s_cache"
	GIN_CTX_JENKINS_CACHE        = "jenkins_cache"
	GIN_CTX_JWT_CLAIM            = "jwt_claim"
	GIN_CTX_JWT_USER_NAME        = "jwt_user_name"
	COMMON_STATUS_ENABLE         = "1"
	COMMON_STATUS_DISABLE        = "0"

	RESOURCE_TYPE_ECS = "ecs"
	RESOURCE_TYPE_ELB = "elb"
	RESOURCE_TYPE_RDS = "rds"
	RESOURCE_TYPE_DNS = "dns"

	AGENT_VAR_ENV     = "VAR_ENV"
	AGENT_VERSION     = "1.0"
	ERR_ECS_NOT_FOUND = "ResourceEcs不存在"

	// 任务本地状态
	AGENT_TASK_STATUS_RUNNING = "running"
	AGENT_TASK_STATUS_KILLED  = "killed"
	AGENT_TASK_STATUS_SUCCESS = "success"
	AGENT_TASK_STATUS_FAILED  = "failed"

	// 任务执行动作
	AGENT_TASK_ACTION_START  = "start"
	AGENT_TASK_ACTION_KILL   = "kill"
	AGENT_TASK_ACTION_STOP   = "stop"
	AGENT_TASK_ACTION_PAUSE  = "pause"
	AGENT_TASK_ACTION_RESUME = "resume"

	AGENT_TASK_EXEC_SHELL   = "shell"
	AGENT_TASK_EXEC_PYTHON  = "python"
	AGENT_TASK_EXEC_ANSIBLE = "ansible"

	//job的状态
	JOB_STATUS_PENDING  = "pending"
	JOB_STATUS_RUNNING  = "running"
	JOB_STATUS_PAUSED   = "paused"
	JOB_STATUS_KILLED   = "killed"
	JOB_STATUS_KILLING  = "killing"
	JOB_STATUS_FINISHED = "finished"

	// job的执行错误策略
	JOB_ONERROR_STRATEGY_PAUSE  = "pause"
	JOB_ONERROR_STRATEGY_IGNORE = "ignore"
	JOB_ONERROR_STRATEGY_STOP   = "stop"

	// 监控采集器的服务类型
	MONITOR_SCRAPE_JOB_SD_TYPE_K8S          = "kubernetes"
	MONITOR_SCRAPE_JOB_SD_TYPE_HTTP         = "http"
	MONITOR_SCRAPE_JOB_SD_TYPE_BLACKBOX_DNS = "blackbox_dns"

	MONITOR_ALERT_MATCH_KEY     = "alert_send_group"
	MONITOR_ALERT_RULE_KEY      = "alert_rule_id"
	MONITOR_ALERT_NAME_KEY      = "alertname"
	MONITOR_ALERT_SEVERITY_KEY  = "severity"
	MONITOR_ALERT_BIND_NODE_KEY = "bind_stree_node"

	MONITOR_ALERT_SEVERITY_CRITICAL = "critical"
	MONITOR_ALERT_SEVERITY_WARNING  = "warning"
	MONITOR_ALERT_SEVERITY_INFO     = "info"
	MONITOR_ALERT_RULE_ANNO_VALUE   = "description_value"

	MONITOR_ALERT_STATUS_FIRING    = "firing"
	MONITOR_ALERT_STATUS_RESOLVED  = "resolved"
	MONITOR_ALERT_STATUS_RENLING   = "renling"
	MONITOR_ALERT_STATUS_UPGRADED  = "upgraded"
	MONITOR_ALERT_STATUS_SILIENCED = "silenced"

	GORM_ENABLE_RES_YES = 1
	GORM_ENABLE_RES_NO  = 2

	// k8s集群环境
	RUN_ENV_TYPE_PROD  = "prod"
	RUN_ENV_TYPE_STAGE = "stage"
	RUN_ENV_TYPE_TEST  = "test"
	RUN_ENV_TYPE_DEV   = "dev"
	RUN_ENV_TYPE_PRESS = "press"

	LabelNodeRolePrefix = "node-role.kubernetes.io/"
	NodeLabelRole       = "kubernetes.io/role"

	K8S_YAMLTASK_STATUS_PENDING = "pending"
	K8S_YAMLTASK_STATUS_APPLIED = "applied"
	K8S_YAMLTASK_STATUS_FAILED  = "failed"
)

var (
	MONITOR_ALERT_STATUS_ARRAY = []string{
		MONITOR_ALERT_STATUS_FIRING,
		MONITOR_ALERT_STATUS_RESOLVED,
		MONITOR_ALERT_STATUS_UPGRADED,
		MONITOR_ALERT_STATUS_SILIENCED,
	}
	RUN_ENV_TYPE_ARRAY = []string{
		RUN_ENV_TYPE_PROD,
		RUN_ENV_TYPE_STAGE,
		RUN_ENV_TYPE_TEST,
		RUN_ENV_TYPE_DEV,
		RUN_ENV_TYPE_PRESS,
	}
	MONITOR_ALERT_SEVERITY_TITLE_COLOR_MAP = map[string]string{
		MONITOR_ALERT_SEVERITY_CRITICAL: "red",
		MONITOR_ALERT_SEVERITY_WARNING:  "yellow",
		MONITOR_ALERT_SEVERITY_INFO:     "blue",
	}
	MONITOR_ALERT_STATUS_CH_MAP = map[string]string{
		MONITOR_ALERT_STATUS_FIRING:   "触发中",
		MONITOR_ALERT_STATUS_RESOLVED: "已恢复",
	}
	MONITOR_ALERT_STATUS_COLOR_MAP = map[string]string{
		MONITOR_ALERT_STATUS_FIRING:   "red",
		MONITOR_ALERT_STATUS_RESOLVED: "green",
	}
	COMMON_SHOW_MAP = map[string]bool{
		"1": true,
		"0": false,
	}
	JOB_ACTION_NEXT_STATUS_MAP = map[string]string{
		AGENT_TASK_ACTION_START:  JOB_STATUS_RUNNING,
		AGENT_TASK_ACTION_KILL:   JOB_STATUS_KILLING,
		AGENT_TASK_ACTION_PAUSE:  JOB_STATUS_PAUSED,
		AGENT_TASK_ACTION_RESUME: JOB_STATUS_RUNNING,
		AGENT_TASK_ACTION_STOP:   JOB_STATUS_FINISHED,
	}

	FLOW_TYPE_APPROVAL = "Approval"
	FLOW_TYPE_ACTION   = "Action"
	//FLOW_TYPE_MAP      = map[string]string{
	//	FLOW_TYPE_APPROVAL: "审批节点",
	//	FLOW_TYPE_ACTION:   "执行节点",
	//	"Start":            "开始节点",
	//	"Stop":             "结束节点",
	//}

	FLOW_TYPE_MAP = map[string]string{
		"起始节点": "起始节点",
		"审批节点": "审批节点",
		"执行节点": "执行节点",
		"结束节点": "结束节点",
	}

	ApprovalActionPass   = "pass"
	ApprovalActionReject = "reject"
	ApprovalActionMap    = map[string]string{
		ApprovalActionPass:   "",
		ApprovalActionReject: "",
	}

	WORKORDER_INSTANCE_QUERYMODE_MINE     = "mine"
	WORKORDER_INSTANCE_QUERYMODE_ALL      = "all"
	WORKORDER_INSTANCE_QUERYMODE_APPROVAL = FLOW_TYPE_APPROVAL
	WORKORDER_INSTANCE_QUERYMODE_ACTION   = FLOW_TYPE_ACTION

	WORKORDER_INSTANCE_PENDINGAPPROVAL = "pendingApproval"
	WORKORDER_INSTANCE_APPROVAL_REJECT = "approvalReject"
	WORKORDER_INSTANCE_PENDING_ACTION  = "pendingAction"
	WORKORDER_INSTANCE_FINISHED        = "finished"
)
