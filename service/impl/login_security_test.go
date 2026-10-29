package impl

import (
	"testing"

	"github.com/hipoint-airpress/airpress/cache"
	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

func TestLoginSecurityPasswordLockout(t *testing.T) {
	security := newLoginSecurity(cache.NewCache())
	const identifier = "admin"
	const ip = "127.0.0.1"
	const userID int32 = 7

	if err := security.CheckPasswordLock(identifier, ip); err != nil {
		t.Fatalf("初始状态不应锁定: %v", err)
	}

	for i := 0; i < consts.LoginMaxFailures; i++ {
		security.RecordPasswordFailure(identifier, userID, ip)
	}

	err := security.CheckPasswordLock(identifier, ip)
	if err == nil {
		t.Fatal("达到阈值后应锁定")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", got, xerr.StatusTooManyRequests)
	}

	security.ClearPasswordFailures(identifier, userID, ip)
	if err := security.CheckPasswordLock(identifier, ip); err != nil {
		t.Fatalf("清除失败计数后应解锁: %v", err)
	}
}

func TestLoginSecurityIPLockout(t *testing.T) {
	security := newLoginSecurity(cache.NewCache())
	const ip = "127.0.0.2"

	for i := 0; i < consts.LoginIPMaxFailures; i++ {
		security.RecordPasswordFailure("nobody", 0, ip)
	}

	err := security.CheckPasswordLock("nobody", ip)
	if err == nil {
		t.Fatal("同一 IP 达到阈值后应锁定")
	}
}

func TestLoginSecurityMFALockout(t *testing.T) {
	security := newLoginSecurity(cache.NewCache())
	const userID int32 = 8

	for i := 0; i < consts.MFAVerifyMaxFailures; i++ {
		security.RecordMFAFailure(userID)
	}

	err := security.CheckMFALock(userID)
	if err == nil {
		t.Fatal("MFA 达到阈值后应锁定")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", got, xerr.StatusTooManyRequests)
	}

	security.ClearMFAFailures(userID)
	if err := security.CheckMFALock(userID); err != nil {
		t.Fatalf("清除 MFA 失败计数后应解锁: %v", err)
	}
}
