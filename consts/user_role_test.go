package consts

import (
	"encoding/json"
	"testing"
)

func TestUserRoleNumericValues(t *testing.T) {
	if UserRoleAuthor != 1 {
		t.Fatalf("UserRoleAuthor = %d, want 1", UserRoleAuthor)
	}
	if UserRoleEditor != 2 {
		t.Fatalf("UserRoleEditor = %d, want 2", UserRoleEditor)
	}
	if UserRoleSuperAdmin != 999 {
		t.Fatalf("UserRoleSuperAdmin = %d, want 999", UserRoleSuperAdmin)
	}
}

func TestUserRoleIsValid(t *testing.T) {
	cases := []struct {
		name string
		role UserRole
		want bool
	}{
		{name: "未知角色", role: UserRoleUnknown, want: false},
		{name: "作者", role: UserRoleAuthor, want: true},
		{name: "编辑", role: UserRoleEditor, want: true},
		{name: "超级管理员", role: UserRoleSuperAdmin, want: true},
		{name: "越界角色", role: UserRole(1000), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.role.IsValid(); got != tc.want {
				t.Fatalf("IsValid() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUserRoleJSON(t *testing.T) {
	cases := []struct {
		name string
		role UserRole
		want string
	}{
		{name: "作者", role: UserRoleAuthor, want: `"AUTHOR"`},
		{name: "编辑", role: UserRoleEditor, want: `"EDITOR"`},
		{name: "超级管理员", role: UserRoleSuperAdmin, want: `"SUPER_ADMIN"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.role)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(data) != tc.want {
				t.Fatalf("Marshal() = %s, want %s", data, tc.want)
			}

			var got UserRole
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if got != tc.role {
				t.Fatalf("Unmarshal() = %v, want %v", got, tc.role)
			}
		})
	}
}

func TestUserRoleJSONRejectsUnknown(t *testing.T) {
	var role UserRole
	if err := json.Unmarshal([]byte(`"OWNER"`), &role); err == nil {
		t.Fatal("Unmarshal() should reject unknown role")
	}
}

func TestUserRoleScanAndValue(t *testing.T) {
	var role UserRole
	if err := role.Scan(int64(2)); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if role != UserRoleEditor {
		t.Fatalf("Scan() = %v, want %v", role, UserRoleEditor)
	}

	value, err := role.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	if value != int64(2) {
		t.Fatalf("Value() = %v, want 2", value)
	}
}

func TestMFAPolicyRequires(t *testing.T) {
	cases := []struct {
		name   string
		policy MFAPolicy
		role   UserRole
		want   bool
	}{
		{name: "不强制-作者", policy: MFAPolicyNone, role: UserRoleAuthor, want: false},
		{name: "不强制-超管", policy: MFAPolicyNone, role: UserRoleSuperAdmin, want: false},
		{name: "所有用户-作者", policy: MFAPolicyAll, role: UserRoleAuthor, want: true},
		{name: "所有用户-编辑", policy: MFAPolicyAll, role: UserRoleEditor, want: true},
		{name: "所有用户-超管", policy: MFAPolicyAll, role: UserRoleSuperAdmin, want: true},
		{name: "仅管理角色-作者", policy: MFAPolicyAdminEditor, role: UserRoleAuthor, want: false},
		{name: "仅管理角色-编辑", policy: MFAPolicyAdminEditor, role: UserRoleEditor, want: true},
		{name: "仅管理角色-超管", policy: MFAPolicyAdminEditor, role: UserRoleSuperAdmin, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.policy.Requires(tc.role); got != tc.want {
				t.Fatalf("Requires() = %v, want %v", got, tc.want)
			}
		})
	}
}
