package view_server

import (
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zhs "github.com/go-playground/validator/v10/translations/zh"
)

var (
	validate = validator.New()          // 实例化验证器
	chinese  = zh.New()                 // 获取中文翻译器
	uni      = ut.New(chinese, chinese) // 设置中文翻译器
	trans, _ = uni.GetTranslator("zh")  // 获取中文字典
	_        = zhs.RegisterDefaultTranslations(validate, trans)
)
