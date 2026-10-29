package consts

import (
	"database/sql/driver"

	"github.com/hipoint-airpress/airpress/util/xerr"
)

// UserRole 表示后台用户的角色。数值一旦发布不要修改，数据库按整数存储。
type UserRole int32

const (
	// UserRoleUnknown 是非法兜底值，不能写入数据库。
	UserRoleUnknown UserRole = iota
	UserRoleAuthor           // 1，只能管理自己的内容
	UserRoleEditor           // 2，管理全部内容
	// UserRoleSuperAdmin 单独使用 999，便于后续在编辑与超管之间扩展角色。
	UserRoleSuperAdmin UserRole = 999
)

func (r UserRole) IsValid() bool {
	switch r {
	case UserRoleAuthor, UserRoleEditor, UserRoleSuperAdmin:
		return true
	}
	return false
}

func (r UserRole) String() string {
	switch r {
	case UserRoleAuthor:
		return "AUTHOR"
	case UserRoleEditor:
		return "EDITOR"
	case UserRoleSuperAdmin:
		return "SUPER_ADMIN"
	default:
		return "UNKNOWN"
	}
}

func (r UserRole) MarshalJSON() ([]byte, error) {
	return []byte(`"` + r.String() + `"`), nil
}

func (r *UserRole) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"AUTHOR"`:
		*r = UserRoleAuthor
	case `"EDITOR"`:
		*r = UserRoleEditor
	case `"SUPER_ADMIN"`:
		*r = UserRoleSuperAdmin
	default:
		return xerr.BadParam.New("").WithMsg("unknown UserRole")
	}
	return nil
}

func (r *UserRole) Scan(src interface{}) error {
	if src == nil {
		return xerr.BadParam.New("").WithMsg("field nil")
	}
	switch data := src.(type) {
	case int64:
		*r = UserRole(data)
	case int32:
		*r = UserRole(data)
	case int:
		*r = UserRole(data)
	default:
		return xerr.BadParam.New("").WithMsg("bad type")
	}
	return nil
}

func (r UserRole) Value() (driver.Value, error) {
	return int64(r), nil
}

// MFAPolicy 表示两步验证的强制策略。
type MFAPolicy string

const (
	MFAPolicyNone        MFAPolicy = "NONE"
	MFAPolicyAll         MFAPolicy = "ALL"
	MFAPolicyAdminEditor MFAPolicy = "ADMIN_EDITOR"
)

func (p MFAPolicy) IsValid() bool {
	switch p {
	case MFAPolicyNone, MFAPolicyAll, MFAPolicyAdminEditor:
		return true
	}
	return false
}

// Requires 判断该角色是否被当前策略强制要求开启 MFA。
func (p MFAPolicy) Requires(role UserRole) bool {
	switch p {
	case MFAPolicyAll:
		return role.IsValid()
	case MFAPolicyAdminEditor:
		return role == UserRoleEditor || role == UserRoleSuperAdmin
	default:
		return false
	}
}
