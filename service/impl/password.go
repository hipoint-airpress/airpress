package impl

import (
	"unicode"
	"unicode/utf8"

	"github.com/hipoint-airpress/airpress/util/xerr"
)

// validatePasswordStrength 校验新密码必须满足：
// 8-100 位，且同时包含大写字母、小写字母、数字和特殊字符。
func validatePasswordStrength(password string) error {
	length := utf8.RuneCountInString(password)
	if length < 8 || length > 100 {
		return xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("密码长度必须在 8 - 100 之间")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	switch {
	case !hasUpper:
		return xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("密码必须包含至少一个大写字母")
	case !hasLower:
		return xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("密码必须包含至少一个小写字母")
	case !hasDigit:
		return xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("密码必须包含至少一个数字")
	case !hasSpecial:
		return xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("密码必须包含至少一个特殊字符")
	}
	return nil
}
