package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BaseResp 定义一个通用的返回给前端的字段结构体
type BaseResp struct {
	Code    int         `json:"code"` // 这是的code不是Http的状态码 而是前后端交互定义的
	Data    interface{} `json:"result"`
	Message string      `json:"message"`
	Type    string      `json:"type"`
}

const (
	ERROR   = 7
	SUCCESS = 0
)

func Result(code int, data interface{}, msg string, c *gin.Context) {
	c.JSON(http.StatusOK, BaseResp{
		Code:    code,
		Data:    data,
		Message: msg,
		Type:    "",
	})
}

// Result400 参数错误
func Result400(code int, data interface{}, msg string, c *gin.Context) {
	c.JSON(http.StatusBadRequest, BaseResp{
		Code:    code,
		Data:    data,
		Message: msg,
		Type:    "",
	})
}

// Result401 未认证
func Result401(code int, data interface{}, msg string, c *gin.Context) {
	c.JSON(http.StatusUnauthorized, BaseResp{
		Code:    code,
		Data:    data,
		Message: msg,
		Type:    "",
	})
}

// Result403 访问拒绝
func Result403(code int, data interface{}, msg string, c *gin.Context) {
	c.JSON(http.StatusForbidden, BaseResp{
		Code:    code,
		Data:    data,
		Message: msg,
		Type:    "",
	})
}

func Result5xx(code int, data interface{}, msg string, c *gin.Context) {
	c.JSON(http.StatusInternalServerError, BaseResp{
		Code:    code,
		Data:    data,
		Message: msg,
		Type:    "",
	})
}

func Ok(c *gin.Context) {
	Result(SUCCESS, map[string]interface{}{}, "操作成功", c)
}

func OkWithMessage(message string, c *gin.Context) {
	Result(SUCCESS, map[string]interface{}{}, message, c)
}

func OkWithData(data interface{}, c *gin.Context) {
	Result(SUCCESS, data, "查询成功", c)
}

func OkWithDetailed(data interface{}, message string, c *gin.Context) {
	Result(SUCCESS, data, message, c)
}

func Fail(c *gin.Context) {
	Result(ERROR, map[string]interface{}{}, "操作失败", c)
}

func FailWithMessage(message string, c *gin.Context) {
	Result(ERROR, map[string]interface{}{}, message, c)
}

func FailWithDetailed(data interface{}, message string, c *gin.Context) {
	Result(ERROR, data, message, c)
}

func ReqBadFailWithMessage(message string, c *gin.Context) {
	Result400(ERROR, map[string]interface{}{}, message, c)
}

func ReqBadFailWithDetailed(data interface{}, message string, c *gin.Context) {
	Result400(ERROR, data, message, c)
}

func Req401WithDetailed(data interface{}, message string, c *gin.Context) {
	Result401(ERROR, data, message, c)
}

func Req403WithMessage(message string, c *gin.Context) {
	Result403(ERROR, map[string]interface{}{}, message, c)
}

func Req5xxWithDetailed(data interface{}, message string, c *gin.Context) {
	Result5xx(ERROR, data, message, c)
}
