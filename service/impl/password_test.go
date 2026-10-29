package impl

import (
	"testing"

	"github.com/hipoint-airpress/airpress/util/xerr"
)

func TestValidatePasswordStrength(t *testing.T) {
	cases := []struct {
		name     string
		password string
		wantOK   bool
	}{
		{name: "合法密码", password: "Abcdef1!", wantOK: true},
		{name: "刚好 8 位", password: "Aa1!aaaa", wantOK: true},
		{name: "包含多个特殊字符", password: "P@ssw0rd#", wantOK: true},
		{name: "太短", password: "Ab1!", wantOK: false},
		{name: "缺少大写", password: "abcdef1!", wantOK: false},
		{name: "缺少小写", password: "ABCDEF1!", wantOK: false},
		{name: "缺少数字", password: "Abcdefg!", wantOK: false},
		{name: "缺少特殊字符", password: "Abcdefg1", wantOK: false},
		{name: "空密码", password: "", wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePasswordStrength(tc.password)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("validatePasswordStrength() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("validatePasswordStrength() should fail")
			}
			if got := xerr.GetHTTPStatus(err); got != xerr.StatusBadRequest {
				t.Fatalf("status = %d, want %d", got, xerr.StatusBadRequest)
			}
		})
	}
}
