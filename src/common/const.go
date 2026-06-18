package common

const (
	GIN_CTX_CONFIG_LOGGER = "gin_logger"
	GIN_CTX_CONFIG_CONFIG = "gin_config"
	GIN_CTX_JWT_CLAIM     = "jwt_claim"
	GIN_CTX_JWT_USER_NAME = "jwt_user_name"
	COMMON_STATUS_ENABLE  = "1"
	COMMON_STATUS_DISABLE = "0"
	RESOURCE_TYPE_ECS     = "ecs"
	RESOURCE_TYPE_ELB     = "elb"
	RESOURCE_TYPE_RDS     = "rds"
	RESOURCE_TYPE_DNS     = "dns"

	AGENT_VAR_ENV     = "VAR_ENV"
	AGENT_VERSION     = "1.0"
	ERR_ECS_NOT_FOUND = "ResourceEcs不存在"

	// 任务本地状态
	AGENT_TASK_STATUS_RUNNING = "running"
	AGENT_TASK_STATUS_KILLED  = "killed"
	AGENT_TASK_STATUS_SUCCESS = "success"
	AGENT_TASK_STATUS_FAILED  = "failed"

	// 任务执行动作
	AGENT_TASK_ACTION_START = "start"
	AGENT_TASK_ACTION_KILL  = "kill"
	AGENT_TASK_ACTION_STOP  = "stop"

	AGENT_TASK_EXEC_SHELL   = "shell"
	AGENT_TASK_EXEC_PYTHON  = "python"
	AGENT_TASK_EXEC_ANSIBLE = "ansible"
)

var (
	COMMON_SHOW_MAP = map[string]bool{
		"1": true,
		"0": false,
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
