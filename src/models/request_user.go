package models

import "github.com/golang-jwt/jwt/v5"

type UserLoginRequest struct {
	Username string `json:"username" validate:"required,min=3,max=20"` // 用户名
	Password string `json:"password" validate:"required,min=3,max=20"` // 密码
	//Email    string `json:"email" validate:"required,email"`
}
type UserCustomClaims struct {
	*User
	jwt.RegisteredClaims
}

type UserLoginResponse struct {
	*User
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
