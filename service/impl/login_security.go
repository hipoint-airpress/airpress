package impl

import (
	"strconv"
	"strings"
	"time"

	"github.com/hipoint-airpress/airpress/cache"
	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

const (
	loginFailIDPrefix   = "login_fail_id_"
	loginLockIDPrefix   = "login_lock_id_"
	loginFailUserPrefix = "login_fail_user_"
	loginLockUserPrefix = "login_lock_user_"
	loginFailIPPrefix   = "login_fail_ip_"
	loginLockIPPrefix   = "login_lock_ip_"
	mfaFailUserPrefix   = "mfa_fail_user_"
	mfaLockUserPrefix   = "mfa_lock_user_"
)

// loginSecurity 用进程内缓存记录登录与 MFA 失败次数。多实例部署时状态不共享。
type loginSecurity struct {
	cache cache.Cache
}

func newLoginSecurity(c cache.Cache) *loginSecurity {
	return &loginSecurity{cache: c}
}

func normalizeIdentifier(identifier string) string {
	return strings.ToLower(strings.TrimSpace(identifier))
}

func tooManyAttemptsError() error {
	return xerr.WithStatus(nil, xerr.StatusTooManyRequests).WithMsg("尝试次数过多，请稍后再试")
}

func (s *loginSecurity) CheckPasswordLock(identifier, ip string) error {
	identifier = normalizeIdentifier(identifier)
	if s.locked(loginLockIDPrefix + identifier) {
		return tooManyAttemptsError()
	}
	if ip != "" && s.locked(loginLockIPPrefix+ip) {
		return tooManyAttemptsError()
	}
	return nil
}

func (s *loginSecurity) RecordPasswordFailure(identifier string, userID int32, ip string) {
	identifier = normalizeIdentifier(identifier)
	if s.increment(loginFailIDPrefix+identifier, consts.LoginFailureWindow) >= consts.LoginMaxFailures {
		s.cache.Set(loginLockIDPrefix+identifier, true, consts.LoginLockDuration)
	}
	if userID > 0 {
		if s.increment(loginFailUserPrefix+strconv.Itoa(int(userID)), consts.LoginFailureWindow) >= consts.LoginMaxFailures {
			s.cache.Set(loginLockUserPrefix+strconv.Itoa(int(userID)), true, consts.LoginLockDuration)
		}
	}
	if ip != "" {
		if s.increment(loginFailIPPrefix+ip, consts.LoginFailureWindow) >= consts.LoginIPMaxFailures {
			s.cache.Set(loginLockIPPrefix+ip, true, consts.LoginLockDuration)
		}
	}
}

func (s *loginSecurity) ClearPasswordFailures(identifier string, userID int32, ip string) {
	identifier = normalizeIdentifier(identifier)
	s.cache.Delete(loginFailIDPrefix + identifier)
	s.cache.Delete(loginLockIDPrefix + identifier)
	if userID > 0 {
		s.cache.Delete(loginFailUserPrefix + strconv.Itoa(int(userID)))
		s.cache.Delete(loginLockUserPrefix + strconv.Itoa(int(userID)))
	}
}

func (s *loginSecurity) CheckMFALock(userID int32) error {
	if s.locked(mfaLockUserPrefix + strconv.Itoa(int(userID))) {
		return tooManyAttemptsError()
	}
	return nil
}

func (s *loginSecurity) RecordMFAFailure(userID int32) {
	if s.increment(mfaFailUserPrefix+strconv.Itoa(int(userID)), consts.MFAVerifyWindow) >= consts.MFAVerifyMaxFailures {
		s.cache.Set(mfaLockUserPrefix+strconv.Itoa(int(userID)), true, consts.MFALockDuration)
	}
}

func (s *loginSecurity) ClearMFAFailures(userID int32) {
	s.cache.Delete(mfaFailUserPrefix + strconv.Itoa(int(userID)))
	s.cache.Delete(mfaLockUserPrefix + strconv.Itoa(int(userID)))
}

func (s *loginSecurity) ClearAll(identifier string, userID int32, ip string) {
	identifier = normalizeIdentifier(identifier)
	s.cache.Delete(loginFailIDPrefix + identifier)
	s.cache.Delete(loginLockIDPrefix + identifier)
	if userID > 0 {
		s.cache.Delete(loginFailUserPrefix + strconv.Itoa(int(userID)))
		s.cache.Delete(loginLockUserPrefix + strconv.Itoa(int(userID)))
	}
	if ip != "" {
		s.cache.Delete(loginFailIPPrefix + ip)
		s.cache.Delete(loginLockIPPrefix + ip)
	}
}

func (s *loginSecurity) locked(key string) bool {
	value, ok := s.cache.Get(key)
	if !ok {
		return false
	}
	locked, ok := value.(bool)
	return ok && locked
}

func (s *loginSecurity) increment(key string, expiration time.Duration) int {
	count := 0
	if value, ok := s.cache.Get(key); ok {
		if current, ok := value.(int); ok {
			count = current
		}
	}
	count++
	s.cache.Set(key, count, expiration)
	return count
}
