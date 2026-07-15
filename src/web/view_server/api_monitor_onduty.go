package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type OnDutyPlanResponse struct {
	Details       []OnDutyOne       `json:"details"`
	Map           map[string]string `json:"map"`
	UserNameMap   map[string]string `json:"userNameMap"`
	OriginUserMap map[string]string `json:"originUserMap"`
}

type OnDutyOne struct {
	Date       string       `json:"date,omitempty"`
	User       *models.User `json:"user,omitempty"`
	OriginUser string       `json:"originUser,omitempty"`
	Remark     string       `json:"remark,omitempty"`
}

func getMonitorOndutyGroupList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	currentPage, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	searchUserID := c.DefaultQuery("UserID", "")
	searchUserIDInt, _ := strconv.Atoi(searchUserID)

	searchTitle := c.DefaultQuery("name", "")

	searchEnable := c.DefaultQuery("enable", "")
	searchEnableInt, _ := strconv.Atoi(searchEnable)

	shiftDays := c.DefaultQuery("shiftDays", "")
	searchShiftDaysInt, _ := strconv.Atoi(shiftDays)

	searchCreateUserName := c.DefaultQuery("createUserName", "")

	offset := 0
	limit := pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	objs, err := models.GetMonitorOndutyGroupAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有的值班组执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的值班组执行错误：%v", err.Error()), c)
		return
	}
	allIds := []int{}

	for _, obj := range objs {
		if searchUserID != "" && int(obj.UserID) != searchUserIDInt {
			continue
		}
		if searchEnable != "" && obj.Enable != searchEnableInt {
			continue
		}

		if searchTitle != "" && !strings.Contains(obj.Name, searchTitle) {
			continue
		}

		// 填充前端需要的数据（拿到组合好的 CreateUserName）
		obj.FillFrontAllData()

		// 🚀 修复 1：对比的应该是刚填充好的 CreateUserName 字段，而不是 UserID
		if searchCreateUserName != "" && !strings.Contains(obj.CreateUserName, searchCreateUserName) {
			continue
		}
		if shiftDays != "" && int(obj.ShiftDays) != searchShiftDaysInt {
			continue
		}

		allIds = append(allIds, int(obj.ID))
	}

	// 如果过滤后没有数据，直接返回空列表
	if len(allIds) == 0 {
		common.OkWithDetailed(gin.H{
			"items": []models.MonitorOndutyGroup{},
			"total": 0,
		}, "ok", c)
		return
	}

	// 根据过滤后的 ID 进行分页查询
	pagedObjs, err := models.GetMonitorOndutyGroupByIdsWithLimitOffset(allIds, limit, offset)
	if err != nil {
		sc.Logger.Error("limit-offset 去数据库中拿所有的值班组执行错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有的值班组执行错误：%v", err.Error()), c)
		return
	}

	// 🚀 修复 2：分页查出来的新对象，必须再次遍历填充一次虚拟字段，否则响应里还是空的！
	for _, obj := range pagedObjs {
		obj.FillFrontAllData()
		obj.FillToDayOndutyUser()
	}

	common.OkWithDetailed(gin.H{
		"items": pagedObjs,
		"total": len(allIds),
	}, "ok", c)
}

func getMonitorOndutyGroupFuturePlan(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	startDay := c.DefaultQuery("startDay", "") // 查询的起止时间 开始可能是之前的天数 30天前到今天
	endDay := c.DefaultQuery("endDay", "")     // 结束也可能是未来的时间 30天后
	var (
		startDayTime, endDayTime time.Time
		err                      error
	)
	startDayTime, err = time.Parse("2006-01-02", startDay)
	if err != nil {
		sc.Logger.Error("解析日期错误", zap.Any("值班组", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	endDayTime, err = time.Parse("2006-01-02", endDay)
	if err != nil {
		sc.Logger.Error("解析日期错误", zap.Any("值班组", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	if endDayTime.Sub(startDayTime) < 0 {
		msg := fmt.Sprintf("结束时间比开始时间要小")
		sc.Logger.Error(msg, zap.Any("值班组", id), zap.Error(err))
		common.FailWithMessage(msg, c)
		return
	}

	// 判断是否完全历史
	todayTime := time.Now()
	if endDayTime.Before(todayTime) {
		historys, err := models.GetMonitorOnDutyHistoryByOnDutyGroupIdAndTimeRange(intVar, startDay, endDay)
		if err != nil {
			sc.Logger.Error("根据值班组id找历史错误", zap.Any("值班组id", id), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		tmpRes := []OnDutyOne{}
		ondutyPlanResponse := OnDutyPlanResponse{}
		tmp := map[string]string{}
		for _, history := range historys {
			history := history
			user, err := models.GetUserById(int(history.OndutyUserId))
			if err != nil {
				continue
			}
			one := OnDutyOne{
				Date: history.DateString,
				User: user,
			}
			tmpRes = append(tmpRes, one)
			tmp[history.DateString] = one.User.RealName
		}
		ondutyPlanResponse.Details = tmpRes
		ondutyPlanResponse.Map = tmp
		common.OkWithData(ondutyPlanResponse, c)
		return

	}

	// 后面的逻辑可以混在一起 从start到今天
	if startDayTime.Sub(startDayTime) > 0 {

	}

	dbObj, err := models.GetMonitorOndutyGroupById(intVar)
	if err != nil {
		sc.Logger.Error("解析日期错误", zap.Any("值班组", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbObj.FillFrontAllData()
	if dbObj.Members == nil {
		sc.Logger.Error("根据id找值班组member为空", zap.Any("值班组", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	tmpRes := []OnDutyOne{}
	todayDate := time.Now().Format("2006-01-02")
	historys, err := models.GetMonitorOnDutyHistoryByOnDutyGroupIdAndTimeRange(intVar, startDay, todayDate)
	if err != nil {
		sc.Logger.Error("根据值班组id找历史错误", zap.Any("值班组id", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	for _, history := range historys {
		history := history
		user, err := models.GetUserById(int(history.OndutyUserId))
		if err != nil {
			continue
		}
		tmpRes = append(tmpRes, OnDutyOne{
			Date: history.DateString,
			User: user,
		})

	}

	toDayHistory, _ := models.GetMonitorOnDutyHistoryByOnDutyGroupIdAndDay(uint(intVar), todayDate)
	onDutyUsers := dbObj.Members

	//var yesterdayUserId uint
	var toDayUserId uint
	if toDayHistory != nil && toDayHistory.OndutyUserId > 0 {
		toDayUserId = toDayHistory.OndutyUserId
	} else if len(onDutyUsers) > 0 {
		toDayUserId = onDutyUsers[0].ID
	}

	if len(onDutyUsers) == 0 {
		common.FailWithMessage("该值班组没有配置值班人员", c)
		return
	}

	// 1. 计算需要预测的未来天数
	toDay, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02")) //今天的日期

	futureNum := int(endDayTime.Sub(toDay).Hours() / 24)
	if futureNum < 0 {
		futureNum = 0 // 🚀 修复负数导致越界崩溃的 Bug
	}

	// 2. 找到明天的起始轮转索引
	firstLeftNum := 0
	if toDayHistory != nil && toDayHistory.OndutyUserId > 0 {
		for index, user := range onDutyUsers {
			if user.ID == toDayUserId {
				firstLeftNum = (index + 1) % len(onDutyUsers) // 🚀 自动轮转到下一个人
				break
			}
		}
	}

	// 3. 用一个极简的 for 循环推演未来，抛弃容易报错的切片截取和倍数余数计算
	start := toDay.Add(24 * time.Hour) // 因为 history 已经查到了今天，预测从明天开始
	currentIndex := firstLeftNum

	for i := 0; i < futureNum; i++ {
		day := start.Format("2006-01-02")
		planUser := onDutyUsers[currentIndex]

		tmpRes = append(tmpRes, OnDutyOne{
			Date: day,
			User: planUser,
		})

		start = start.Add(24 * time.Hour)
		currentIndex = (currentIndex + 1) % len(onDutyUsers) // 索引步进并自动取模
	}

	// 4. 统一执行终极过滤（剔除 startDay 之前，以及 endDay 之后的脏数据）
	ffRes := []OnDutyOne{}
	tmp := map[string]string{}
	userNameMap := map[string]string{}
	originUserMap := map[string]string{}

	for _, node := range tmpRes {

		// 先获取换班记录
		dbChange, _ := models.GetMonitorOndutyChangeByOnDutyGroupIdAndDay(dbObj.ID, node.Date)
		if dbChange.OndutyUserId > 0 {
			user, _ := models.GetUserById(int(dbChange.OndutyUserId))
			oriUser, _ := models.GetUserById(int(dbChange.OriginUserId))
			if user.RealName != "" {
				node.User = user
				node.OriginUser = oriUser.RealName
				node.Remark = dbChange.Remark
			}
		}

		thisDateTime, _ := time.Parse("2006-01-02", node.Date)

		// 如果这条数据的日期 < 搜索的开始日期，跳过
		if thisDateTime.Unix() < startDayTime.Unix() {
			continue
		}
		// 如果这条数据的日期 > 搜索的结束日期，跳过
		if thisDateTime.Unix() > endDayTime.Unix() {
			continue
		}

		ffRes = append(ffRes, node)
		tmp[node.Date] = node.User.RealName
		originUserMap[node.Date] = node.OriginUser
		userNameMap[node.Date] = node.User.Username

	}

	ondutyPlanResponse := OnDutyPlanResponse{}
	ondutyPlanResponse.Details = ffRes
	ondutyPlanResponse.Map = tmp
	ondutyPlanResponse.UserNameMap = userNameMap
	ondutyPlanResponse.OriginUserMap = originUserMap

	// 🚀 这里是唯一的出口，确保所有数据都经过了上面的时间过滤
	common.OkWithData(ondutyPlanResponse, c)
}

func createMonitorOndutyGroup(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj models.MonitorOndutyGroup
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增值班组执行请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 获取当前用户ID
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)

	if err == nil && dbUser != nil {
		reqObj.UserID = dbUser.ID
	}

	reqObj.Members = commonGetUsersByNames(reqObj.UserNames, sc.Logger, c)
	// 转化userName到members
	//for _, userName := range reqObj.FirstUserNames {
	//	userName := userName
	//	dbUser, err := models.GetUserByUsername(userName)
	//	if err != nil {
	//		sc.Logger.Error("解析新增值班组执行请求失败", zap.Error(err))
	//		common.FailWithMessage(err.Error(), c)
	//		return
	//	}
	//	reqObj.Members = append(reqObj.Members, dbUser)
	//}

	// 存入数据库
	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增值班组执行数据库失败", zap.Error(err))
		common.FailWithMessage("存入数据库失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

func updateMonitorOndutyGroup(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	// 🚀 致命修复：同上
	var reqObj models.MonitorOndutyGroup
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析更新值班组请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	// 检查是否存在
	_, err = models.GetMonitorOndutyGroupById(int(reqObj.ID))
	if err != nil {
		common.FailWithMessage("值班组不存在", c)
		return
	}

	// 转化userName到members
	reqObj.Members = commonGetUsersByNames(reqObj.UserNames, sc.Logger, c)

	err = reqObj.UpdateOne()
	if err != nil {
		sc.Logger.Error("更新值班组执行错误", zap.Error(err))
		common.FailWithMessage("更新失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

// setMonitorOndutyGroupEnableReq 请求参数结构体
type setMonitorOndutyGroupEnableReq struct {
	Id     uint `json:"id" validate:"required"`
	Enable int  `json:"enable" validate:"required,oneof=1 2"` // 假设 1=启用 2=禁用
}

func deleteMonitorOndutyGroup(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetMonitorOndutyGroupById(intVar)
	if err != nil {
		common.FailWithMessage("值班组不存在", c)
		return
	}
	dbSendGroups, _ := models.GetMonitorAlertManagerSendGroupByOndutyGroupId(uint(intVar))
	if dbSendGroups != nil && len(dbSendGroups) > 0 {
		sc.Logger.Warn("该值班组已经绑定了发送组，禁止直接删除！", zap.Any("", id))
		common.FailWithMessage("该值班组已经绑定了发送组，禁止直接删除！", c)
		return
	}
	err = dbObj.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除值班组执行错误", zap.Error(err))
		common.FailWithMessage("删除失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("删除成功", c)
}

func getMonitorOndutyGroupOne(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")

	intVar, _ := strconv.Atoi(id)

	dbObj, err := models.GetMonitorOndutyGroupById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找值班组错误", zap.Any("id", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbObj.FillFrontAllData()

	common.OkWithData(dbObj, c)
}

func createMonitorOndutyChange(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj models.MonitorOndutyChange
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析新增值班组执行请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 获取当前用户ID
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		sc.Logger.Error("解析新增值班组执行请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbGroup, err := models.GetMonitorOndutyGroupById(int(reqObj.OndutyGroupId))
	if err != nil {
		sc.Logger.Error("解析新增值班组执行请求失败", zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	dbOriginUser, err := models.GetUserByUsername(reqObj.OriginUserName)
	if err != nil {
		sc.Logger.Error("通过dbOriginUser去数据库中找user失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("通过dbOriginUser去数据库中找user失败%v", err.Error()), c)
		return
	}

	dbTargetUser, err := models.GetUserByUsername(reqObj.TargetUserName)
	if err != nil {
		sc.Logger.Error("通过dbTargetUser去数据库中找user失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("通过dbTargetUser去数据库中找user失败%v", err.Error()), c)
		return
	}
	// 🚀 新增拦截防线：判断源用户ID和目标用户ID是否一致
	if dbOriginUser.ID == dbTargetUser.ID {
		common.FailWithMessage("无效操作：替班人员不能是原定值班人自己", c)
		return
	}

	isValidMember := false
	for _, member := range dbGroup.Members {
		if member.ID == dbTargetUser.ID {
			isValidMember = true
			break
		}
	}
	if !isValidMember {
		common.FailWithMessage("越权操作：替班人员必须是当前值班组的成员", c)
		return
	}
	reqObj.UserId = dbUser.ID
	reqObj.OriginUserId = dbOriginUser.ID
	reqObj.OndutyUserId = dbTargetUser.ID

	// 存入数据库
	err = reqObj.CreateOne()
	if err != nil {
		sc.Logger.Error("新增值班组执行数据库失败", zap.Error(err))
		common.FailWithMessage("存入数据库失败: "+err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

// setMonitorOndutyStatus 设置采集任务的启用/禁用状态
func setMonitorOndutyStatus(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj setMonitorOndutyGroupEnableReq
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("解析值班组状态请求失败", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 结构体数据校验
	err = validate.Struct(&reqObj)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	// 1. 查询数据库中原有的记录
	dbJob, err := models.GetMonitorOndutyGroupById(int(reqObj.Id))
	if err != nil {
		sc.Logger.Error("根据id查找值班组错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	// 2. 内存中修改状态
	dbJob.Enable = reqObj.Enable

	// 3. 执行更新
	err = dbJob.UpdateEnable()
	if err != nil {
		sc.Logger.Error("更新值班组状态错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("状态修改成功", c)
}
