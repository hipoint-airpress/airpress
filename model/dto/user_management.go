package dto

import "github.com/hipoint-airpress/airpress/consts"

type UserContentCount struct {
	Posts       int64 `json:"posts"`
	Sheets      int64 `json:"sheets"`
	Attachments int64 `json:"attachments"`
}

type SelectableUser struct {
	ID       int32           `json:"id"`
	Username string          `json:"username"`
	Nickname string          `json:"nickname"`
	Role     consts.UserRole `json:"role"`
}
