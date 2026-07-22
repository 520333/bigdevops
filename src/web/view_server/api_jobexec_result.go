package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// @Summary      查询作业任务在各主机的执行日志结果
// @Description  查询作业任务在各主机的执行日志结果 接口
// @Tags         jobexec-task
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "查询作业任务在各主机的执行日志结果 响应结果"
// @Router       /jobexec/getJobExecResultByJobId [get]
// @Security     Bearer
func getJobExecResultByJobId(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	// 1. 获取分页参数
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	limit := pageSize
	offset := (currentPage - 1) * limit

	// 2. 获取过滤参数
	jobId, _ := strconv.Atoi(c.DefaultQuery("jobId", "0"))
	searchStatus := c.DefaultQuery("status", "")
	searchIp := c.DefaultQuery("ip", "") // 接收前端传来的IP模糊搜索参数

	if jobId == 0 {
		common.ReqBadFailWithMessage("jobId 不能为空", c)
		return
	}

	// 3. 调用刚写的底层查询方法
	objs, total, err := models.GetJobResultsByFilters(jobId, searchStatus, searchIp, limit, offset)
	if err != nil {
		sc.Logger.Error("查询任务执行结果失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("查询任务执行结果失败：%v", err.Error()), c)
		return
	}

	// 4. 返回标准的分页 JSON 结构
	common.OkWithDetailed(gin.H{
		"items": objs,
		"total": total,
	}, "ok", c)
}
