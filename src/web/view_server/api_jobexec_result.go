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

func getJobExecResultOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetJobResultById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找脚本模板错误", zap.Any("脚本模板", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithDetailed(dbObj, "ok", c)

}

func getJobExecResultDetail(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("脚本模板", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetJobResultById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找脚本模板错误", zap.Any("脚本模板", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	//dbObj.FillFrontAllData()

	common.OkWithDetailed(dbObj, "ok", c)
}
