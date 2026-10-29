package dto

import "github.com/hipoint-airpress/airpress/consts"

type User struct {
	ID                    int32           `json:"id"`
	Username              string          `json:"username"`
	Nickname              string          `json:"nickname"`
	Email                 string          `json:"email"`
	Avatar                string          `json:"avatar"`
	Description           string          `json:"description"`
	Role                  consts.UserRole `json:"role"`
	Disabled              bool            `json:"disabled"`
	MFAType               consts.MFAType  `json:"mfaType"`
	MFAEnrollmentRequired bool            `json:"mfaEnrollmentRequired"`
	MFARequired           bool            `json:"mfaRequired"`
	CreateTime            int64           `json:"createTime"`
	UpdateTime            int64           `json:"updateTime"`
}
