package view

import (
	"bigdevops/src/common"
	"bigdevops/src/config"
	"bigdevops/src/models"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

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

// UserLogout 处理用户退出
func UserLogout(c *gin.Context) {
	// 1. 获取 Token (假设中间件已经通过 Header 拿到了)
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(200, gin.H{"code": 0, "message": "Already logged out"})
		return
	}

	// 2. 将 Token 加入 Redis 黑名单 (防止 Token 在有效期内被二次使用)
	// 这里的过期时间应该设为 JWT 剩余的有效期
	// 假设你已经定义了 global.Redis
	/*
	   claims := c.MustGet("claims").(*utils.CustomClaims)
	   waitTime := time.Until(time.Unix(claims.ExpiresAt, 0))
	   global.Redis.Set(context.Background(), "blacklist:"+token, "1", waitTime)
	*/

	// 3. 返回 Vben 期待的固定格式
	c.JSON(200, gin.H{
		"code":    0,
		"result":  nil,
		"message": "退出成功",
		"type":    "success",
	})
}

// 登录后获取用户信息 来自于jwt header
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

func createAccount(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqUser models.User
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

	reqUser.Roles = make([]*models.Role, 0)

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
	reqUser.Password = common.BcryptHash(reqUser.Password)
	reqUser.HomePath = "/system/role"
	err = reqUser.CreateOne()
	if err != nil {
		sc.Logger.Error("创建用户错误", zap.Any("菜单", reqUser), zap.Error(err))
		common.FailWithMessage(err.Error(), c)
		return
	}
	common.OkWithMessage("创建成功", c)
}

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

func updateAccount(c *gin.Context) {
	// 校验menu字段
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	var reqUser models.User
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

	reqUser.Roles = make([]*models.Role, 0)

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

func getAccountList(c *gin.Context) {
	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
	// 数据库中拿到所有的menu列表
	users, err := models.GetUserAll()
	if err != nil {
		sc.Logger.Error("去数据库中拿所有用户错误", zap.Error(err))
		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有用户错误：%v", err.Error()), c)
		return
	}

	//for i := 0; i < len(users); i++ {
	//
	//}

	common.OkWithDetailed(users, "ok", c)
}

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

//func getAllUserAndRoles(c *gin.Context) {
//	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
//	// 数据库中拿到所有的menu列表
//	users, err := models.GetUserAll()
//	if err != nil {
//		sc.Logger.Error("去数据库中拿所有用户错误", zap.Error(err))
//		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有用户错误：%v", err.Error()), c)
//		return
//	}
//	roles, err := models.GetRoleAll()
//	if err != nil {
//		sc.Logger.Error("去数据库中拿所有角色错误", zap.Error(err))
//		common.ReqBadFailWithMessage(fmt.Sprintf("去数据库中拿所有角色错误：%v", err.Error()), c)
//		return
//	}
//	var res []DefineUserOrGroup
//	for _, user := range users {
//		user := user
//		key := user.Username
//		//key := fmt.Sprintf("%s@%s", "用户", user.Username)
//		one := DefineUserOrGroup{
//			Label: key,
//			Value: key,
//		}
//		res = append(res, one)
//	}
//	for _, role := range roles {
//		role := role
//		key := role.RoleName
//		one := DefineUserOrGroup{
//			Label: key,
//			Value: key,
//		}
//		res = append(res, one)
//	}
//	common.OkWithDetailed(res, "ok", c)
//}

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

//type CreateUserReq struct {
//	UserName string   `json:"userName" validate:"required"`
//	Password string   `json:"password" validate:"required"`
//	RealName string   `json:"realName"`
//	Desc     string   `json:"desc"`
//	Roles    []string `json:"roles"` // 👈 用字符串数组接收 ["super", "frontAdmin"]
//}
//
//func createAccount(c *gin.Context) {
//	sc := c.MustGet(common.GIN_CTX_CONFIG_CONFIG).(*config.ServerConfig)
//
//	// 1. 改用 DTO 接收，避免 Unmarshal 报错
//	var req CreateUserReq
//	if err := c.ShouldBindJSON(&req); err != nil {
//		sc.Logger.Error("解析新增用户请求失败", zap.Error(err))
//		common.FailWithMessage(err.Error(), c)
//		return
//	}
//
//	// 2. 校验
//	if err := validate.Struct(&req); err != nil {
//		if errors, ok := err.(validator.ValidationErrors); ok {
//			common.ReqBadFailWithDetailed(errors.Translate(trans), "请求出错", c)
//			return
//		}
//	}
//
//	// 3. 处理角色转换：根据字符串去数据库查出真实的 Role 对象
//	var dbRoles []*models.Role
//	if len(req.Roles) > 0 {
//		// 这里的 role_value 对应你数据库里存 "super" 的那个字段
//		err := models.Db.Where("role_value IN ?", req.Roles).Find(&dbRoles).Error
//		if err != nil {
//			sc.Logger.Error("查询角色失败", zap.Error(err))
//		}
//	}
//
//	// 4. 组装真正的 User 模型
//	// 密码加密逻辑建议放在这里或者 User 的钩子里
//	hashedPassword := common.BcryptHash(req.Password)
//	newUser := models.User{
//		Username: req.UserName,
//		Password: hashedPassword,
//		RealName: req.RealName,
//		Desc:     req.Desc,
//		Roles:    dbRoles, // 👈 关联查出来的实体
//	}
//
//	// 5. 执行创建
//	err := newUser.CreateOne()
//	if err != nil {
//		sc.Logger.Error("创建用户错误", zap.Any("用户", req.UserName), zap.Error(err))
//		common.FailWithMessage(err.Error(), c)
//		return
//	}
//
//	common.OkWithMessage("创建成功", c)
//}
