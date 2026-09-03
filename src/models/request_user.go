package models

import "github.com/golang-jwt/jwt/v5"

type UserLoginRequest struct {
	Username string `json:"username" validate:"required,min=3,max=20"` // 用户名
	Password string `json:"password" validate:"required,min=3,max=20"` // 密码
	//Email    string `json:"email" validate:"required,email"`
}
type UserCustomClaims struct {
	*SystemUser
	jwt.RegisteredClaims
}

type UserLoginResponse struct {
	*SystemUser
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"`
}

type AccountExistRequest struct {
	Account string `json:"account"`
}

type ChangePasswordRequest struct {
	PasswordOld string `json:"passwordOld"`
	PasswordNew string `json:"passwordNew"`
}

type UpdateUserInfoRequest struct {
	RealName     string `json:"realName" validate:"required,min=1,max=50"`
	Avatar       string `json:"avatar"`
	Email        string `json:"email" validate:"omitempty,email"`
	Desc         string `json:"desc" validate:"max=200"`
	FeiShuUserId string `json:"feiShuUserId" validate:"max=50"`
	HomePath     string `json:"homePath" validate:"max=100"`
}
