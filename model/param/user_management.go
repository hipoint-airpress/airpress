package param

import "github.com/hipoint-airpress/airpress/consts"

type UserQuery struct {
	Page
	Keyword  string           `form:"keyword"`
	Role     *consts.UserRole `form:"role"`
	Disabled *bool            `form:"disabled"`
}

type CreateUser struct {
	Username    string          `json:"username" binding:"required,lte=50"`
	Nickname    string          `json:"nickname" binding:"required,lte=255"`
	Email       string          `json:"email" binding:"required,email,lte=127"`
	Password    string          `json:"password" binding:"required,gte=8,lte=100"`
	Role        consts.UserRole `json:"role"`
	Avatar      string          `json:"avatar" binding:"lte=1023"`
	Description string          `json:"description" binding:"lte=1023"`
}

type UpdateUser struct {
	Username    string `json:"username" binding:"required,lte=50"`
	Nickname    string `json:"nickname" binding:"required,lte=255"`
	Email       string `json:"email" binding:"required,email,lte=127"`
	Avatar      string `json:"avatar" binding:"lte=1023"`
	Description string `json:"description" binding:"lte=1023"`
}

type UpdateUserRole struct {
	Role consts.UserRole `json:"role" binding:"required"`
}

type UpdateUserStatus struct {
	Disabled bool `json:"disabled"`
}

type ResetUserPassword struct {
	NewPassword string `json:"newPassword" binding:"required,gte=8,lte=100"`
}
