package view_server

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

// UserLogin 用户登录接口
// @Summary      用户登录
// @Description  校验用户名密码，登录成功后在响应及Header中返回 JWT Token
// @Tags         系统管理模块
// @Accept       json
// @Produce      json
// @Param        data  body      models.UserLoginRequest  true  "登录请求参数"
// @Success      200   {object}  map[string]interface{}  "登录成功返回"
// @Failure      400   {object}  map[string]interface{}  "参数错误或密码错误"
// @Router       /../login [post]
func UserLogin(c *gin.Context) {
	// 校验用户账号密码
	var user models.UserLoginRequest
	err := c.ShouldBindJSON(&user)
	if err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	err = validate.Struct(&user)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			//common.ReqBadFailWithDetailed(
			//	gin.H{
			//		//"翻译前": err.Error(),
			//		"翻译后": errors.Translate(trans),
			//	}, "请求出错", c)
			//return
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	dbUser, err := models.CheckUserPassword(&user)
	if err != nil {
		sc.Logger.Error("登录失败！用户名不存在或者密码错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("用户不存在或者密码错误 %v", err.Error()), c)
		return
	}
	models.TokenNext(dbUser, c)
}

// UserLogout 用户登出接口
// @Summary      用户登出
// @Description  用户登出 接口
// @Tags         system-user
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "用户登出 响应结果"
// @Router       /logout [get]
func UserLogout(c *gin.Context) {
	// 1. 获取 Token 并清理活跃会话映射
	token := c.GetHeader("Authorization")
	if token != "" {
		parts := strings.SplitN(token, " ", 2)
		tokenStr := token
		if len(parts) == 2 && parts[0] == "Bearer" {
			tokenStr = parts[1]
		}
		sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
		if claims, err := models.ParseToken(tokenStr, sc); err == nil && claims != nil {
			models.ClearUserActiveToken(claims.Username)
		}
	}

	// 2. 返回 Vben 期待的固定格式
	c.JSON(200, gin.H{
		"code":    0,
		"result":  nil,
		"message": "退出成功",
		"type":    "success",
	})
}

// @Summary      获取当前登录用户信息
// @Description  获取当前登录用户信息 接口
// @Tags         system-user
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取当前登录用户信息 响应结果"
// @Router       /getUserInfo [get]
// @Security     Bearer
func getUserAfterLogin(c *gin.Context) {
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		sc.Logger.Error("通过token解析到的userName去数据库中找User", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("通过token解析到的userName去数据库中找User失败 %v", err.Error()), c)
		return
	}
	common.OkWithDetailed(dbUser, "ok", c)
}

// @Summary      获取当前用户权限码
// @Description  获取当前用户权限码 接口
// @Tags         system-user
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取当前用户权限码 响应结果"
// @Router       /getPermCode [get]
// @Security     Bearer
func getPermCode(c *gin.Context) {
	// 1. 从 JWT 或上下文获取当前登录用户的角色
	userNameInter, exists := c.Get(common.GIN_CTX_JWT_USER_NAME)
	if !exists {
		common.OkWithDetailed([]string{}, "未登录或Token无效", c)
		return
	}
	userName := userNameInter.(string)

	// 2. 查出用户信息及其拥有的角色
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		common.OkWithDetailed([]string{}, "获取用户失败", c)
		return
	}

	permCodes := make([]string, 0)
	permSet := make(map[string]bool) // 使用 map 进行权限去重（防止多个角色拥有相同的 API）

	// 3. 遍历用户拥有的所有角色
	for _, role := range dbUser.Roles {

		// ================= 超级管理员特权 =================
		if role.RoleValue == "super" {
			apis, err := models.GetApiAll()
			if err == nil {
				for _, api := range apis {
					permCode := fmt.Sprintf("%s:%s", api.Method, api.Path)
					if !permSet[permCode] {
						permCodes = append(permCodes, permCode)
						permSet[permCode] = true
					}
				}
			}
			// 如果是超管，拥有全部权限，直接返回即可
			common.OkWithDetailed(permCodes, "ok", c)
			return
		}
		// ===================================================

		// ================= 普通角色 =================
		// 根据 roleValue 查出该角色关联的 APIs
		dbRole, err := models.GetRoleByRoleValue(role.RoleValue)
		if err == nil {
			for _, api := range dbRole.Apis {
				permCode := fmt.Sprintf("%s:%s", api.Method, api.Path)
				if !permSet[permCode] { // 去重判断
					permCodes = append(permCodes, permCode)
					permSet[permCode] = true
				}
			}
		}
	}

	// 4. 返回合并去重后的真实权限码
	common.OkWithDetailed(permCodes, "ok", c)
}

// @Summary      创建系统账号
// @Description  创建系统账号 接口
// @Tags         system-account
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "创建系统账号 响应结果"
// @Router       /system/createAccount [post]
// @Security     Bearer
func createAccount(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqUser models.SystemUser
	err := c.ShouldBindJSON(&reqUser)
	if err != nil {
		sc.Logger.Error("解析新增用户请求失败", zap.Any("用户", reqUser), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqUser)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	reqUser.Roles = make([]*models.SystemRole, 0)

	for _, roleValue := range reqUser.RolesFront {
		dbRole, err := models.GetRoleByRoleValue(roleValue)
		if err != nil {
			sc.Logger.Error("根据RolesFront去db中查询角色失败", zap.Any("用户", reqUser), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		reqUser.Roles = append(reqUser.Roles, dbRole)
	}

	//hashPwd := common.BcryptHash(reqUser.Password)
	//reqUser.Password = common.BcryptHash(reqUser.Password)
	reqUser.Password = common.BcryptHash(reqUser.ReqPassword)
	reqUser.HomePath = "/dashboard/analysis"
	err = reqUser.CreateOne()
	if err != nil {
		sc.Logger.Error("创建用户错误", zap.Any("菜单", reqUser), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("创建成功", c)
}

// @Summary      检查账号是否存在
// @Description  检查账号是否存在 接口
// @Tags         system-account
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "检查账号是否存在 响应结果"
// @Router       /system/accountExist [post]
// @Security     Bearer
func accountExist(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqUser models.AccountExistRequest
	err := c.ShouldBindJSON(&reqUser)
	if err != nil {
		sc.Logger.Error("解析编辑用户请求失败", zap.Any("用户", reqUser), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqUser)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	dbUser, _ := models.GetUserByName(reqUser.Account)
	if dbUser != nil {
		sc.Logger.Info("用户已存在", zap.Any("用户", reqUser))
		common.FailWithMessage("用户名已存在，请更换", c)
		return
	}

	common.OkWithMessage("用户名可用", c)
}

// @Summary      更新系统账号
// @Description  更新系统账号 接口
// @Tags         system-account
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "更新系统账号 响应结果"
// @Router       /system/updateAccount [post]
// @Security     Bearer
func updateAccount(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqUser models.SystemUser
	err := c.ShouldBindJSON(&reqUser)
	if err != nil {
		sc.Logger.Error("解析编辑用户请求失败", zap.Any("用户", reqUser), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqUser)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	_, err = models.GetUserById(int(reqUser.ID))
	if err != nil {
		sc.Logger.Error("根据id找用户错误", zap.Any("用户", reqUser), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	reqUser.Roles = make([]*models.SystemRole, 0)

	for _, roleValue := range reqUser.RolesFront {
		dbRole, err := models.GetRoleByRoleValue(roleValue)
		if err != nil {
			sc.Logger.Error("根据RolesFront去db中查询角色失败", zap.Any("用户", reqUser), zap.Error(err))
			common.FailWithMessage(err.Error(), c)
			return
		}
		reqUser.Roles = append(reqUser.Roles, dbRole)
	}

	err = reqUser.UpdateOne(reqUser.Roles)
	if err != nil {
		sc.Logger.Error("编辑用户错误", zap.Any("菜单", reqUser), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("编辑成功", c)
}

// @Summary      删除系统账号
// @Description  删除系统账号 接口
// @Tags         system-account
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "删除系统账号 响应结果"
// @Router       /system/deleteAccount/{id} [delete]
// @Security     Bearer
func deleteAccount(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	id := c.Param("id")
	sc.Logger.Info("删除用户", zap.Any("id", id))
	intVar, _ := strconv.Atoi(id)

	dbUser, err := models.GetUserById(intVar)
	if err != nil {
		sc.Logger.Error("根据id找用户错误", zap.Any("用户", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	err = dbUser.DeleteOne()
	if err != nil {
		sc.Logger.Error("删除用户错误", zap.Any("用户", id), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("删除成功", c)
}

// @Summary      获取系统账号列表
// @Description  获取系统账号列表 接口
// @Tags         system-account
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取系统账号列表 响应结果"
// @Router       /system/getAccountList [get]
// @Security     Bearer
func getAccountList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	// 数据库中拿到所有的menu列表
	users, err := models.GetUserAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有用户错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有用户错误：%v", err.Error()), c)
		return
	}

	common.OkWithDetailed(users, "ok", c)
}

// @Summary      修改当前用户密码
// @Description  修改当前用户密码 接口
// @Tags         system-account
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "修改当前用户密码 响应结果"
// @Router       /system/changePassword [post]
// @Security     Bearer
func changePassword(c *gin.Context) {
	//{"passwordOld":"123456","passwordNew":"1"}
	// 校验menu字段
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		sc.Logger.Error("通过token解析到的userName去数据库中找User失败", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("通过token解析到的userName去数据库中找User失败：%v", err.Error()), c)
		return
	}

	var reqChange models.ChangePasswordRequest
	err = c.ShouldBindJSON(&reqChange)
	if err != nil {
		sc.Logger.Error("解析修改密码请求失败", zap.Any("用户", reqChange), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqChange)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	ok := common.BcryptCheck(reqChange.PasswordOld, dbUser.Password)
	if !ok {
		sc.Logger.Error("旧密码错误", zap.Any("用户", reqChange), zap.Error(err))
		common.FailWithMessage("旧密码错误", c)
		return
	}
	dbUser.Password = common.BcryptHash(reqChange.PasswordNew)
	err = dbUser.UpdateOne(dbUser.Roles)
	if err != nil {
		sc.Logger.Error("解析修改密码请求失败", zap.Any("菜单", reqChange), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("密码修改成功 ", c)
}

type DefineUserOrGroup struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// @Summary      获取全量用户与角色下拉列表
// @Description  获取全量用户与角色下拉列表 接口
// @Tags         system-account
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取全量用户与角色下拉列表 响应结果"
// @Router       /system/getAllUserAndRoles [get]
// @Security     Bearer
func getAllUserAndRoles(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	users, err := models.GetUserAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有用户错误", zap.Error(err))
		common.ReqBadFailWithMessage("获取用户失败", c)
		return
	}
	roles, err := models.GetRoleAll()
	if err != nil {
		common.ReqBadFailWithMessage("获取角色失败", c)
		return
	}

	var res []DefineUserOrGroup

	// 组装用户
	for _, user := range users {
		res = append(res, DefineUserOrGroup{
			Label: user.Username,
			Value: user.Username, // 纯用户名，后端识别 User
			Type:  "user",        // 明确标注为用户
		})
	}

	// 组装角色 (即组)
	for _, role := range roles {
		// 使用你喜欢的格式：组@角色值
		key := fmt.Sprintf("组@%s", role.RoleName)
		res = append(res, DefineUserOrGroup{
			Label: role.RoleName,
			Value: key,     // 带有“组@”前缀，后端识别 Group
			Type:  "group", // 明确标注为组
		})
	}

	common.OkWithDetailed(res, "ok", c)
}

type setAccountEnableReq struct {
	Id     uint `json:"id" validate:"required"`
	Enable int  `json:"enable" validate:"required,oneof=1 2"` // 假设 1=启用 2=禁用
}

// @Summary      设置账号启用状态
// @Description  设置账号启用状态 接口
// @Tags         system-account
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "设置账号启用状态 响应结果"
// @Router       /system/setAccountStatus [post]
// @Security     Bearer
func setAccountStatus(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	var reqObj setAccountEnableReq
	err := c.ShouldBindJSON(&reqObj)
	if err != nil {
		sc.Logger.Error("修改用户状态请求失败", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqObj)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	dbJob, err := models.GetUserById(int(reqObj.Id))
	if err != nil {
		sc.Logger.Error("根据id查找用户错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	dbJob.Enable = reqObj.Enable

	err = dbJob.UpdateEnable()
	if err != nil {
		sc.Logger.Error("更新用户启停状态错误", zap.Any("req", reqObj), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("状态修改成功", c)
}

// @Summary      修改当前登录用户个人设置
// @Description  修改当前登录用户个人设置/个人资料 接口
// @Tags         system-account
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "修改当前登录用户个人设置 响应结果"
// @Router       /system/updateUserInfo [post]
// @Security     Bearer
func updateUserInfo(c *gin.Context) {
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("用户不存在: %v", err), c)
		return
	}

	var reqObj models.UpdateUserInfoRequest
	err = c.ShouldBindJSON(&reqObj)
	if err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}

	err = validate.Struct(&reqObj)
	if err != nil {
		if errors, ok := err.(validator.ValidationErrors); ok {
			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
			return
		}
	}

	dbUser.RealName = reqObj.RealName
	if reqObj.Avatar != "" {
		dbUser.Avatar = reqObj.Avatar
	}
	dbUser.Email = reqObj.Email
	dbUser.Desc = reqObj.Desc
	dbUser.FeiShuUserId = reqObj.FeiShuUserId
	if reqObj.HomePath != "" {
		dbUser.HomePath = reqObj.HomePath
	}

	err = dbUser.UpdateOne(dbUser.Roles)
	if err != nil {
		sc.Logger.Error("更新个人信息失败", zap.Error(err))
		common.FailWithMessage("更新个人信息失败: "+err.Error(), c)
		return
	}

	common.OkWithDetailed(dbUser, "个人设置更新成功", c)
}

// @Summary      上传用户头像到 MinIO OSS
// @Description  上传用户头像文件到 MinIO 对象存储 接口
// @Tags         system-account
// @Accept       multipart/form-data
// @Produce      json
// @Success      200 {object} common.BaseResp "上传用户头像 响应结果"
// @Router       /system/uploadAvatar [post]
// @Security     Bearer
func uploadAvatar(c *gin.Context) {
	userName := c.MustGet(common.GIN_CTX_JWT_USER_NAME).(string)
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)

	dbUser, err := models.GetUserByUsername(userName)
	if err != nil {
		common.ReqBadFailWithMessage(fmt.Sprintf("用户不存在: %v", err), c)
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		common.ReqBadFailWithMessage("未获取到上传的文件: "+err.Error(), c)
		return
	}
	defer file.Close()

	// 限制文件大小不能超过 5MB
	if header.Size > 5*1024*1024 {
		common.ReqBadFailWithMessage("文件大小不能超过 5MB", c)
		return
	}

	// 校验图片扩展名
	filename := header.Filename
	ext := strings.ToLower(path.Ext(filename))
	validExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	if !validExts[ext] {
		common.ReqBadFailWithMessage("仅支持上传 jpg, jpeg, png, gif, webp 格式图片", c)
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}

	// 唯一对象文件名格式: avatars/user_<ID>_<Timestamp><Ext>
	objectName := fmt.Sprintf("avatars/user_%d_%d%s", dbUser.ID, time.Now().UnixNano(), ext)

	fileURL, err := common.UploadAvatarToMinio(sc, objectName, file, header.Size, contentType)
	if err != nil {
		sc.Logger.Error("上传头像到 MinIO 失败", zap.Error(err))
		common.ReqBadFailWithMessage("上传头像失败: "+err.Error(), c)
		return
	}

	// 自动同步数据库中该用户的 Avatar 字段
	dbUser.Avatar = fileURL
	_ = dbUser.UpdateOne(dbUser.Roles)

	common.OkWithDetailed(gin.H{
		"url":    fileURL,
		"avatar": fileURL,
	}, "头像上传成功", c)
}
