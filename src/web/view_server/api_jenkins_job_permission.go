package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// getRoleJobPermissions 获取指定角色的服务基线数据权限列表
func getRoleJobPermissions(c *gin.Context) {
	roleIdStr := c.Query("roleId")
	roleId, err := strconv.ParseUint(roleIdStr, 10, 64)
	if err != nil || roleId == 0 {
		common.ReqBadFailWithMessage("缺少有效的角色 ID (roleId)", c)
		return
	}

	list, err := models.GetPermissionsByRoleID(uint(roleId))
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("查询角色权限数据失败: %v", err), c)
		return
	}

	common.OkWithData(list, c)
}

// saveRoleJobPermissionsReq 保存角色数据权限请求体
type saveRoleJobPermissionsReq struct {
	RoleID      uint                        `json:"roleId" binding:"required"`
	Permissions []saveRoleJobPermissionItem `json:"permissions"`
}

type saveRoleJobPermissionItem struct {
	ProjectName string `json:"projectName"`
	DeployEnv   string `json:"deployEnv"`
	AllowBuild  int    `json:"allowBuild"`
}

// saveRoleJobPermissions 批量保存指定角色的服务基线数据权限
func saveRoleJobPermissions(c *gin.Context) {
	var req saveRoleJobPermissionsReq
	if err := c.ShouldBindJSON(&req); err != nil || req.RoleID == 0 {
		common.ReqBadFailWithMessage("入参解析异常或缺少角色 ID (roleId)", c)
		return
	}

	err := models.Db.Transaction(func(tx *gorm.DB) error {
		// 1. 先清空该角色原有的权限定义
		if err := tx.Where("role_id = ?", req.RoleID).Delete(&models.JenkinsJobPermission{}).Error; err != nil {
			return err
		}

		// 2. 批量写入新的权限定义
		for _, item := range req.Permissions {
			proj := strings.TrimSpace(item.ProjectName)
			env := strings.TrimSpace(item.DeployEnv)
			if proj == "" || env == "" {
				continue
			}
			allowBuild := item.AllowBuild
			if allowBuild != 1 && allowBuild != 2 {
				allowBuild = 1
			}

			record := models.JenkinsJobPermission{
				RoleID:      req.RoleID,
				ProjectName: proj,
				DeployEnv:   env,
				AllowBuild:  allowBuild,
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("保存角色权限失败: %v", err), c)
		return
	}

	common.OkWithMessage("角色服务基线权限保存成功！", c)
}

// getJobProjectOptions 获取现有可授权的项目空间及环境元数据，供管理端选择勾选或前端筛选 (基于权限过滤)
func getJobProjectOptions(c *gin.Context) {
	var currentUser *models.SystemUser
	var superRoleName string
	if scVal, exists := c.Get(common.GIN_CTX_CONFIG_CONFIG); exists {
		if sc, ok := scVal.(*config.ServerConfig); ok && sc != nil {
			superRoleName = sc.SuperRoleName
		}
	}
	if userNameVal, ok := c.Get(common.GIN_CTX_JWT_USER_NAME); ok {
		userName := fmt.Sprintf("%v", userNameVal)
		currentUser, _ = models.GetUserByUsername(userName)
	}

	isSuper := models.IsUserSuperRole(currentUser, superRoleName)
	forceAll := (c.Query("all") == "true") && isSuper

	var projects []string
	var envs []string

	if isSuper || forceAll {
		// 超级管理员：可看全部项目与环境
		models.Db.Model(&models.JenkinsJob{}).
			Where("project_name != '' AND project_name IS NOT NULL").
			Distinct("project_name").
			Order("project_name asc").
			Pluck("project_name", &projects)

		models.Db.Model(&models.JenkinsJob{}).
			Where("deploy_env != '' AND deploy_env IS NOT NULL").
			Distinct("deploy_env").
			Pluck("deploy_env", &envs)

		standardEnvs := []string{"dev", "test", "stage", "uat", "pre", "prod"}
		envSet := make(map[string]bool)
		for _, e := range envs {
			if strings.TrimSpace(e) != "" {
				envSet[e] = true
			}
		}
		for _, se := range standardEnvs {
			if !envSet[se] {
				envs = append(envs, se)
			}
		}
		sort.Strings(projects)
		sort.Strings(envs)

		common.OkWithData(gin.H{
			"projects": projects,
			"envs":     envs,
		}, c)
		return
	}

	// 非超级管理员：严格根据用户角色绑定的基线数据权限过滤
	if currentUser == nil || len(currentUser.Roles) == 0 {
		common.OkWithData(gin.H{
			"projects": []string{},
			"envs":     []string{},
		}, c)
		return
	}

	var roleIDs []uint
	for _, r := range currentUser.Roles {
		if r != nil && r.ID > 0 {
			roleIDs = append(roleIDs, r.ID)
		}
	}
	if len(roleIDs) == 0 {
		common.OkWithData(gin.H{
			"projects": []string{},
			"envs":     []string{},
		}, c)
		return
	}

	perms, err := models.GetPermissionsByRoleIDs(roleIDs)
	if err != nil || len(perms) == 0 {
		common.OkWithData(gin.H{
			"projects": []string{},
			"envs":     []string{},
		}, c)
		return
	}

	// 检查是否有全局通配符权限 ProjectName = "*"
	hasGlobalProj := false
	for _, p := range perms {
		if strings.TrimSpace(p.ProjectName) == "*" {
			hasGlobalProj = true
			break
		}
	}

	if hasGlobalProj {
		models.Db.Model(&models.JenkinsJob{}).
			Where("project_name != '' AND project_name IS NOT NULL").
			Distinct("project_name").
			Pluck("project_name", &projects)
	} else {
		// 结合从数据库中根据该用户数据权限物理过滤出的可用 Job 项目，以及权限规则中显式授权的有效项目
		jobQuery := models.Db.Model(&models.JenkinsJob{})
		jobQuery, hasAccess := models.FilterJobsByPermission(jobQuery, currentUser, superRoleName)
		projSet := make(map[string]bool)

		if hasAccess {
			var jobProjects []string
			jobQuery.Where("project_name != '' AND project_name IS NOT NULL").
				Distinct("project_name").
				Pluck("project_name", &jobProjects)
			for _, jp := range jobProjects {
				jp = strings.TrimSpace(jp)
				if jp != "" {
					projSet[jp] = true
				}
			}
		}

		for _, p := range perms {
			proj := strings.TrimSpace(p.ProjectName)
			if proj != "" && proj != "*" {
				projSet[proj] = true
			}
		}

		for p := range projSet {
			projects = append(projects, p)
		}
	}

	// 环境过滤：从权限规则与用户可见 Job 中提取
	envSet := make(map[string]bool)
	hasGlobalEnv := false
	for _, p := range perms {
		for _, e := range strings.Split(p.DeployEnv, ",") {
			e = strings.TrimSpace(e)
			if e == "*" {
				hasGlobalEnv = true
			} else if e != "" {
				envSet[e] = true
			}
		}
	}

	if hasGlobalEnv {
		models.Db.Model(&models.JenkinsJob{}).
			Where("deploy_env != '' AND deploy_env IS NOT NULL").
			Distinct("deploy_env").
			Pluck("deploy_env", &envs)
		standardEnvs := []string{"dev", "test", "stage", "uat", "pre", "prod"}
		for _, se := range standardEnvs {
			if !envSet[se] {
				envSet[se] = true
				envs = append(envs, se)
			}
		}
	} else {
		for e := range envSet {
			envs = append(envs, e)
		}
	}

	if projects == nil {
		projects = []string{}
	}
	if envs == nil {
		envs = []string{}
	}
	sort.Strings(projects)
	sort.Strings(envs)

	common.OkWithData(gin.H{
		"projects": projects,
		"envs":     envs,
	}, c)
}
