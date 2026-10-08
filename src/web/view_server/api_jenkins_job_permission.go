package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/models"
	"fmt"
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

// getJobProjectOptions 获取现有可授权的项目空间及环境元数据，供管理端选择勾选
func getJobProjectOptions(c *gin.Context) {
	var projects []string
	var envs []string

	models.Db.Model(&models.JenkinsJob{}).
		Where("project_name != '' AND project_name IS NOT NULL").
		Distinct("project_name").
		Pluck("project_name", &projects)

	models.Db.Model(&models.JenkinsJob{}).
		Where("deploy_env != '' AND deploy_env IS NOT NULL").
		Distinct("deploy_env").
		Pluck("deploy_env", &envs)

	// 补充内置常见标准环境
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

	common.OkWithData(gin.H{
		"projects": projects,
		"envs":     envs,
	}, c)
}
