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
)

var COMMON_SHOW_MAP = map[string]bool{
	"1": true,
	"0": false,
}
