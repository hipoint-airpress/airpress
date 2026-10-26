package impl

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/hipoint-airpress/airpress/cache"
	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/event"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type stubAdminUserService struct {
	service.UserService
	user *entity.User
}

func (s *stubAdminUserService) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	if s.user.Username == username {
		return s.user, nil
	}
	return nil, xerr.NoRecord.New("not found")
}

func (s *stubAdminUserService) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	if s.user.Email == email {
		return s.user, nil
	}
	return nil, xerr.NoRecord.New("not found")
}

func (s *stubAdminUserService) GetByID(ctx context.Context, id int32) (*entity.User, error) {
	return s.user, nil
}

func (s *stubAdminUserService) PasswordMatch(ctx context.Context, hashedPassword, plainPassword string) bool {
	return plainPassword == "secret"
}

func (s *stubAdminUserService) MustNotExpire(ctx context.Context, expireTime *time.Time) error {
	return nil
}

type stubAdminOptionService struct {
	service.OptionService
	policy consts.MFAPolicy
}

func (s *stubAdminOptionService) GetMFAPolicy(ctx context.Context) (consts.MFAPolicy, error) {
	return s.policy, nil
}

type stubTOTPService struct {
	service.TwoFactorTOTPMFAService
}

func (s *stubTOTPService) UseMFA(mfaType consts.MFAType) bool {
	return mfaType != consts.MFANone
}

func (s *stubTOTPService) ValidateTFACode(tfaKey, tfaCode string) bool {
	return tfaCode == "123456"
}

type stubRecoveryCodeService struct {
	service.RecoveryCodeService
	validCode string
}

func (s *stubRecoveryCodeService) ValidateAndConsume(ctx context.Context, userID int32, code string) (bool, error) {
	return code == s.validCode, nil
}

func (s *stubRecoveryCodeService) DeleteAll(ctx context.Context, userID int32) error {
	return nil
}

func newTestAdminService(user *entity.User, policy consts.MFAPolicy, recoveryCode string) (*adminServiceImpl, cache.Cache) {
	c := cache.NewCache()
	return &adminServiceImpl{
		UserService:         &stubAdminUserService{user: user},
		OptionService:       &stubAdminOptionService{policy: policy},
		Cache:               c,
		Config:              &config.Config{},
		Event:               event.NewSyncEventBus(zap.NewNop()),
		TwoFactorTOTPMFA:    &stubTOTPService{},
		RecoveryCodeService: &stubRecoveryCodeService{validCode: recoveryCode},
		LoginSecurity:       newLoginSecurity(c),
	}, c
}

func loginParam(username, password, authCode string) param.LoginParam {
	return param.LoginParam{Username: username, Password: password, AuthCode: authCode}
}

func TestAdminAuthRequiresValidTOTP(t *testing.T) {
	user := &entity.User{
		ID: 1, Username: "admin", Email: "admin@example.com",
		MfaType: consts.MFATFATotp, MfaKey: "key", Role: consts.UserRoleSuperAdmin,
	}
	svc, c := newTestAdminService(user, consts.MFAPolicyNone, "")
	ctx := context.Background()

	if _, err := svc.Auth(ctx, loginParam("admin", "secret", "000000")); err == nil {
		t.Fatal("invalid TOTP should fail")
	} else if xerr.GetHTTPStatus(err) != xerr.StatusBadRequest {
		t.Fatalf("status = %d, want %d", xerr.GetHTTPStatus(err), xerr.StatusBadRequest)
	}

	token, err := svc.Auth(ctx, loginParam("admin", "secret", "123456"))
	if err != nil {
		t.Fatalf("Auth() error = %v", err)
	}
	if token.AccessToken == "" || token.RefreshToken == "" {
		t.Fatal("Auth() should return access and refresh tokens")
	}
	if value, ok := c.Get(cache.BuildTokenAccessKey(token.AccessToken)); !ok || value != user.ID {
		t.Fatalf("access token not cached, ok=%v value=%v", ok, value)
	}
}

func TestAdminAuthAcceptsRecoveryCode(t *testing.T) {
	user := &entity.User{
		ID: 2, Username: "admin", Email: "admin@example.com",
		MfaType: consts.MFATFATotp, MfaKey: "key", Role: consts.UserRoleSuperAdmin,
	}
	svc, _ := newTestAdminService(user, consts.MFAPolicyNone, "AAAA-BBBB-CCCC")
	ctx := context.Background()

	if _, err := svc.Auth(ctx, loginParam("admin", "secret", "AAAA-BBBB-CCCC")); err != nil {
		t.Fatalf("Auth() with recovery code error = %v", err)
	}
}

func TestAdminAuthRejectsInvalidMFAAndRecoveryCode(t *testing.T) {
	user := &entity.User{
		ID: 3, Username: "admin", Email: "admin@example.com",
		MfaType: consts.MFATFATotp, MfaKey: "key", Role: consts.UserRoleSuperAdmin,
	}
	svc, _ := newTestAdminService(user, consts.MFAPolicyNone, "AAAA-BBBB-CCCC")
	ctx := context.Background()

	if _, err := svc.Auth(ctx, loginParam("admin", "secret", "WRONG-CODE")); err == nil {
		t.Fatal("invalid TOTP and recovery code should fail")
	}
}

func TestAdminAuthReportsMFAEnrollmentRequirement(t *testing.T) {
	user := &entity.User{
		ID: 4, Username: "admin", Email: "admin@example.com",
		MfaType: consts.MFANone, Role: consts.UserRoleSuperAdmin,
	}

	svc, _ := newTestAdminService(user, consts.MFAPolicyAll, "")
	token, err := svc.Auth(context.Background(), loginParam("admin", "secret", ""))
	if err != nil {
		t.Fatalf("Auth() error = %v", err)
	}
	if !token.MFAEnrollmentRequired {
		t.Fatal("policy ALL should require MFA enrollment")
	}

	svc, _ = newTestAdminService(user, consts.MFAPolicyNone, "")
	token, err = svc.Auth(context.Background(), loginParam("admin", "secret", ""))
	if err != nil {
		t.Fatalf("Auth() error = %v", err)
	}
	if token.MFAEnrollmentRequired {
		t.Fatal("policy NONE should not require MFA enrollment")
	}
}

func TestAdminAuthenticateLocksAfterFailures(t *testing.T) {
	user := &entity.User{
		ID: 5, Username: "admin", Email: "admin@example.com",
		Role: consts.UserRoleSuperAdmin,
	}
	svc, _ := newTestAdminService(user, consts.MFAPolicyNone, "")
	ctx := context.Background()

	for i := 0; i < consts.LoginMaxFailures; i++ {
		if _, err := svc.Authenticate(ctx, loginParam("admin", "wrong", "")); err == nil {
			t.Fatal("wrong password should fail")
		}
	}

	_, err := svc.Authenticate(ctx, loginParam("admin", "secret", ""))
	if err == nil {
		t.Fatal("locked account should be rejected")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", got, xerr.StatusTooManyRequests)
	}
}

func TestAdminAuthenticateRejectsDisabledUser(t *testing.T) {
	user := &entity.User{
		ID: 6, Username: "admin", Email: "admin@example.com",
		Role: consts.UserRoleSuperAdmin, Disabled: true,
	}
	svc, _ := newTestAdminService(user, consts.MFAPolicyNone, "")

	_, err := svc.Authenticate(context.Background(), loginParam("admin", "secret", ""))
	if err == nil {
		t.Fatal("disabled user should be rejected")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusForbidden {
		t.Fatalf("status = %d, want %d", got, xerr.StatusForbidden)
	}
}

func TestGenerateAccessToken(t *testing.T) {
	first, err := GenerateAccessToken()
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	second, err := GenerateAccessToken()
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	if len(first) != 64 {
		t.Fatalf("token length = %d, want 64", len(first))
	}
	if _, err := hex.DecodeString(first); err != nil {
		t.Fatalf("token is not hex: %v", err)
	}
	if first == second {
		t.Fatal("tokens should be unique")
	}
}

func TestBuildAuthTokenSingleSession(t *testing.T) {
	user := &entity.User{ID: 7, Username: "admin", Role: consts.UserRoleSuperAdmin}
	svc, c := newTestAdminService(user, consts.MFAPolicyNone, "")

	first, err := svc.buildAuthToken(user)
	if err != nil {
		t.Fatalf("buildAuthToken() error = %v", err)
	}
	second, err := svc.buildAuthToken(user)
	if err != nil {
		t.Fatalf("buildAuthToken() error = %v", err)
	}

	if _, ok := c.Get(cache.BuildTokenAccessKey(first.AccessToken)); ok {
		t.Fatal("old access token should be invalidated")
	}
	if _, ok := c.Get(cache.BuildTokenRefreshKey(first.RefreshToken)); ok {
		t.Fatal("old refresh token should be invalidated")
	}
	if value, ok := c.Get(cache.BuildTokenAccessKey(second.AccessToken)); !ok || value != user.ID {
		t.Fatalf("new access token not cached, ok=%v value=%v", ok, value)
	}
}

func TestClearUserTokens(t *testing.T) {
	user := &entity.User{ID: 8, Username: "admin", Role: consts.UserRoleSuperAdmin}
	svc, c := newTestAdminService(user, consts.MFAPolicyNone, "")

	token, err := svc.buildAuthToken(user)
	if err != nil {
		t.Fatalf("buildAuthToken() error = %v", err)
	}
	clearUserTokens(c, user.ID)

	if _, ok := c.Get(cache.BuildTokenAccessKey(token.AccessToken)); ok {
		t.Fatal("access token should be removed")
	}
	if _, ok := c.Get(cache.BuildTokenRefreshKey(token.RefreshToken)); ok {
		t.Fatal("refresh token should be removed")
	}
	if _, ok := c.Get(cache.BuildAccessTokenKey(user.ID)); ok {
		t.Fatal("user access token pointer should be removed")
	}
	if _, ok := c.Get(cache.BuildRefreshTokenKey(user.ID)); ok {
		t.Fatal("user refresh token pointer should be removed")
	}
}

func TestRefreshTokenRejectsDisabledUser(t *testing.T) {
	user := &entity.User{ID: 9, Username: "admin", Role: consts.UserRoleSuperAdmin, Disabled: true}
	svc, c := newTestAdminService(user, consts.MFAPolicyNone, "")

	refreshToken := "refresh-token"
	c.Set(cache.BuildTokenRefreshKey(refreshToken), user.ID, time.Hour)

	if _, err := svc.RefreshToken(context.Background(), refreshToken); err == nil {
		t.Fatal("disabled user should not refresh token")
	}
}
