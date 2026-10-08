package models

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// JenkinsJobPermission 服务基线数据权限表（基于角色组与项目/环境绑定）
type JenkinsJobPermission struct {
	Model
	RoleID      uint   `json:"roleId" gorm:"index:idx_role_proj;not null;comment:关联角色ID"`
	ProjectName string `json:"projectName" gorm:"index:idx_role_proj;type:varchar(128);not null;comment:项目空间名称(*表示全部)"`
	DeployEnv   string `json:"deployEnv" gorm:"type:varchar(128);not null;comment:允许的环境列表(dev,test或*)"`
	AllowBuild  int    `json:"allowBuild" gorm:"default:1;comment:是否允许触发构建(1允许 2只读)"`
}

func (JenkinsJobPermission) TableName() string {
	return "jenkins_job_permission"
}

// GetPermissionsByRoleIDs 查询指定角色列表的所有权限规则
func GetPermissionsByRoleIDs(roleIDs []uint) ([]*JenkinsJobPermission, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var list []*JenkinsJobPermission
	err := Db.Where("role_id IN ?", roleIDs).Find(&list).Error
	return list, err
}

// GetPermissionsByRoleID 查询单个角色的权限规则
func GetPermissionsByRoleID(roleID uint) ([]*JenkinsJobPermission, error) {
	var list []*JenkinsJobPermission
	err := Db.Where("role_id = ?", roleID).Find(&list).Error
	return list, err
}

// IsUserSuperRole 判断用户是否拥有超级管理员角色
func IsUserSuperRole(user *SystemUser, superRoleName string) bool {
	if user == nil {
		return false
	}
	if superRoleName == "" {
		superRoleName = "super"
	}
	for _, role := range user.Roles {
		if role != nil && (strings.EqualFold(role.RoleValue, superRoleName) || strings.EqualFold(role.RoleName, superRoleName)) {
			return true
		}
	}
	return false
}

// FilterJobsByPermission 根据用户角色权限对查询构建物理行级过滤
// 智能兼容：既支持规范的独立 project_name 和 deploy_env 字段匹配，
// 又完美支持类似 card-test-admin-bin、card-prod-server-bin 的服务名内嵌特征匹配！
// 返回 (query, hasAccess)
// 若 hasAccess 为 false，表示非超管且未被授予任何可见范围，上层可直接返回空列表([])
func FilterJobsByPermission(query *gorm.DB, user *SystemUser, superRoleName string) (*gorm.DB, bool) {
	// 1. 超级管理员：拥有最高权限，无条件展示所有基线作业
	if IsUserSuperRole(user, superRoleName) {
		return query, true
	}

	if user == nil || len(user.Roles) == 0 {
		return query, false
	}

	var roleIDs []uint
	for _, r := range user.Roles {
		if r != nil && r.ID > 0 {
			roleIDs = append(roleIDs, r.ID)
		}
	}
	if len(roleIDs) == 0 {
		return query, false
	}

	// 2. 查出该用户所有角色绑定的权限规则
	perms, err := GetPermissionsByRoleIDs(roleIDs)
	if err != nil || len(perms) == 0 {
		return query, false
	}

	// 3. 解析规则集合（多角色取并集）
	projEnvMap := make(map[string]map[string]bool)
	hasGlobalWildcard := false

	for _, p := range perms {
		proj := strings.TrimSpace(p.ProjectName)
		if proj == "" {
			continue
		}
		rawEnvs := strings.Split(p.DeployEnv, ",")

		if proj == "*" {
			for _, e := range rawEnvs {
				e = strings.TrimSpace(e)
				if e == "*" {
					hasGlobalWildcard = true
					break
				}
			}
		}

		if projEnvMap[proj] == nil {
			projEnvMap[proj] = make(map[string]bool)
		}
		for _, e := range rawEnvs {
			e = strings.TrimSpace(e)
			if e != "" {
				projEnvMap[proj][e] = true
			}
		}
	}

	// 若存在通配所有项目及环境的全局规则，直接放行
	if hasGlobalWildcard {
		return query, true
	}

	if len(projEnvMap) == 0 {
		return query, false
	}

	// 4. 组装物理 SQL 过滤条件（智能兼容规范字段 + 任务名命名特征如 card-test-admin-bin）
	subQuery := Db.Where("1 = 0") // 初始假条件，以便后续使用 Or 串联各项目的并集规则

	for proj, envMap := range projEnvMap {
		// A. 构造项目匹配条件
		var projCondition string
		var projArgs []interface{}
		if proj == "*" {
			projCondition = "1 = 1"
		} else {
			// 精确字段优先：若已有明确的 project_name，则严格比对 project_name；仅当为空时才通过任务名或前缀兜底
			projCondition = "((project_name != '' AND project_name IS NOT NULL AND project_name = ?) OR ((project_name = '' OR project_name IS NULL) AND (name = ? OR name LIKE ? OR name LIKE ?)))"
			projArgs = []interface{}{proj, proj, proj + "-%", proj + "_%"}
		}

		// B. 构造环境匹配条件
		var envCondition string
		var envArgs []interface{}
		if envMap["*"] {
			envCondition = "1 = 1"
		} else {
			var envs []string
			var envLikeClauses []string
			var envLikeArgs []interface{}

			for e := range envMap {
				envs = append(envs, e)
				// 匹配服务名内嵌环境特征：-test- / -test / test- / _test_ / _test
				envLikeClauses = append(envLikeClauses, "(name LIKE ? OR name LIKE ? OR name LIKE ? OR name LIKE ? OR name LIKE ?)")
				envLikeArgs = append(envLikeArgs,
					"%-"+e+"-%",
					"%-"+e,
					e+"-%",
					"%_"+e+"_%",
					"%_"+e,
				)
			}

			if len(envs) > 0 {
				// 1. 精确匹配分支：deploy_env 字段有明确值时，必须严格在授权列表中
				strictPart := "(deploy_env != '' AND deploy_env IS NOT NULL AND deploy_env IN (?))"
				var envConditionArgs []interface{}
				envConditionArgs = append(envConditionArgs, envs)

				// 2. 兜底匹配分支：仅当 deploy_env 为空或未设置时，才允许通过服务名特征推断
				if len(envLikeClauses) > 0 {
					fallbackPart := "((deploy_env = '' OR deploy_env IS NULL) AND (" + strings.Join(envLikeClauses, " OR ") + "))"
					envCondition = "(" + strictPart + " OR " + fallbackPart + ")"
					envConditionArgs = append(envConditionArgs, envLikeArgs...)
				} else {
					envCondition = strictPart
				}

				envArgs = envConditionArgs
			}
		}

		// C. 组合项目与环境：(项目匹配 AND 环境匹配)
		if projCondition != "" && envCondition != "" {
			combinedSql := fmt.Sprintf("(%s AND %s)", projCondition, envCondition)
			var combinedArgs []interface{}
			combinedArgs = append(combinedArgs, projArgs...)
			combinedArgs = append(combinedArgs, envArgs...)
			subQuery = subQuery.Or(combinedSql, combinedArgs...)
		}
	}

	return query.Where(subQuery), true
}

// CheckJobOperationPermission 校验用户对特定 Job 的项目和环境是否拥有操作或构建权限
// 支持传入 jobName，精确字段优先，当字段为空时兜底兼容 card-test-admin-bin、card-prod-server-bin 等内嵌命名规则！
func CheckJobOperationPermission(user *SystemUser, superRoleName, projectName, deployEnv, jobName string, requireBuild bool) bool {
	// 1. 超管无条件允许
	if IsUserSuperRole(user, superRoleName) {
		return true
	}
	if user == nil || len(user.Roles) == 0 {
		return false
	}

	var roleIDs []uint
	for _, r := range user.Roles {
		if r != nil && r.ID > 0 {
			roleIDs = append(roleIDs, r.ID)
		}
	}
	if len(roleIDs) == 0 {
		return false
	}

	perms, err := GetPermissionsByRoleIDs(roleIDs)
	if err != nil || len(perms) == 0 {
		return false
	}

	projectName = strings.TrimSpace(projectName)
	deployEnv = strings.TrimSpace(deployEnv)
	jobName = strings.TrimSpace(jobName)

	for _, p := range perms {
		// 校验构建权限 (1=允许 2=只读)
		if requireBuild && p.AllowBuild != 1 {
			continue
		}

		// A. 校验项目空间匹配
		pProj := strings.TrimSpace(p.ProjectName)
		projMatch := false
		if pProj == "*" {
			projMatch = true
		} else if projectName != "" {
			// 精确字段优先
			projMatch = strings.EqualFold(pProj, projectName)
		} else {
			// projectName 为空时兜底匹配任务名及前缀
			if strings.EqualFold(pProj, jobName) ||
				(jobName != "" && (strings.HasPrefix(strings.ToLower(jobName), strings.ToLower(pProj)+"-") || strings.HasPrefix(strings.ToLower(jobName), strings.ToLower(pProj)+"_"))) {
				projMatch = true
			}
		}

		if !projMatch {
			continue
		}

		// B. 校验部署环境匹配
		envs := strings.Split(p.DeployEnv, ",")
		for _, e := range envs {
			e = strings.TrimSpace(e)
			if e == "*" {
				return true
			}
			if deployEnv != "" {
				// 精确字段优先：有明确的 deployEnv，直接严格比对，不再从 jobName 兜底
				if strings.EqualFold(e, deployEnv) {
					return true
				}
			} else if jobName != "" {
				// 仅当 deployEnv 为空时，才允许从任务名推断内嵌环境特征
				lowerJob := strings.ToLower(jobName)
				lowerE := strings.ToLower(e)
				if strings.Contains(lowerJob, "-"+lowerE+"-") ||
					strings.HasSuffix(lowerJob, "-"+lowerE) ||
					strings.HasPrefix(lowerJob, lowerE+"-") ||
					strings.Contains(lowerJob, "_"+lowerE+"_") ||
					strings.HasSuffix(lowerJob, "_"+lowerE) {
					return true
				}
			}
		}
	}

	return false
}
